package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"fmt"
	"sort"
	"strings"

	"github.com/lib/pq"
)

const varianceThresholdPct = 5.0

// ppicReportWarehouses menghasilkan daftar gudang laporan (scope + filter opsional)
// beserta outlet-outlet-nya (untuk sisi penjualan/teoretis).
func ppicReportWarehouses(warehouseID string, outletScope []string) ([]string, []string, error) {
	q := `SELECT w.id, COALESCE(w.outlet_id, '') FROM warehouses w WHERE w.is_active = true`
	args := []interface{}{}
	if outletScope != nil {
		args = append(args, pq.Array(outletScope))
		q += ` AND (w.outlet_id = ANY($1) OR w.type = 'central')`
	}
	if warehouseID != "" {
		args = append(args, warehouseID)
		q += fmt.Sprintf(` AND w.id = $%d`, len(args))
	}
	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var whIDs, outletIDs []string
	for rows.Next() {
		var wh, outlet string
		if rows.Scan(&wh, &outlet) == nil {
			whIDs = append(whIDs, wh)
			if outlet != "" {
				outletIDs = append(outletIDs, outlet)
			}
		}
	}
	return whIDs, outletIDs, nil
}

// GetPpicVarianceReport — pemakaian teoretis (penjualan × resep + WO × resep
// internal) vs aktual (semua stok keluar kecuali transfer) per item.
func GetPpicVarianceReport(dateFrom, dateTo, warehouseID string, outletScope []string) (*models.PpicVarianceReport, error) {
	whIDs, outletIDs, err := ppicReportWarehouses(warehouseID, outletScope)
	if err != nil {
		return nil, err
	}
	rep := &models.PpicVarianceReport{DateFrom: dateFrom, DateTo: dateTo, Rows: []models.PpicVarianceRow{}}
	if len(whIDs) == 0 {
		return rep, nil
	}

	type agg struct{ theoQty, actQty, actValue float64 }
	items := map[string]*agg{}
	get := func(id string) *agg {
		if a, ok := items[id]; ok {
			return a
		}
		a := &agg{}
		items[id] = a
		return a
	}

	// ── 1. Teoretis dari penjualan (per outlet, produk per NAMA → resep) ──
	// LATERAL LIMIT 1: satu baris produk per nama per outlet (hindari duplikat).
	if len(outletIDs) > 0 {
		theoQ := `
			WITH sold AS (
				SELECT o.outlet_id, COALESCE(NULLIF(item->>'product_name', ''), 'Unknown') AS pname,
					SUM(COALESCE((item->>'qty')::numeric, 0)) AS qty
				FROM cloud_orders o, jsonb_array_elements(COALESCE(o.items, '[]'::jsonb)) AS item
				WHERE o.outlet_id = ANY($1)
					AND o.created_at >= tz_day_start($2::date) AND o.created_at < tz_day_start($3::date + 1)
					AND NULLIF(o.payment_info->>'voided_at', '') IS NULL
					AND COALESCE(o.is_holding, false) = false
				GROUP BY 1, 2
			),
			prod AS (
				SELECT s.outlet_id, s.pname, s.qty,
					cp.id AS pid, COALESCE(cp.stock_type, '') AS stype,
					COALESCE(cp.linked_stock_item_id, '') AS linked, COALESCE(cp.recipe_master_id, '') AS master
				FROM sold s
				LEFT JOIN LATERAL (
					SELECT p.id, p.stock_type, p.linked_stock_item_id, p.recipe_master_id
					FROM cloud_products p
					WHERE p.outlet_id = s.outlet_id AND p.name = s.pname AND COALESCE(p.is_deleted, false) = false
					LIMIT 1
				) cp ON true
			)
			SELECT x.item_id, SUM(x.qty_base), 0
			FROM (
				SELECT p.linked AS item_id, p.qty AS qty_base FROM prod p WHERE p.stype = 'single' AND p.linked <> ''
				UNION ALL
				SELECT ri.item_id, ri.qty_base * p.qty FROM prod p JOIN recipe_items ri ON ri.recipe_master_id = p.master WHERE p.master <> ''
				UNION ALL
				SELECT pr.item_id, pr.qty_base * p.qty FROM prod p JOIN product_recipes pr ON pr.product_id = p.pid
					WHERE p.master = '' AND p.stype = 'recipe'
			) x GROUP BY x.item_id`
		rows, err := database.DB.Query(theoQ, pq.Array(outletIDs), dateFrom, dateTo)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			var qty, zero float64
			if rows.Scan(&id, &qty, &zero) == nil {
				get(id).theoQty += qty
			}
		}
		rows.Close()

		// Coverage resep + omzet.
		covQ := `
			WITH sold AS (
				SELECT o.outlet_id, COALESCE(NULLIF(item->>'product_name', ''), 'Unknown') AS pname,
					SUM(COALESCE((item->>'qty')::numeric, 0)) AS qty
				FROM cloud_orders o, jsonb_array_elements(COALESCE(o.items, '[]'::jsonb)) AS item
				WHERE o.outlet_id = ANY($1)
					AND o.created_at >= tz_day_start($2::date) AND o.created_at < tz_day_start($3::date + 1)
					AND NULLIF(o.payment_info->>'voided_at', '') IS NULL
					AND COALESCE(o.is_holding, false) = false
				GROUP BY 1, 2
			)
			SELECT COALESCE(SUM(s.qty), 0),
				COALESCE(SUM(CASE WHEN cp.ok THEN s.qty ELSE 0 END), 0)
			FROM sold s
			LEFT JOIN LATERAL (
				SELECT (
					(p.stock_type = 'single' AND COALESCE(p.linked_stock_item_id, '') <> '')
					OR (COALESCE(p.recipe_master_id, '') <> '' AND EXISTS (SELECT 1 FROM recipe_items ri WHERE ri.recipe_master_id = p.recipe_master_id))
					OR (p.stock_type = 'recipe' AND EXISTS (SELECT 1 FROM product_recipes pr WHERE pr.product_id = p.id))
				) AS ok
				FROM cloud_products p
				WHERE p.outlet_id = s.outlet_id AND p.name = s.pname AND COALESCE(p.is_deleted, false) = false
				LIMIT 1
			) cp ON true`
		database.DB.QueryRow(covQ, pq.Array(outletIDs), dateFrom, dateTo).Scan(&rep.QtySold, &rep.QtySoldWithRecipe)

		database.DB.QueryRow(`
			SELECT COALESCE(SUM(o.total_amount), 0) FROM cloud_orders o
			WHERE o.outlet_id = ANY($1)
			AND o.created_at >= tz_day_start($2::date) AND o.created_at < tz_day_start($3::date + 1)
			AND NULLIF(o.payment_info->>'voided_at', '') IS NULL
			AND COALESCE(o.is_holding, false) = false`, pq.Array(outletIDs), dateFrom, dateTo).Scan(&rep.Revenue)
	}

	// ── 2. Teoretis dari produksi (WO selesai × resep internal) ──
	woRows, err := database.DB.Query(`
		SELECT sir.child_item_id, SUM(sir.qty_base * wo.qty_actual_base)
		FROM work_orders wo
		JOIN stock_item_recipes sir ON sir.parent_item_id = wo.item_id
		WHERE wo.status = 'done' AND wo.warehouse_id = ANY($1)
		AND wo.finished_at >= tz_day_start($2::date) AND wo.finished_at < tz_day_start($3::date + 1)
		GROUP BY sir.child_item_id`, pq.Array(whIDs), dateFrom, dateTo)
	if err != nil {
		return nil, err
	}
	for woRows.Next() {
		var id string
		var qty float64
		if woRows.Scan(&id, &qty) == nil {
			get(id).theoQty += qty
		}
	}
	woRows.Close()

	// ── 3. Aktual: stok keluar kecuali transfer (relokasi bukan konsumsi) ──
	actRows, err := database.DB.Query(`
		SELECT sm.item_id, SUM(-sm.qty_base), SUM(-sm.qty_base * sm.cost_per_base)
		FROM stock_movements sm
		WHERE sm.warehouse_id = ANY($1) AND sm.qty_base < 0
		AND sm.movement_type <> 'transfer_out'
		AND sm.created_at >= tz_day_start($2::date) AND sm.created_at < tz_day_start($3::date + 1)
		GROUP BY sm.item_id`, pq.Array(whIDs), dateFrom, dateTo)
	if err != nil {
		return nil, err
	}
	for actRows.Next() {
		var id string
		var qty, value float64
		if actRows.Scan(&id, &qty, &value) == nil {
			a := get(id)
			a.actQty += qty
			a.actValue += value
		}
	}
	actRows.Close()

	// ── 4. Susun baris (info item + nilai teoretis) ──────────
	ids := make([]string, 0, len(items))
	for id := range items {
		ids = append(ids, id)
	}
	type itemInfo struct {
		code, name, cat, unit string
		avgCost               float64
	}
	infos := map[string]itemInfo{}
	if len(ids) > 0 {
		rows, err := database.DB.Query(`SELECT id, code, name, category, base_unit, avg_cost FROM stock_items WHERE id = ANY($1)`, pq.Array(ids))
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			var inf itemInfo
			if rows.Scan(&id, &inf.code, &inf.name, &inf.cat, &inf.unit, &inf.avgCost) == nil {
				infos[id] = inf
			}
		}
		rows.Close()
	}

	for id, a := range items {
		inf, ok := infos[id]
		if !ok {
			continue
		}
		cost := inf.avgCost
		if a.actQty > 0 {
			cost = a.actValue / a.actQty // biaya riil periode ini lebih akurat
		}
		r := models.PpicVarianceRow{
			ItemID: id, ItemCode: inf.code, ItemName: inf.name, Category: inf.cat, BaseUnit: inf.unit,
			TheoQty: round4(a.theoQty), TheoValue: round4(a.theoQty * cost),
			ActQty: round4(a.actQty), ActValue: round4(a.actValue),
			DiffQty: round4(a.actQty - a.theoQty), DiffValue: round4(a.actValue - a.theoQty*cost),
		}
		if a.theoQty > 0 {
			r.VariancePct = round4((a.actQty - a.theoQty) / a.theoQty * 100)
			r.IsOver = r.VariancePct > varianceThresholdPct || r.VariancePct < -varianceThresholdPct
		} else if a.actQty > 0 {
			r.VariancePct = 100
			r.IsOver = true
		}
		rep.Rows = append(rep.Rows, r)
		rep.TheoValue += r.TheoValue
		rep.ActValue += r.ActValue
	}
	sort.Slice(rep.Rows, func(i, j int) bool { return rep.Rows[i].DiffValue > rep.Rows[j].DiffValue })

	if rep.TheoValue > 0 {
		rep.VariancePct = round4((rep.ActValue - rep.TheoValue) / rep.TheoValue * 100)
	}
	if rep.Revenue > 0 {
		rep.FoodCostPct = round4(rep.ActValue / rep.Revenue * 100)
	}
	if rep.QtySold > 0 {
		rep.RecipeCoveragePct = round4(rep.QtySoldWithRecipe / rep.QtySold * 100)
	}
	rep.Representative = rep.RecipeCoveragePct >= 80
	return rep, nil
}

// GetPpicProductionReport — realisasi WO: yield, HPP, variance bahan.
func GetPpicProductionReport(dateFrom, dateTo, warehouseID string, outletScope []string) (*models.PpicProductionReport, error) {
	whIDs, _, err := ppicReportWarehouses(warehouseID, outletScope)
	if err != nil {
		return nil, err
	}
	rep := &models.PpicProductionReport{Rows: []models.PpicProductionRow{}}
	if len(whIDs) == 0 {
		return rep, nil
	}
	rows, err := database.DB.Query(`
		SELECT wo.wo_number, COALESCE(pp.plan_number, ''), w.name, si.name, si.base_unit,
			wo.qty_planned_base, COALESCE(wo.qty_actual_base, 0), COALESCE(wo.yield_pct, 0),
			COALESCE(wo.cost_total, 0), COALESCE(wo.cost_per_unit, 0),
			COALESCE((SELECT SUM((m.qty_actual_base - m.qty_plan_base) * m.cost_per_base)
				FROM work_order_materials m WHERE m.wo_id = wo.id AND m.qty_actual_base IS NOT NULL), 0),
			wo.finished_at::text, COALESCE(wo.executed_by, '')
		FROM work_orders wo
		JOIN warehouses w ON w.id = wo.warehouse_id
		JOIN stock_items si ON si.id = wo.item_id
		LEFT JOIN production_plans pp ON pp.id = wo.plan_id
		WHERE wo.status = 'done' AND wo.warehouse_id = ANY($1)
		AND wo.finished_at >= tz_day_start($2::date) AND wo.finished_at < tz_day_start($3::date + 1)
		ORDER BY wo.finished_at DESC`, pq.Array(whIDs), dateFrom, dateTo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sumPlanned, sumActual, sumYield float64
	for rows.Next() {
		var r models.PpicProductionRow
		var finishedAt *string
		if err := rows.Scan(&r.WoNumber, &r.PlanNumber, &r.WarehouseName, &r.ItemName, &r.BaseUnit,
			&r.QtyPlanned, &r.QtyActual, &r.YieldPct, &r.CostTotal, &r.CostPerUnit,
			&r.MatVariance, &finishedAt, &r.ExecutedBy); err != nil {
			return nil, err
		}
		if finishedAt != nil {
			r.FinishedAt = *finishedAt
		}
		rep.Rows = append(rep.Rows, r)
		rep.WoDone++
		rep.TotalCost += r.CostTotal
		rep.TotalMatVar += r.MatVariance
		sumPlanned += r.QtyPlanned
		sumActual += r.QtyActual
		sumYield += r.YieldPct
	}
	if rep.WoDone > 0 {
		rep.AvgYieldPct = round4(sumYield / float64(rep.WoDone))
	}
	if sumPlanned > 0 {
		rep.PlanAdherence = round4(sumActual / sumPlanned * 100)
	}
	return rep, nil
}

// GetPpicForecastAccReport — akurasi & bias forecast per produk per outlet.
func GetPpicForecastAccReport(dateFrom, dateTo, outletID string, outletScope []string) (*models.PpicForecastAccReport, error) {
	rep := &models.PpicForecastAccReport{Rows: []models.PpicForecastAccRow{}}
	cond := ` WHERE df.qty_actual > 0 AND df.forecast_date >= $1::date AND df.forecast_date <= $2::date`
	args := []interface{}{dateFrom, dateTo}
	if outletID != "" {
		args = append(args, outletID)
		cond += fmt.Sprintf(" AND df.outlet_id = $%d", len(args))
	} else if outletScope != nil {
		args = append(args, pq.Array(outletScope))
		cond += fmt.Sprintf(" AND df.outlet_id = ANY($%d)", len(args))
	}

	rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT COALESCE(o.name, df.outlet_id), df.product_name, COUNT(*),
			AVG(ABS(df.qty_actual - %s) / df.qty_actual),
			AVG((%s - df.qty_actual) / df.qty_actual)
		FROM demand_forecasts df
		LEFT JOIN outlets o ON o.id = df.outlet_id
		%s
		GROUP BY 1, 2 ORDER BY 3 DESC, 2`, forecastQtyExpr, forecastQtyExpr, cond), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sumMape float64
	for rows.Next() {
		var r models.PpicForecastAccRow
		var mape, bias float64
		if err := rows.Scan(&r.OutletName, &r.ProductName, &r.EvalRows, &mape, &bias); err != nil {
			return nil, err
		}
		r.AccuracyPct = round4(maxf(0, (1-mape)*100))
		r.BiasPct = round4(bias * 100)
		rep.Rows = append(rep.Rows, r)
		sumMape += mape * float64(r.EvalRows)
		rep.EvalRows += r.EvalRows
	}
	if rep.EvalRows > 0 {
		rep.AccuracyPct = round4(maxf(0, (1-sumMape/float64(rep.EvalRows))*100))
	}
	return rep, nil
}

// GetPpicOtifReport — kinerja vendor: tepat waktu terhadap need_by_date.
// Basis: PR ber-vendor dengan need_by_date terisi (mulai ada sejak MRP Fase 2);
// "diterima" = pr.received_at terisi. Lengkap-nya kiriman memakai proxy status
// received (rincian qty PR bersifat teks bebas — dicatat sebagai keterbatasan).
func GetPpicOtifReport(dateFrom, dateTo string, outletScope []string) (*models.PpicOtifReport, error) {
	rep := &models.PpicOtifReport{
		Rows: []models.PpicOtifRow{},
		Note: "Basis: PR dengan tanggal-butuh (need_by_date) — otomatis terisi untuk PR hasil MRP. In-full memakai proxy status diterima.",
	}
	cond := ` WHERE pr.need_by_date IS NOT NULL AND COALESCE(pr.vendor_name, '') <> ''
		AND pr.status NOT IN ('rejected','cancelled')
		AND pr.created_at >= tz_day_start($1::date) AND pr.created_at < tz_day_start($2::date + 1)`
	args := []interface{}{dateFrom, dateTo}
	if outletScope != nil {
		args = append(args, pq.Array(outletScope))
		cond += fmt.Sprintf(" AND (pr.outlet_id IS NULL OR pr.outlet_id = ANY($%d))", len(args))
	}

	rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT pr.vendor_name, COUNT(*),
			COUNT(pr.received_at),
			COUNT(CASE WHEN pr.received_at IS NOT NULL AND pr.received_at::date <= pr.need_by_date THEN 1 END),
			COUNT(CASE WHEN pr.received_at IS NOT NULL AND pr.received_at::date > pr.need_by_date THEN 1 END),
			COUNT(CASE WHEN pr.received_at IS NULL AND pr.need_by_date < CURRENT_DATE THEN 1 END),
			COALESCE(AVG(CASE WHEN pr.received_at IS NOT NULL
				THEN EXTRACT(EPOCH FROM (pr.received_at - pr.created_at)) / 86400 END), 0)
		FROM purchase_requests pr
		%s
		GROUP BY pr.vendor_name ORDER BY 2 DESC`, cond), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var totalReceived, totalOnTime int
	for rows.Next() {
		var r models.PpicOtifRow
		if err := rows.Scan(&r.VendorName, &r.TotalPR, &r.Received, &r.OnTime, &r.Late, &r.OutstandingOverdue, &r.AvgLeadDays); err != nil {
			return nil, err
		}
		r.AvgLeadDays = round4(r.AvgLeadDays)
		if r.Received > 0 {
			r.OtifPct = round4(float64(r.OnTime) / float64(r.Received) * 100)
		}
		rep.Rows = append(rep.Rows, r)
		rep.TotalPR += r.TotalPR
		totalReceived += r.Received
		totalOnTime += r.OnTime
	}
	if totalReceived > 0 {
		rep.OtifPct = round4(float64(totalOnTime) / float64(totalReceived) * 100)
	}
	return rep, nil
}

// GetPpicSoldReport — qty produk terjual per outlet (TANPA nominal penjualan;
// laporan khusus PPIC — angka rupiah ada di Laporan → Penjualan Produk yang
// izinnya terpisah). Void & titipan dikecualikan, konsisten dengan laporan lain.
func GetPpicSoldReport(dateFrom, dateTo, outletID, search string, outletScope []string) (*models.PpicSoldReport, error) {
	rep := &models.PpicSoldReport{DateFrom: dateFrom, DateTo: dateTo, Rows: []models.PpicSoldRow{}}

	// Jumlah hari pada rentang (untuk rata-rata per hari).
	database.DB.QueryRow(`SELECT ($2::date - $1::date) + 1`, dateFrom, dateTo).Scan(&rep.Days)
	if rep.Days < 1 {
		rep.Days = 1
	}

	cond := ` WHERE o.created_at >= tz_day_start($1::date) AND o.created_at < tz_day_start($2::date + 1)
		AND NULLIF(o.payment_info->>'voided_at', '') IS NULL
		AND COALESCE(o.is_holding, false) = false`
	args := []interface{}{dateFrom, dateTo}
	if outletID != "" {
		args = append(args, outletID)
		cond += fmt.Sprintf(" AND o.outlet_id = $%d", len(args))
	} else if outletScope != nil {
		args = append(args, pq.Array(outletScope))
		cond += fmt.Sprintf(" AND o.outlet_id = ANY($%d)", len(args))
	}

	q := fmt.Sprintf(`
		WITH sold AS (
			SELECT o.outlet_id, COALESCE(ot.name, o.outlet_code, '') AS outlet_name,
				COALESCE(NULLIF(item->>'product_name', ''), 'Unknown') AS pname,
				SUM(COALESCE((item->>'qty')::numeric, 0)) AS qty
			FROM cloud_orders o
			LEFT JOIN outlets ot ON ot.id = o.outlet_id,
				jsonb_array_elements(COALESCE(o.items, '[]'::jsonb)) AS item
			%s
			GROUP BY 1, 2, 3
		)
		SELECT s.outlet_name, s.pname,
			COALESCE(cp.category, ''), COALESCE(cp.ok, false), s.qty
		FROM sold s
		LEFT JOIN LATERAL (
			SELECT COALESCE(p.category_name, '') AS category, (
				(p.stock_type = 'single' AND COALESCE(p.linked_stock_item_id, '') <> '')
				OR (COALESCE(p.recipe_master_id, '') <> '' AND EXISTS (SELECT 1 FROM recipe_items ri WHERE ri.recipe_master_id = p.recipe_master_id))
				OR (p.stock_type = 'recipe' AND EXISTS (SELECT 1 FROM product_recipes pr WHERE pr.product_id = p.id))
			) AS ok
			FROM cloud_products p
			WHERE p.outlet_id = s.outlet_id AND p.name = s.pname AND COALESCE(p.is_deleted, false) = false
			LIMIT 1
		) cp ON true
		ORDER BY s.qty DESC, s.pname`, cond)

	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r models.PpicSoldRow
		if err := rows.Scan(&r.OutletName, &r.ProductName, &r.Category, &r.HasRecipe, &r.Qty); err != nil {
			return nil, err
		}
		if search != "" && !strings.Contains(strings.ToLower(r.ProductName), strings.ToLower(search)) &&
			!strings.Contains(strings.ToLower(r.Category), strings.ToLower(search)) {
			continue
		}
		r.AvgPerDay = round4(r.Qty / float64(rep.Days))
		rep.Rows = append(rep.Rows, r)
		rep.TotalQty += r.Qty
		if !r.HasRecipe {
			rep.NoRecipeQty += r.Qty
		}
	}
	rep.ProductCount = len(rep.Rows)
	for i := range rep.Rows {
		if rep.TotalQty > 0 {
			rep.Rows[i].SharePct = round4(rep.Rows[i].Qty / rep.TotalQty * 100)
		}
	}
	return rep, nil
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
