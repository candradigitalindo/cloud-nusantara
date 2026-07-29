package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"fmt"
	"math"
)

// Parameter standar perencanaan F&B:
// safety stock = z × σ(pemakaian harian) × √lead time  (z 1.65 ≈ service level 95%)
// ROP          = pemakaian harian rata-rata × lead time + safety stock
// Par level    = pemakaian harian × (lead time + siklus pesan) + safety stock
const (
	ppicServiceZ    = 1.65
	ppicReviewCycle = 3 // hari, siklus pemesanan default untuk par level
)

// usageStatsExpr: rata-rata & deviasi pemakaian harian per item-gudang, 28 hari
// terakhir (hari tanpa pemakaian dihitung 0 — penting untuk σ yang jujur).
const usageStatsLateral = `
	LEFT JOIN LATERAL (
		SELECT COALESCE(AVG(x.day_out), 0) AS avg_daily,
			COALESCE(STDDEV_POP(x.day_out), 0) AS std_daily
		FROM (
			SELECT COALESCE(SUM(-sm.qty_base), 0) AS day_out
			FROM generate_series(CURRENT_DATE - 27, CURRENT_DATE, '1 day'::interval) d(dt)
			LEFT JOIN stock_movements sm
				ON sm.item_id = si.id AND sm.warehouse_id = $1
				AND sm.qty_base < 0
				AND DATE(sm.created_at AT TIME ZONE 'Asia/Jakarta') = d.dt
			GROUP BY d.dt
		) x
	) u ON true`

// ListPlanningParams menampilkan parameter perencanaan per item untuk satu gudang,
// berdampingan dengan stok saat ini dan statistik pemakaian aktual.
func ListPlanningParams(warehouseID, search, category string, belowOnly bool, page, limit int) (*models.PlanningParamsResponse, error) {
	if warehouseID == "" {
		return nil, fmt.Errorf("warehouse_id wajib diisi")
	}

	where := ` WHERE si.is_active = true`
	args := []interface{}{warehouseID}
	if search != "" {
		args = append(args, "%"+search+"%")
		where += fmt.Sprintf(" AND (si.name ILIKE $%d OR si.code ILIKE $%d)", len(args), len(args))
	}
	if category != "" {
		args = append(args, category)
		where += fmt.Sprintf(" AND si.category = $%d", len(args))
	}
	if belowOnly {
		where += fmt.Sprintf(" AND %s > 0 AND COALESCE(sl.qty_base,0) <= %s", ppicThresholdExpr, ppicThresholdExpr)
	}

	base := `
		FROM stock_items si
		LEFT JOIN stock_ledger sl ON sl.item_id = si.id AND sl.warehouse_id = $1
		LEFT JOIN item_planning_params pp ON pp.item_id = si.id AND pp.warehouse_id = $1`

	var total int
	if err := database.DB.QueryRow(`SELECT COUNT(*)`+base+where, args...).Scan(&total); err != nil {
		return nil, err
	}

	q := fmt.Sprintf(`
		SELECT si.id, si.code, si.name, si.category, si.base_unit,
			si.dist_unit, si.dist_ratio, si.dist_unit_label,
			COALESCE(sl.qty_base, 0), COALESCE(sl.avg_cost, 0), COALESCE(sl.min_stock, 0),
			u.avg_daily, u.std_daily,
			(pp.item_id IS NOT NULL),
			COALESCE(pp.lead_time_days, NULLIF(si.default_lead_time_days, 0), 0),
			COALESCE(pp.safety_stock, 0), COALESCE(pp.reorder_point, 0),
			COALESCE(pp.par_level, 0), COALESCE(pp.moq, 0)
		%s%s%s
		ORDER BY si.name LIMIT %d OFFSET %d`, base, usageStatsLateral, where, limit, (page-1)*limit)

	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	resp := &models.PlanningParamsResponse{Data: []models.PlanningParamRow{}, Total: total}
	for rows.Next() {
		var r models.PlanningParamRow
		if err := rows.Scan(&r.ItemID, &r.ItemCode, &r.ItemName, &r.Category, &r.BaseUnit,
			&r.DistUnit, &r.DistRatio, &r.DistUnitLabel,
			&r.QtyBase, &r.AvgCost, &r.MinStock,
			&r.AvgDailyUsage, &r.StdDailyUsage,
			&r.HasParams, &r.LeadTimeDays,
			&r.SafetyStock, &r.ReorderPoint, &r.ParLevel, &r.Moq); err != nil {
			return nil, err
		}
		r.WarehouseID = warehouseID
		r.SugSafetyStock, r.SugReorderPoint, r.SugParLevel = suggestParams(r.AvgDailyUsage, r.StdDailyUsage, r.LeadTimeDays)
		threshold := r.ReorderPoint
		if threshold == 0 {
			threshold = r.MinStock
		}
		r.IsBelowRop = threshold > 0 && r.QtyBase <= threshold
		resp.Data = append(resp.Data, r)
	}
	return resp, nil
}

// suggestParams menghitung saran safety stock / ROP / par level dari statistik
// pemakaian. Lead time 0 dianggap 1 hari agar rumus tetap bermakna.
func suggestParams(avgDaily, stdDaily float64, leadTimeDays int) (ss, rop, par float64) {
	lt := float64(leadTimeDays)
	if lt <= 0 {
		lt = 1
	}
	ss = ppicRound2(ppicServiceZ * stdDaily * math.Sqrt(lt))
	rop = ppicRound2(avgDaily*lt + ss)
	par = ppicRound2(avgDaily*(lt+ppicReviewCycle) + ss)
	return
}

func ppicRound2(v float64) float64 { return math.Round(v*100) / 100 }

// UpsertPlanningParams menyimpan parameter perencanaan (bulk, satu transaksi).
func UpsertPlanningParams(req models.PlanningParamsUpdateRequest, actor string) (int, error) {
	if len(req.Rows) == 0 {
		return 0, fmt.Errorf("tidak ada baris untuk disimpan")
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	n := 0
	for _, r := range req.Rows {
		if r.ItemID == "" || r.WarehouseID == "" {
			return 0, fmt.Errorf("item_id dan warehouse_id wajib diisi")
		}
		if r.LeadTimeDays < 0 || r.SafetyStock < 0 || r.ReorderPoint < 0 || r.ParLevel < 0 || r.Moq < 0 {
			return 0, fmt.Errorf("nilai parameter tidak boleh negatif")
		}
		if _, err := tx.Exec(`
			INSERT INTO item_planning_params (item_id, warehouse_id, lead_time_days, safety_stock, reorder_point, par_level, moq, updated_by, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW() AT TIME ZONE 'UTC')
			ON CONFLICT (item_id, warehouse_id) DO UPDATE SET
				lead_time_days = EXCLUDED.lead_time_days,
				safety_stock = EXCLUDED.safety_stock,
				reorder_point = EXCLUDED.reorder_point,
				par_level = EXCLUDED.par_level,
				moq = EXCLUDED.moq,
				updated_by = EXCLUDED.updated_by,
				updated_at = NOW() AT TIME ZONE 'UTC'`,
			r.ItemID, r.WarehouseID, r.LeadTimeDays, r.SafetyStock, r.ReorderPoint, r.ParLevel, r.Moq, actor); err != nil {
			return 0, err
		}
		n++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return n, nil
}

// AutoFillPlanningParams menghitung saran dari histori pemakaian 28 hari dan
// MENYIMPANNYA untuk semua item aktif yang punya pemakaian di gudang tersebut.
// Item yang sudah punya parameter ikut diperbarui (angka saran terbaru).
func AutoFillPlanningParams(warehouseID, actor string) (int, error) {
	if warehouseID == "" {
		return 0, fmt.Errorf("warehouse_id wajib diisi")
	}
	q := `
		SELECT si.id,
			COALESCE(pp.lead_time_days, NULLIF(si.default_lead_time_days, 0), 0),
			COALESCE(pp.moq, 0),
			u.avg_daily, u.std_daily
		FROM stock_items si
		LEFT JOIN item_planning_params pp ON pp.item_id = si.id AND pp.warehouse_id = $1` +
		usageStatsLateral + `
		WHERE si.is_active = true AND u.avg_daily > 0`

	rows, err := database.DB.Query(q, warehouseID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var upserts []models.PlanningParamUpsert
	for rows.Next() {
		var itemID string
		var leadTime int
		var moq, avgDaily, stdDaily float64
		if err := rows.Scan(&itemID, &leadTime, &moq, &avgDaily, &stdDaily); err != nil {
			return 0, err
		}
		ss, rop, par := suggestParams(avgDaily, stdDaily, leadTime)
		lt := leadTime
		if lt <= 0 {
			lt = 1
		}
		upserts = append(upserts, models.PlanningParamUpsert{
			ItemID: itemID, WarehouseID: warehouseID, LeadTimeDays: lt,
			SafetyStock: ss, ReorderPoint: rop, ParLevel: par, Moq: moq,
		})
	}
	if len(upserts) == 0 {
		return 0, fmt.Errorf("tidak ada item dengan histori pemakaian 28 hari di gudang ini")
	}
	return UpsertPlanningParams(models.PlanningParamsUpdateRequest{Rows: upserts}, actor)
}
