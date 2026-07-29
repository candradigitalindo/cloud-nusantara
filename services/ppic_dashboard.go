package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
)

// ppicWarehouseScope menghasilkan klausa filter gudang untuk scope outlet
// (pola sama dengan GetWarehouseDashboard): user ber-scope melihat gudang
// pusat + gudang outlet miliknya. Klausa memakai alias tabel "w".
func ppicWarehouseScope(outletIDs []string) (string, []interface{}) {
	if outletIDs == nil {
		return "", nil
	}
	return " AND (w.outlet_id = ANY($1) OR w.type = 'central')", []interface{}{pq.Array(outletIDs)}
}

// Ambang "di bawah ROP": pakai reorder_point dari item_planning_params bila
// terisi; jika tidak, jatuh ke min_stock per gudang (stock_ledger.min_stock).
const ppicThresholdExpr = `COALESCE(NULLIF(pp.reorder_point, 0), sl.min_stock)`

// GetPpicDashboard mengagregasi KPI, alert, dan data grafik untuk dashboard PPIC.
func GetPpicDashboard(outletIDs []string) (*models.PpicDashboardStats, error) {
	stats := &models.PpicDashboardStats{
		Alerts:         []models.PpicAlert{},
		UsageTrend:     []models.PpicDailyValuePoint{},
		TopWaste:       []models.PpicWasteItem{},
		ExpiryCalendar: []models.PpicExpiryPoint{},
		WarehouseRows:  []models.PpicWarehouseRow{},
	}
	scope, args := ppicWarehouseScope(outletIDs)

	run := func(q string, dest ...interface{}) {
		if args != nil {
			database.DB.QueryRow(q, args...).Scan(dest...)
		} else {
			database.DB.QueryRow(q).Scan(dest...)
		}
	}
	query := func(q string) (*sql.Rows, error) {
		if args != nil {
			return database.DB.Query(q, args...)
		}
		return database.DB.Query(q)
	}

	// ── 1. Nilai stok + status ambang ────────────────────────
	run(fmt.Sprintf(`
		SELECT
			COALESCE(SUM(CASE WHEN sl.qty_base > 0 THEN sl.qty_base * sl.avg_cost ELSE 0 END), 0),
			COUNT(CASE WHEN %s > 0 AND sl.qty_base > 0 AND sl.qty_base <= %s THEN 1 END),
			COUNT(CASE WHEN %s > 0 AND sl.qty_base <= 0 THEN 1 END)
		FROM stock_ledger sl
		JOIN stock_items si ON si.id = sl.item_id AND si.is_active = true
		JOIN warehouses w ON w.id = sl.warehouse_id
		LEFT JOIN item_planning_params pp ON pp.item_id = sl.item_id AND pp.warehouse_id = sl.warehouse_id
		WHERE w.is_active = true%s`, ppicThresholdExpr, ppicThresholdExpr, ppicThresholdExpr, scope),
		&stats.TotalStockValue, &stats.BelowRopCount, &stats.OutOfStockCount)

	// ── 2. Pemakaian (stok keluar) 30 & 28 hari → turnover + coverage ──
	var usage28 float64
	run(fmt.Sprintf(`
		SELECT
			COALESCE(SUM(CASE WHEN sm.created_at >= NOW() - INTERVAL '30 days' THEN -sm.qty_base * sm.cost_per_base END), 0),
			COALESCE(SUM(CASE WHEN sm.created_at >= NOW() - INTERVAL '28 days' THEN -sm.qty_base * sm.cost_per_base END), 0)
		FROM stock_movements sm
		JOIN warehouses w ON w.id = sm.warehouse_id
		WHERE sm.qty_base < 0 AND w.is_active = true%s`, scope),
		&stats.UsageValue30d, &usage28)
	if usage28 > 0 {
		stats.CoverageDays = stats.TotalStockValue / (usage28 / 28)
	}
	if stats.TotalStockValue > 0 {
		stats.TurnoverRatio30d = stats.UsageValue30d / stats.TotalStockValue
	}

	// ── 3. Risiko kedaluwarsa (batch aktif) ──────────────────
	run(fmt.Sprintf(`
		SELECT
			COUNT(CASE WHEN sb.expiry_date < CURRENT_DATE THEN 1 END),
			COALESCE(SUM(CASE WHEN sb.expiry_date < CURRENT_DATE THEN sb.qty_base * sb.cost_per_base END), 0),
			COUNT(CASE WHEN sb.expiry_date >= CURRENT_DATE AND sb.expiry_date < CURRENT_DATE + 7 THEN 1 END),
			COALESCE(SUM(CASE WHEN sb.expiry_date >= CURRENT_DATE AND sb.expiry_date < CURRENT_DATE + 7 THEN sb.qty_base * sb.cost_per_base END), 0)
		FROM stock_batches sb
		JOIN warehouses w ON w.id = sb.warehouse_id
		WHERE sb.qty_base > 0 AND sb.expiry_date IS NOT NULL AND w.is_active = true%s`, scope),
		&stats.ExpiredCount, &stats.ExpiredValue, &stats.Expiring7dCount, &stats.Expiring7dValue)

	// ── 4. Waste 30 hari ─────────────────────────────────────
	run(fmt.Sprintf(`
		SELECT COALESCE(SUM(sw.total_cost), 0)
		FROM stock_wastes sw
		JOIN warehouses w ON w.id = sw.warehouse_id
		WHERE sw.created_at >= NOW() - INTERVAL '30 days' AND w.is_active = true%s`, scope),
		&stats.WasteValue30d)
	if stats.UsageValue30d > 0 {
		stats.WastePct30d = stats.WasteValue30d / stats.UsageValue30d * 100
	}

	// ── 5. Dead stock: ada stok, tanpa pergerakan keluar 30 hari ──
	run(fmt.Sprintf(`
		SELECT COUNT(*), COALESCE(SUM(sl.qty_base * sl.avg_cost), 0)
		FROM stock_ledger sl
		JOIN stock_items si ON si.id = sl.item_id AND si.is_active = true
		JOIN warehouses w ON w.id = sl.warehouse_id
		WHERE sl.qty_base > 0 AND w.is_active = true%s
		AND NOT EXISTS (
			SELECT 1 FROM stock_movements sm
			WHERE sm.item_id = sl.item_id AND sm.warehouse_id = sl.warehouse_id
			AND sm.qty_base < 0 AND sm.created_at >= NOW() - INTERVAL '30 days')`, scope),
		&stats.DeadStockCount, &stats.DeadStockValue)

	// ── 6. Opname terakhir ───────────────────────────────────
	stats.LastOpname = getLastOpnameSummary(outletIDs)

	// ── 6b. Akurasi forecast 7 hari (Fase 2) ─────────────────
	stats.ForecastAccuracy7d, stats.ForecastEvalRows = forecastAccuracy(7, outletIDs)

	// ── 7. Alert feed ────────────────────────────────────────
	stats.Alerts = collectPpicAlerts(scope, args)

	// ── 8. Grafik: nilai masuk vs keluar per hari, 30 hari ───
	whSub := "SELECT id FROM warehouses w WHERE w.is_active = true" + scope
	utRows, utErr := query(fmt.Sprintf(`
		SELECT TO_CHAR(d.dt, 'YYYY-MM-DD'),
			COALESCE(SUM(CASE WHEN sm.qty_base > 0 THEN sm.qty_base * sm.cost_per_base END), 0),
			COALESCE(SUM(CASE WHEN sm.qty_base < 0 THEN -sm.qty_base * sm.cost_per_base END), 0)
		FROM generate_series(CURRENT_DATE - 29, CURRENT_DATE, '1 day'::interval) d(dt)
		LEFT JOIN stock_movements sm
			ON DATE(sm.created_at AT TIME ZONE 'Asia/Jakarta') = d.dt
			AND sm.warehouse_id IN (%s)
		GROUP BY d.dt ORDER BY d.dt`, whSub))
	if utErr == nil {
		defer utRows.Close()
		for utRows.Next() {
			var p models.PpicDailyValuePoint
			if err := utRows.Scan(&p.Date, &p.InValue, &p.OutValue); err == nil {
				stats.UsageTrend = append(stats.UsageTrend, p)
			}
		}
	}

	// ── 9. Top 10 waste 30 hari ──────────────────────────────
	twRows, twErr := query(fmt.Sprintf(`
		SELECT sw.item_id, MAX(sw.item_name), MAX(sw.base_unit),
			COALESCE(SUM(sw.qty_base), 0), COALESCE(SUM(sw.total_cost), 0)
		FROM stock_wastes sw
		JOIN warehouses w ON w.id = sw.warehouse_id
		WHERE sw.created_at >= NOW() - INTERVAL '30 days' AND w.is_active = true%s
		GROUP BY sw.item_id ORDER BY SUM(sw.total_cost) DESC LIMIT 10`, scope))
	if twErr == nil {
		defer twRows.Close()
		for twRows.Next() {
			var r models.PpicWasteItem
			if err := twRows.Scan(&r.ItemID, &r.ItemName, &r.BaseUnit, &r.TotalQty, &r.Value); err == nil {
				stats.TopWaste = append(stats.TopWaste, r)
			}
		}
	}

	// ── 10. Kalender kedaluwarsa 30 hari ke depan ────────────
	ecRows, ecErr := query(fmt.Sprintf(`
		SELECT TO_CHAR(sb.expiry_date, 'YYYY-MM-DD'), COUNT(*), COALESCE(SUM(sb.qty_base * sb.cost_per_base), 0)
		FROM stock_batches sb
		JOIN warehouses w ON w.id = sb.warehouse_id
		WHERE sb.qty_base > 0 AND sb.expiry_date IS NOT NULL
		AND sb.expiry_date >= CURRENT_DATE AND sb.expiry_date < CURRENT_DATE + 30
		AND w.is_active = true%s
		GROUP BY sb.expiry_date ORDER BY sb.expiry_date`, scope))
	if ecErr == nil {
		defer ecRows.Close()
		for ecRows.Next() {
			var p models.PpicExpiryPoint
			if err := ecRows.Scan(&p.Date, &p.Count, &p.Value); err == nil {
				stats.ExpiryCalendar = append(stats.ExpiryCalendar, p)
			}
		}
	}

	// ── 11. Ringkasan per gudang ─────────────────────────────
	wrRows, wrErr := query(fmt.Sprintf(`
		SELECT w.id, w.name, w.type, COALESCE(o.name, '—'),
			COALESCE(SUM(CASE WHEN sl.qty_base > 0 THEN sl.qty_base * sl.avg_cost END), 0),
			COUNT(CASE WHEN %s > 0 AND sl.qty_base <= %s THEN 1 END),
			COALESCE((SELECT SUM(sb.qty_base * sb.cost_per_base) FROM stock_batches sb
				WHERE sb.warehouse_id = w.id AND sb.qty_base > 0
				AND sb.expiry_date IS NOT NULL AND sb.expiry_date < CURRENT_DATE + 7), 0)
		FROM warehouses w
		LEFT JOIN outlets o ON o.id = w.outlet_id
		LEFT JOIN stock_ledger sl ON sl.warehouse_id = w.id
		LEFT JOIN stock_items si ON si.id = sl.item_id AND si.is_active = true
		LEFT JOIN item_planning_params pp ON pp.item_id = sl.item_id AND pp.warehouse_id = sl.warehouse_id
		WHERE w.is_active = true%s
		GROUP BY w.id, w.name, w.type, o.name
		ORDER BY w.type DESC, 5 DESC`, ppicThresholdExpr, ppicThresholdExpr, scope))
	if wrErr == nil {
		defer wrRows.Close()
		for wrRows.Next() {
			var r models.PpicWarehouseRow
			if err := wrRows.Scan(&r.WarehouseID, &r.WarehouseName, &r.WarehouseType, &r.OutletName,
				&r.StockValue, &r.BelowRopCount, &r.ExpiringValue); err == nil {
				stats.WarehouseRows = append(stats.WarehouseRows, r)
			}
		}
	}

	return stats, nil
}

// collectPpicAlerts membangun feed peringatan terurut prioritas.
// Batch yang sudah ditandai "ditindak" (ppic_ack_at) tidak dimunculkan lagi.
func collectPpicAlerts(scope string, args []interface{}) []models.PpicAlert {
	alerts := []models.PpicAlert{}

	query := func(q string) (*sql.Rows, error) {
		if args != nil {
			return database.DB.Query(q, args...)
		}
		return database.DB.Query(q)
	}

	// 1. Batch sudah lewat kedaluwarsa (merah)
	if rows, err := query(fmt.Sprintf(`
		SELECT si.id, si.name, si.base_unit, w.id, w.name,
			sb.qty_base, sb.qty_base * sb.cost_per_base, sb.expiry_date::text
		FROM stock_batches sb
		JOIN stock_items si ON si.id = sb.item_id
		JOIN warehouses w ON w.id = sb.warehouse_id
		WHERE sb.qty_base > 0 AND sb.expiry_date < CURRENT_DATE
		AND sb.ppic_ack_at IS NULL AND w.is_active = true%s
		ORDER BY sb.expiry_date ASC LIMIT 10`, scope)); err == nil {
		defer rows.Close()
		for rows.Next() {
			a := models.PpicAlert{Type: "expired", Severity: "red"}
			if rows.Scan(&a.ItemID, &a.ItemName, &a.Unit, &a.WarehouseID, &a.WarehouseName,
				&a.Qty, &a.Value, &a.Date) == nil {
				a.Message = "Batch sudah lewat kedaluwarsa"
				alerts = append(alerts, a)
			}
		}
	}

	// 2. Stok habis padahal ambang terpasang (merah)
	if rows, err := query(fmt.Sprintf(`
		SELECT si.id, si.name, si.base_unit, w.id, w.name, sl.qty_base
		FROM stock_ledger sl
		JOIN stock_items si ON si.id = sl.item_id AND si.is_active = true
		JOIN warehouses w ON w.id = sl.warehouse_id
		LEFT JOIN item_planning_params pp ON pp.item_id = sl.item_id AND pp.warehouse_id = sl.warehouse_id
		WHERE %s > 0 AND sl.qty_base <= 0 AND w.is_active = true%s
		ORDER BY si.name LIMIT 10`, ppicThresholdExpr, scope)); err == nil {
		defer rows.Close()
		for rows.Next() {
			a := models.PpicAlert{Type: "stockout", Severity: "red"}
			if rows.Scan(&a.ItemID, &a.ItemName, &a.Unit, &a.WarehouseID, &a.WarehouseName, &a.Qty) == nil {
				a.Message = "Stok habis"
				alerts = append(alerts, a)
			}
		}
	}

	// 3. Batch kedaluwarsa ≤ 3 hari (kuning)
	if rows, err := query(fmt.Sprintf(`
		SELECT si.id, si.name, si.base_unit, w.id, w.name,
			sb.qty_base, sb.qty_base * sb.cost_per_base, sb.expiry_date::text
		FROM stock_batches sb
		JOIN stock_items si ON si.id = sb.item_id
		JOIN warehouses w ON w.id = sb.warehouse_id
		WHERE sb.qty_base > 0 AND sb.expiry_date >= CURRENT_DATE AND sb.expiry_date < CURRENT_DATE + 3
		AND sb.ppic_ack_at IS NULL AND w.is_active = true%s
		ORDER BY sb.expiry_date ASC LIMIT 10`, scope)); err == nil {
		defer rows.Close()
		for rows.Next() {
			a := models.PpicAlert{Type: "expiring", Severity: "amber"}
			if rows.Scan(&a.ItemID, &a.ItemName, &a.Unit, &a.WarehouseID, &a.WarehouseName,
				&a.Qty, &a.Value, &a.Date) == nil {
				a.Message = "Kedaluwarsa ≤ 3 hari"
				alerts = append(alerts, a)
			}
		}
	}

	// 4. Menyentuh titik pesan ulang (kuning)
	if rows, err := query(fmt.Sprintf(`
		SELECT si.id, si.name, si.base_unit, w.id, w.name, sl.qty_base, %s
		FROM stock_ledger sl
		JOIN stock_items si ON si.id = sl.item_id AND si.is_active = true
		JOIN warehouses w ON w.id = sl.warehouse_id
		LEFT JOIN item_planning_params pp ON pp.item_id = sl.item_id AND pp.warehouse_id = sl.warehouse_id
		WHERE %s > 0 AND sl.qty_base > 0 AND sl.qty_base <= %s AND w.is_active = true%s
		ORDER BY sl.qty_base / %s ASC LIMIT 10`,
		ppicThresholdExpr, ppicThresholdExpr, ppicThresholdExpr, scope, ppicThresholdExpr)); err == nil {
		defer rows.Close()
		for rows.Next() {
			a := models.PpicAlert{Type: "below_rop", Severity: "amber"}
			var threshold float64
			if rows.Scan(&a.ItemID, &a.ItemName, &a.Unit, &a.WarehouseID, &a.WarehouseName, &a.Qty, &threshold) == nil {
				a.Message = fmt.Sprintf("Di bawah titik pesan ulang (%.2f)", threshold)
				alerts = append(alerts, a)
			}
		}
	}

	// 5. Dead stock (abu-abu)
	if rows, err := query(fmt.Sprintf(`
		SELECT si.id, si.name, si.base_unit, w.id, w.name, sl.qty_base, sl.qty_base * sl.avg_cost
		FROM stock_ledger sl
		JOIN stock_items si ON si.id = sl.item_id AND si.is_active = true
		JOIN warehouses w ON w.id = sl.warehouse_id
		WHERE sl.qty_base > 0 AND w.is_active = true%s
		AND NOT EXISTS (
			SELECT 1 FROM stock_movements sm
			WHERE sm.item_id = sl.item_id AND sm.warehouse_id = sl.warehouse_id
			AND sm.qty_base < 0 AND sm.created_at >= NOW() - INTERVAL '30 days')
		ORDER BY sl.qty_base * sl.avg_cost DESC LIMIT 5`, scope)); err == nil {
		defer rows.Close()
		for rows.Next() {
			a := models.PpicAlert{Type: "dead_stock", Severity: "gray"}
			if rows.Scan(&a.ItemID, &a.ItemName, &a.Unit, &a.WarehouseID, &a.WarehouseName, &a.Qty, &a.Value) == nil {
				a.Message = "Tanpa pergerakan keluar > 30 hari"
				alerts = append(alerts, a)
			}
		}
	}

	return alerts
}

// getLastOpnameSummary mengambil ringkasan sesi opname terakhir (dalam scope).
func getLastOpnameSummary(outletIDs []string) *models.PpicOpnameSummary {
	scope, args := ppicWarehouseScope(outletIDs)
	q := fmt.Sprintf(`
		SELECT so.id, so.opname_number, w.name, so.status, so.created_at::text,
			COUNT(soi.id),
			COUNT(soi.qty_counted_base),
			COUNT(CASE WHEN soi.qty_counted_base IS NOT NULL AND soi.qty_counted_base <> soi.qty_system_base THEN 1 END),
			COALESCE(SUM(CASE WHEN soi.qty_counted_base IS NOT NULL
				THEN (soi.qty_counted_base - soi.qty_system_base) * soi.cost_per_base END), 0)
		FROM stock_opnames so
		JOIN warehouses w ON w.id = so.warehouse_id
		LEFT JOIN stock_opname_items soi ON soi.opname_id = so.id
		WHERE so.status <> 'cancelled'%s
		GROUP BY so.id, so.opname_number, w.name, so.status, so.created_at
		ORDER BY so.created_at DESC LIMIT 1`, scope)

	var s models.PpicOpnameSummary
	var diffValue float64
	row := database.DB.QueryRow(q)
	if args != nil {
		row = database.DB.QueryRow(q, args...)
	}
	if err := row.Scan(&s.ID, &s.OpnameNumber, &s.WarehouseName, &s.Status, &s.CreatedAt,
		&s.ItemsTotal, &s.ItemsCounted, &s.ItemsDiff, &diffValue); err != nil {
		return nil
	}
	if s.ItemsCounted > 0 {
		s.AccuracyPct = float64(s.ItemsCounted-s.ItemsDiff) / float64(s.ItemsCounted) * 100
	}
	return &s
}
