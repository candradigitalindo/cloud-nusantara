package services

import (
	"fmt"
	"strings"

	"cloud-pos/database"
	"cloud-pos/models"
)

// Dashboard modul Aset: nilai (perolehan/penyusutan/buku), kesehatan perawatan,
// sebaran per kondisi/outlet/kategori, tren 12 bulan, dan pelepasan tahun ini.

// assetMonthSeries — 12 bulan berurutan sampai bulan berjalan, sebagai tulang
// punggung grafik tren agar panjangnya selalu tetap.
const assetMonthSeries = `generate_series(
			DATE_TRUNC('month', CURRENT_DATE) - INTERVAL '11 months',
			DATE_TRUNC('month', CURRENT_DATE),
			INTERVAL '1 month') AS s(bulan)`

// assetFilter menyusun kondisi WHERE dasar untuk aset aktif dalam cakupan.
// Semua query dashboard memakainya agar satu angka tidak pernah beda basis.
func assetFilter(outletID string, outletScope []string, startIdx int) (string, []interface{}, int) {
	conds := []string{"a.is_deleted = false", "COALESCE(a.status, 'aktif') <> 'dihapus'"}
	args := []interface{}{}
	idx := startIdx

	if outletID != "" {
		conds = append(conds, fmt.Sprintf("a.outlet_id = $%d", idx))
		args = append(args, outletID)
		idx++
	}
	scopeCond, scopeArgs := assetScopeCond("a", outletScope, idx)
	if scopeCond != "" {
		conds = append(conds, strings.TrimPrefix(scopeCond, " AND "))
		args = append(args, scopeArgs...)
		idx++
	}
	return strings.Join(conds, " AND "), args, idx
}

func AssetDashboardStats(outletID string, outletScope []string) (*models.AssetDashboard, error) {
	where, args, _ := assetFilter(outletID, outletScope, 1)
	d := models.AssetDashboard{DueSoonDays: assetDueSoonDays}

	// ── Nilai & kesehatan perawatan (satu lintasan) ──
	err := database.DB.QueryRow(fmt.Sprintf(`
		SELECT COUNT(*)::int,
		       COALESCE(SUM(a.quantity), 0)::int,
		       COALESCE(SUM(%s), 0),
		       COALESCE(SUM(%s), 0),
		       COALESCE(SUM(%s), 0),
		       COALESCE(SUM(%s), 0),
		       COUNT(CASE WHEN nd.next_due_date < CURRENT_DATE THEN 1 END)::int,
		       COUNT(CASE WHEN nd.next_due_date >= CURRENT_DATE AND nd.next_due_date <= CURRENT_DATE + %d THEN 1 END)::int,
		       COUNT(CASE WHEN nd.next_due_date IS NULL THEN 1 END)::int,
		       COUNT(CASE WHEN a.condition <> 'baik' THEN 1 END)::int,
		       COUNT(CASE WHEN COALESCE(a.useful_life_months,0) > 0
		                   AND GREATEST(COALESCE(a.useful_life_months,0) - %s, 0) <= 3 THEN 1 END)::int,
		       COUNT(CASE WHEN COALESCE(a.useful_life_months,0) > 0 THEN 1 END)::int
		%s
		WHERE %s`,
		assetCostExpr, assetAccumDeprecExpr, assetBookValueExpr, assetMonthlyDeprecExpr,
		assetDueSoonDays, assetAgeExpr, assetFromClause, where), args...).
		Scan(&d.TotalAssets, &d.TotalQuantity, &d.AcquisitionCost, &d.AccumulatedDeprec,
			&d.BookValue, &d.MonthlyDeprec, &d.Overdue, &d.DueSoon, &d.Unscheduled,
			&d.NeedsAttention, &d.EndingSoonCount, &d.DepreciatingCount)
	if err != nil {
		return nil, err
	}

	// ── Biaya perawatan: bulan berjalan & 12 bulan terakhir ──
	database.DB.QueryRow(fmt.Sprintf(`
		SELECT COALESCE(SUM(CASE WHEN mm.maintenance_date >= DATE_TRUNC('month', CURRENT_DATE) THEN mm.cost END), 0),
		       COALESCE(SUM(CASE WHEN mm.maintenance_date >= CURRENT_DATE - INTERVAL '12 months' THEN mm.cost END), 0)
		FROM asset_maintenances mm
		JOIN assets a ON a.id = mm.asset_id
		WHERE %s`, where), args...).Scan(&d.MaintCostMTD, &d.MaintCost12M)

	// ── Pelepasan tahun berjalan ──
	database.DB.QueryRow(fmt.Sprintf(`
		SELECT COUNT(*)::int, COALESCE(SUM(dd.proceeds), 0),
		       COALESCE(SUM(dd.proceeds - dd.book_value_at_disposal), 0)
		FROM asset_disposals dd
		JOIN assets a ON a.id = dd.asset_id
		WHERE dd.disposal_date >= DATE_TRUNC('year', CURRENT_DATE)
		  AND a.is_deleted = false%s`,
		disposalScopeSuffix(outletID, outletScope)), args...).
		Scan(&d.DisposedYTD, &d.ProceedsYTD, &d.GainLossYTD)

	// ── Sebaran ──
	if d.ConditionBreakdown, err = assetBuckets(fmt.Sprintf(`
		SELECT a.condition, a.condition, COUNT(*)::int, COALESCE(SUM(a.quantity),0)::int, COALESCE(SUM(%s),0)
		%s WHERE %s GROUP BY a.condition ORDER BY 3 DESC`,
		assetBookValueExpr, assetFromClause, where), args); err != nil {
		return nil, err
	}
	if d.ByOutlet, err = assetBuckets(fmt.Sprintf(`
		SELECT a.outlet_id, COALESCE(o.name, '(tanpa outlet)'), COUNT(*)::int,
		       COALESCE(SUM(a.quantity),0)::int, COALESCE(SUM(%s),0)
		%s WHERE %s GROUP BY a.outlet_id, o.name ORDER BY 5 DESC`,
		assetBookValueExpr, assetFromClause, where), args); err != nil {
		return nil, err
	}
	if d.ByCategory, err = assetBuckets(fmt.Sprintf(`
		SELECT COALESCE(NULLIF(a.category, ''), 'Tanpa kategori'), COALESCE(NULLIF(a.category, ''), 'Tanpa kategori'),
		       COUNT(*)::int, COALESCE(SUM(a.quantity),0)::int, COALESCE(SUM(%s),0)
		%s WHERE %s GROUP BY 1 ORDER BY 5 DESC LIMIT 10`,
		assetBookValueExpr, assetFromClause, where), args); err != nil {
		return nil, err
	}

	// ── Tren 12 bulan: perolehan & biaya perawatan ──
	// Deret bulan digenerate penuh lalu di-LEFT JOIN, supaya bulan tanpa data
	// tetap muncul sebagai nol. Tanpa ini grafik "12 bulan terakhir" hanya
	// menampilkan bulan yang kebetulan berisi — terbaca sebagai titik acak,
	// bukan tren, dan jarak antar bulan jadi menyesatkan.
	if d.AcquisitionTrend, err = assetMonthlySeries(fmt.Sprintf(`
		SELECT TO_CHAR(s.bulan, 'YYYY-MM'), COALESCE(t.cnt, 0), COALESCE(t.val, 0)
		FROM %s
		LEFT JOIN (
			SELECT DATE_TRUNC('month', ac.acquisition_date) AS bulan,
			       COUNT(*)::int AS cnt, SUM(ac.total_cost) AS val
			FROM asset_acquisitions ac
			JOIN assets a ON a.id = ac.asset_id
			WHERE %s
			GROUP BY 1
		) t ON t.bulan = s.bulan
		ORDER BY s.bulan`, assetMonthSeries, where), args); err != nil {
		return nil, err
	}
	if d.MaintenanceTrend, err = assetMonthlySeries(fmt.Sprintf(`
		SELECT TO_CHAR(s.bulan, 'YYYY-MM'), COALESCE(t.cnt, 0), COALESCE(t.val, 0)
		FROM %s
		LEFT JOIN (
			SELECT DATE_TRUNC('month', mm.maintenance_date) AS bulan,
			       COUNT(*)::int AS cnt, SUM(mm.cost) AS val
			FROM asset_maintenances mm
			JOIN assets a ON a.id = mm.asset_id
			WHERE %s
			GROUP BY 1
		) t ON t.bulan = s.bulan
		ORDER BY s.bulan`, assetMonthSeries, where), args); err != nil {
		return nil, err
	}

	// ── Perawatan paling mendesak (terlambat & segera) ──
	rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT %s%s WHERE %s AND nd.next_due_date IS NOT NULL
		  AND nd.next_due_date <= CURRENT_DATE + %d
		ORDER BY nd.next_due_date ASC LIMIT 8`,
		assetSelectCols, assetFromClause, where, assetDueSoonDays), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	d.UpcomingMaint = make([]models.Asset, 0)
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		d.UpcomingMaint = append(d.UpcomingMaint, a)
	}
	return &d, rows.Err()
}

// disposalScopeSuffix menyusun ulang filter outlet untuk query pelepasan, yang
// tidak memakai klausa aset aktif (aset yang dilepas justru berstatus dihapus).
func disposalScopeSuffix(outletID string, outletScope []string) string {
	idx := 1
	out := ""
	if outletID != "" {
		out += fmt.Sprintf(" AND a.outlet_id = $%d", idx)
		idx++
	}
	if scopeCond, _ := assetScopeCond("a", outletScope, idx); scopeCond != "" {
		out += scopeCond
	}
	return out
}

func assetBuckets(q string, args []interface{}) ([]models.AssetBucket, error) {
	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.AssetBucket, 0)
	for rows.Next() {
		var b models.AssetBucket
		if err := rows.Scan(&b.Key, &b.Label, &b.Count, &b.Quantity, &b.Value); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func assetMonthlySeries(q string, args []interface{}) ([]models.AssetMonthly, error) {
	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.AssetMonthly, 0)
	for rows.Next() {
		var m models.AssetMonthly
		if err := rows.Scan(&m.Month, &m.Count, &m.Value); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
