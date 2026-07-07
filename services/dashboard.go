package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

// dashBounds — semua batas waktu dashboard dihitung SEKALI di Go (zona aplikasi),
// dikonversi ke instant UTC (kolom timestamp kita menyimpan wall-clock UTC), lalu
// disuntikkan sebagai literal ke query. Ini menghindari pemanggilan tz_day_start()/
// tz_today() yang membaca app_settings pada setiap evaluasi — di data ribuan baris
// itu membuat query 100× lebih lambat dibanding perbandingan timestamp biasa.
type dashBounds struct {
	todayStart, tomorrowStart, yesterdayStart string
	monthStart, monthPrevStart                string
	weekStart, weekPrevStart                  string
	rangeStart, rangeEnd                      string // rentang terpilih (end eksklusif)
	prevStart, prevEnd                        string // rentang sebelumnya
	fromDate, toDate                          string // DATE literal untuk generate_series tren
	tz                                        string // nama zona waktu (untuk konversi per-hari di tren)
}

// litTS memformat instant sebagai literal TIMESTAMP UTC (wall-clock, tanpa zona).
func litTS(t time.Time) string {
	return "TIMESTAMP '" + t.UTC().Format("2006-01-02 15:04:05") + "'"
}

// tzLit membungkus nama zona waktu sebagai literal SQL aman.
func tzLit(tz string) string {
	return "'" + strings.ReplaceAll(tz, "'", "''") + "'"
}

func computeDashBounds(dateFrom, dateTo string) dashBounds {
	loc := GetTimezoneLocation()
	tzName, _ := GetTimezone()
	if tzName == "" {
		tzName = "Asia/Jakarta"
	}
	now := time.Now().In(loc)
	midnight := func(t time.Time) time.Time {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	}
	today := midnight(now)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	wd := (int(today.Weekday()) + 6) % 7 // Senin = 0
	weekStart := today.AddDate(0, 0, -wd)

	rf, errF := time.ParseInLocation("2006-01-02", dateFrom, loc)
	rt, errT := time.ParseInLocation("2006-01-02", dateTo, loc)
	if errF != nil {
		rf = today
	}
	if errT != nil {
		rt = today
	}
	rangeStart := midnight(rf)
	rangeEnd := midnight(rt).AddDate(0, 0, 1) // eksklusif
	days := int(rangeEnd.Sub(rangeStart).Hours() / 24)
	if days < 1 {
		days = 1
	}

	return dashBounds{
		todayStart:     litTS(today),
		tomorrowStart:  litTS(today.AddDate(0, 0, 1)),
		yesterdayStart: litTS(today.AddDate(0, 0, -1)),
		monthStart:     litTS(monthStart),
		monthPrevStart: litTS(monthStart.AddDate(0, -1, 0)),
		weekStart:      litTS(weekStart),
		weekPrevStart:  litTS(weekStart.AddDate(0, 0, -7)),
		rangeStart:     litTS(rangeStart),
		rangeEnd:       litTS(rangeEnd),
		prevStart:      litTS(rangeStart.AddDate(0, 0, -days)),
		prevEnd:        litTS(rangeStart),
		fromDate:       "DATE '" + rf.Format("2006-01-02") + "'",
		toDate:         "DATE '" + rt.Format("2006-01-02") + "'",
		tz:             tzName,
	}
}

func GetDashboardStats(dateFrom, dateTo string, scopeIDs []string) (*models.DashboardStats, error) {
	stats := &models.DashboardStats{}
	b := computeDashBounds(dateFrom, dateTo) // batas today/month dihitung sekali di Go

	// Build scope filter: if scopeIDs is nil → no filter, otherwise filter by outlet IDs
	var scopeParam interface{}
	outletFilter := ""    // e.g. " AND outlet_id = ANY($1)" for tables with outlet_id
	outletsIDFilter := "" // e.g. " AND id = ANY($1)" for the outlets table itself
	if scopeIDs != nil {
		scopeParam = pq.Array(scopeIDs)
		outletFilter = " AND outlet_id = ANY($1)"
		outletsIDFilter = " AND id = ANY($1)"
	}

	// Satu pass SUM(CASE) per tabel besar (bukan 17 subquery = 17 scan terpisah).
	// cloud_transactions & cloud_orders masing-masing cukup dibaca sekali.
	run := func(q string, dest ...interface{}) error {
		if scopeParam != nil {
			return database.DB.QueryRow(q, scopeParam).Scan(dest...)
		}
		return database.DB.QueryRow(q).Scan(dest...)
	}

	txQuery := fmt.Sprintf(`
		SELECT
			COUNT(*),
			COALESCE(SUM(total_amount),0),
			COUNT(CASE WHEN created_at >= %[1]s THEN 1 END),
			COUNT(CASE WHEN created_at >= %[2]s AND created_at < %[1]s THEN 1 END),
			COALESCE(SUM(CASE WHEN created_at >= %[1]s THEN total_amount END),0),
			COALESCE(SUM(CASE WHEN created_at >= %[2]s AND created_at < %[1]s THEN total_amount END),0),
			COALESCE(SUM(CASE WHEN created_at >= %[3]s AND created_at < %[4]s THEN total_amount END),0)
		FROM cloud_transactions WHERE 1=1%[5]s`, b.monthStart, b.monthPrevStart, b.todayStart, b.tomorrowStart, outletFilter)
	if err := run(txQuery,
		&stats.TotalTransactions, &stats.TotalRevenue,
		&stats.MonthTransactions, &stats.MonthTransactionsPrev,
		&stats.MonthRevenue, &stats.MonthRevenuePrev, &stats.TodayRevenue,
	); err != nil {
		return nil, fmt.Errorf("dashboard stats (transactions) failed: %w", err)
	}

	ordQuery := fmt.Sprintf(`
		SELECT
			COUNT(*),
			COUNT(CASE WHEN created_at >= %[1]s AND created_at < %[2]s THEN 1 END),
			COUNT(CASE WHEN created_at >= %[3]s AND created_at < %[1]s THEN 1 END),
			COUNT(CASE WHEN COALESCE(payment_info->>'payment_status','unpaid') NOT IN ('paid') AND NULLIF(payment_info->>'voided_at','') IS NULL THEN 1 END),
			COALESCE(SUM(CASE WHEN COALESCE(payment_info->>'payment_status','unpaid') NOT IN ('paid') AND NULLIF(payment_info->>'voided_at','') IS NULL THEN total_amount END),0)
		FROM cloud_orders WHERE 1=1%[4]s`, b.todayStart, b.tomorrowStart, b.yesterdayStart, outletFilter)
	if err := run(ordQuery,
		&stats.TotalOrders, &stats.TodayOrders, &stats.TodayOrdersPrev,
		&stats.TotalUnpaidOrders, &stats.TotalUnpaidAmount,
	); err != nil {
		return nil, fmt.Errorf("dashboard stats (orders) failed: %w", err)
	}

	miscQuery := fmt.Sprintf(`
		SELECT
			(SELECT COUNT(*) FROM outlets WHERE 1=1%s),
			(SELECT COUNT(*) FROM outlets WHERE is_active = true%s),
			(SELECT COUNT(*) FROM cloud_products WHERE is_deleted = false%s),
			(SELECT COUNT(*) FROM sync_logs WHERE 1=1%s),
			(SELECT COUNT(*) FROM sync_conflicts WHERE (resolution IS NULL OR resolution = 'pending')%s)`,
		outletsIDFilter, outletsIDFilter, outletFilter, outletFilter, outletFilter)
	if err := run(miscQuery,
		&stats.TotalOutlets, &stats.ActiveOutlets,
		&stats.TotalProducts, &stats.TotalSyncLogs, &stats.PendingConflicts,
	); err != nil {
		return nil, fmt.Errorf("dashboard stats (misc) failed: %w", err)
	}

	outletBase := `
		SELECT
			o.id,
			o.name,
			COALESCE(SUM(CASE WHEN (t.created_at >= ` + b.todayStart + ` AND t.created_at < ` + b.tomorrowStart + `)
				THEN t.total_amount ELSE 0 END), 0)                                       AS sales_day,
			COALESCE(SUM(CASE WHEN (t.created_at >= ` + b.yesterdayStart + ` AND t.created_at < ` + b.todayStart + `)
				THEN t.total_amount ELSE 0 END), 0)                                       AS sales_day_prev,
			COALESCE(SUM(CASE WHEN t.created_at >= ` + b.weekStart + `
				THEN t.total_amount ELSE 0 END), 0)                                       AS sales_week,
			COALESCE(SUM(CASE WHEN t.created_at >= ` + b.weekPrevStart + ` AND t.created_at < ` + b.weekStart + `
				THEN t.total_amount ELSE 0 END), 0)                                       AS sales_week_prev,
			COALESCE(SUM(CASE WHEN t.created_at >= ` + b.monthStart + `
				THEN t.total_amount ELSE 0 END), 0)                                       AS sales_month,
			COALESCE(SUM(CASE WHEN t.created_at >= ` + b.monthPrevStart + ` AND t.created_at < ` + b.monthStart + `
				THEN t.total_amount ELSE 0 END), 0)                                       AS sales_month_prev`

	const outletStdTail = `,
			0::float8                                                                     AS sales_custom,
			0::float8                                                                     AS sales_custom_prev,
			COALESCE(u.cnt, 0)::int                                                       AS unpaid_orders,
			COALESCE(u.amt, 0)                                                            AS unpaid_amount,
			(SELECT TO_CHAR(MAX(sl.created_at), 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
				FROM sync_logs sl WHERE sl.outlet_id = o.id AND sl.action != 'restore') AS last_sync_at
		FROM outlets o
		LEFT JOIN cloud_transactions t ON t.outlet_id = o.id
		LEFT JOIN (
			SELECT outlet_id, COUNT(*) AS cnt, SUM(total_amount) AS amt
			FROM cloud_orders WHERE COALESCE(payment_info->>'payment_status','unpaid') NOT IN ('paid') AND NULLIF(payment_info->>'voided_at','') IS NULL
			GROUP BY outlet_id
		) u ON u.outlet_id = o.id
		WHERE o.is_active = true
		GROUP BY o.id, o.name, u.cnt, u.amt
		ORDER BY sales_month DESC`

	const outletStdTailScoped = `,
			0::float8                                                                     AS sales_custom,
			0::float8                                                                     AS sales_custom_prev,
			COALESCE(u.cnt, 0)::int                                                       AS unpaid_orders,
			COALESCE(u.amt, 0)                                                            AS unpaid_amount,
			(SELECT TO_CHAR(MAX(sl.created_at), 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
				FROM sync_logs sl WHERE sl.outlet_id = o.id AND sl.action != 'restore') AS last_sync_at
		FROM outlets o
		LEFT JOIN cloud_transactions t ON t.outlet_id = o.id
		LEFT JOIN (
			SELECT outlet_id, COUNT(*) AS cnt, SUM(total_amount) AS amt
			FROM cloud_orders WHERE COALESCE(payment_info->>'payment_status','unpaid') NOT IN ('paid') AND NULLIF(payment_info->>'voided_at','') IS NULL
			GROUP BY outlet_id
		) u ON u.outlet_id = o.id
		WHERE o.is_active = true AND o.id = ANY($1)
		GROUP BY o.id, o.name, u.cnt, u.amt
		ORDER BY sales_month DESC`

	outletRangeTail := `,
			COALESCE(SUM(CASE WHEN t.created_at >= ` + b.rangeStart + ` AND t.created_at < ` + b.rangeEnd + `
				THEN t.total_amount ELSE 0 END), 0)                                       AS sales_custom,
			COALESCE(SUM(CASE WHEN t.created_at >= ` + b.prevStart + ` AND t.created_at < ` + b.prevEnd + `
				THEN t.total_amount ELSE 0 END), 0)                                       AS sales_custom_prev,
			COALESCE(u.cnt, 0)::int                                                       AS unpaid_orders,
			COALESCE(u.amt, 0)                                                            AS unpaid_amount,
			(SELECT TO_CHAR(MAX(sl.created_at), 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
				FROM sync_logs sl WHERE sl.outlet_id = o.id AND sl.action != 'restore') AS last_sync_at
		FROM outlets o
		LEFT JOIN cloud_transactions t ON t.outlet_id = o.id
		LEFT JOIN (
			SELECT outlet_id, COUNT(*) AS cnt, SUM(total_amount) AS amt
			FROM cloud_orders WHERE COALESCE(payment_info->>'payment_status','unpaid') NOT IN ('paid') AND NULLIF(payment_info->>'voided_at','') IS NULL
			GROUP BY outlet_id
		) u ON u.outlet_id = o.id
		WHERE o.is_active = true
		GROUP BY o.id, o.name, u.cnt, u.amt
		ORDER BY sales_month DESC`

	outletRangeTailScoped := `,
			COALESCE(SUM(CASE WHEN t.created_at >= ` + b.rangeStart + ` AND t.created_at < ` + b.rangeEnd + `
				THEN t.total_amount ELSE 0 END), 0)                                       AS sales_custom,
			COALESCE(SUM(CASE WHEN t.created_at >= ` + b.prevStart + ` AND t.created_at < ` + b.prevEnd + `
				THEN t.total_amount ELSE 0 END), 0)                                       AS sales_custom_prev,
			COALESCE(u.cnt, 0)::int                                                       AS unpaid_orders,
			COALESCE(u.amt, 0)                                                            AS unpaid_amount,
			(SELECT TO_CHAR(MAX(sl.created_at), 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
				FROM sync_logs sl WHERE sl.outlet_id = o.id AND sl.action != 'restore') AS last_sync_at
		FROM outlets o
		LEFT JOIN cloud_transactions t ON t.outlet_id = o.id
		LEFT JOIN (
			SELECT outlet_id, COUNT(*) AS cnt, SUM(total_amount) AS amt
			FROM cloud_orders WHERE COALESCE(payment_info->>'payment_status','unpaid') NOT IN ('paid') AND NULLIF(payment_info->>'voided_at','') IS NULL
			GROUP BY outlet_id
		) u ON u.outlet_id = o.id
		WHERE o.is_active = true AND o.id = ANY($1)
		GROUP BY o.id, o.name, u.cnt, u.amt
		ORDER BY sales_month DESC`

	var (
		rows    *sql.Rows
		rowsErr error
	)
	if dateFrom != "" && dateTo != "" {
		if scopeIDs != nil {
			rows, rowsErr = database.DB.Query(outletBase+outletRangeTailScoped, pq.Array(scopeIDs))
		} else {
			rows, rowsErr = database.DB.Query(outletBase + outletRangeTail)
		}
	} else {
		if scopeIDs != nil {
			rows, rowsErr = database.DB.Query(outletBase+outletStdTailScoped, pq.Array(scopeIDs))
		} else {
			rows, rowsErr = database.DB.Query(outletBase + outletStdTail)
		}
	}
	if rowsErr != nil {
		return nil, fmt.Errorf("dashboard outlet query failed: %w", rowsErr)
	}
	defer rows.Close()

	stats.Outlets = []models.OutletDashboardRow{}
	for rows.Next() {
		var row models.OutletDashboardRow
		if err := rows.Scan(
			&row.ID, &row.Name,
			&row.SalesDay, &row.SalesDayPrev,
			&row.SalesWeek, &row.SalesWeekPrev,
			&row.SalesMonth, &row.SalesMonthPrev,
			&row.SalesCustom, &row.SalesCustomPrev,
			&row.UnpaidOrders, &row.UnpaidAmount,
			&row.LastSyncAt,
		); err != nil {
			return nil, fmt.Errorf("dashboard outlet scan failed: %w", err)
		}
		stats.Outlets = append(stats.Outlets, row)
	}

	return stats, nil
}

// GetManagerDashboard returns a rich data set for the manager dashboard.
func GetManagerDashboard(scopeIDs []string, dateFrom, dateTo string) (*models.ManagerDashboardStats, error) {
	stats := &models.ManagerDashboardStats{}

	// Semua batas waktu dihitung sekali di Go (lihat computeDashBounds) → literal
	// timestamp; tidak ada tz_day_start()/tz_today() di jalur query panas.
	b := computeDashBounds(dateFrom, dateTo)
	inRange := fmt.Sprintf("(t.created_at >= %s AND t.created_at < %s)", b.rangeStart, b.rangeEnd)
	inPrev := fmt.Sprintf("(t.created_at >= %s AND t.created_at < %s)", b.prevStart, b.prevEnd)

	// Build scope filter helpers
	var scopeParam interface{}
	outletFilter := ""
	outletsIDFilter := ""
	if scopeIDs != nil {
		scopeParam = pq.Array(scopeIDs)
		outletFilter = " AND outlet_id = ANY($1)"
		outletsIDFilter = " AND id = ANY($1)"
	}

	// ── 1. KPI aggregates ──────────────────────────────────
	kpiQuery := fmt.Sprintf(`
		SELECT
			COALESCE(SUM(CASE WHEN (t.created_at >= %[1]s AND t.created_at < %[2]s) THEN t.total_amount ELSE 0 END), 0),
			COALESCE(COUNT(CASE WHEN (t.created_at >= %[1]s AND t.created_at < %[2]s) THEN 1 END), 0),
			COALESCE(SUM(CASE WHEN (t.created_at >= %[3]s AND t.created_at < %[1]s) THEN t.total_amount ELSE 0 END), 0),
			COALESCE(COUNT(CASE WHEN (t.created_at >= %[3]s AND t.created_at < %[1]s) THEN 1 END), 0),
			COALESCE(SUM(CASE WHEN t.created_at >= %[4]s THEN t.total_amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN t.created_at >= %[5]s AND t.created_at < %[4]s THEN t.total_amount ELSE 0 END), 0),
			COALESCE(COUNT(CASE WHEN t.created_at >= %[4]s THEN 1 END), 0),
			COALESCE(COUNT(CASE WHEN t.created_at >= %[5]s AND t.created_at < %[4]s THEN 1 END), 0),
			COALESCE(SUM(CASE WHEN t.created_at >= %[6]s THEN t.total_amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN t.created_at >= %[7]s AND t.created_at < %[6]s THEN t.total_amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN %[8]s THEN t.total_amount ELSE 0 END), 0),
			COALESCE(COUNT(CASE WHEN %[8]s THEN 1 END), 0),
			COALESCE(SUM(CASE WHEN %[9]s THEN t.total_amount ELSE 0 END), 0),
			COALESCE(COUNT(CASE WHEN %[9]s THEN 1 END), 0)
		FROM cloud_transactions t
		WHERE 1=1%[10]s
	`, b.todayStart, b.tomorrowStart, b.yesterdayStart, b.monthStart, b.monthPrevStart,
		b.weekStart, b.weekPrevStart, inRange, inPrev, outletFilter)

	scanKPI := func(row *sql.Row) error {
		return row.Scan(
			&stats.TodayRevenue, &stats.TodayOrders,
			&stats.YesterdayRevenue, &stats.YesterdayOrders,
			&stats.MonthRevenue, &stats.MonthRevenuePrev,
			&stats.MonthOrders, &stats.MonthOrdersPrev,
			&stats.WeekRevenue, &stats.WeekRevenuePrev,
			&stats.RangeRevenue, &stats.RangeOrders,
			&stats.RangeRevenuePrev, &stats.RangeOrdersPrev,
		)
	}
	var kpiErr error
	if scopeParam != nil {
		kpiErr = scanKPI(database.DB.QueryRow(kpiQuery, scopeParam))
	} else {
		kpiErr = scanKPI(database.DB.QueryRow(kpiQuery))
	}
	if kpiErr != nil {
		return nil, fmt.Errorf("manager dashboard KPI query failed: %w", kpiErr)
	}

	if stats.TodayOrders > 0 {
		stats.TodayAvgOrder = stats.TodayRevenue / float64(stats.TodayOrders)
	}
	if stats.RangeOrders > 0 {
		stats.RangeAvgOrder = stats.RangeRevenue / float64(stats.RangeOrders)
	}

	// ── Pax (jumlah tamu) ──────────────────────────────────
	// pax tersimpan di cloud_orders; dashboard berbasis transaksi (pembayaran).
	// Dihitung per ORDER (anti dobel saat split bill) memakai waktu bayar terakhir.
	// Set-based: satu agregat waktu bayar per order (GROUP BY) lalu JOIN — bukan
	// subquery berkorelasi per baris (yang sebelumnya membuat query ~1,3 detik).
	// Bandingkan langsung ke batas instant UTC (rangeStart..rangeEnd) — bucket per
	// hari lokal identik, tanpa perlu tz_date per baris.
	paxScope := ""
	if scopeIDs != nil {
		paxScope = " AND o2.outlet_id = ANY($1)"
	}
	paxQuery := fmt.Sprintf(`
		WITH pay AS (
			SELECT order_id AS oid, MAX(created_at) AS last_paid
			FROM cloud_transactions GROUP BY order_id
		)
		SELECT o2.outlet_id,
			COALESCE(SUM(CASE WHEN pay.last_paid >= %[1]s AND pay.last_paid < %[2]s THEN o2.pax ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN pay.last_paid >= %[3]s AND pay.last_paid < %[4]s THEN o2.pax ELSE 0 END),0)
		FROM cloud_orders o2
		JOIN pay ON pay.oid = TRIM(o2.id)
		WHERE COALESCE(o2.pax,0) > 0%[5]s
		GROUP BY o2.outlet_id`, b.rangeStart, b.rangeEnd, b.prevStart, b.prevEnd, paxScope)

	paxByOutlet := map[string]int{}
	var pobRows *sql.Rows
	if scopeParam != nil {
		pobRows, _ = database.DB.Query(paxQuery, scopeParam)
	} else {
		pobRows, _ = database.DB.Query(paxQuery)
	}
	if pobRows != nil {
		for pobRows.Next() {
			var id string
			var px, pxPrev int
			if pobRows.Scan(&id, &px, &pxPrev) == nil {
				paxByOutlet[strings.TrimSpace(id)] = px
				stats.RangePax += px
				stats.RangePaxPrev += pxPrev
			}
		}
		pobRows.Close()
	}

	// Active outlets, products, unpaid
	countQuery := fmt.Sprintf(`
		SELECT
			(SELECT COUNT(*) FROM outlets WHERE is_active = true%s),
			(SELECT COUNT(*) FROM cloud_products WHERE is_deleted = false%s),
			(SELECT COUNT(*) FROM cloud_orders WHERE COALESCE(payment_info->>'payment_status','unpaid') NOT IN ('paid') AND NULLIF(payment_info->>'voided_at','') IS NULL%s),
			(SELECT COALESCE(SUM(total_amount),0) FROM cloud_orders WHERE COALESCE(payment_info->>'payment_status','unpaid') NOT IN ('paid') AND NULLIF(payment_info->>'voided_at','') IS NULL%s)
	`, outletsIDFilter, outletFilter, outletFilter, outletFilter)

	if scopeParam != nil {
		_ = database.DB.QueryRow(countQuery, scopeParam).Scan(&stats.ActiveOutlets, &stats.TotalProducts, &stats.UnpaidOrders, &stats.UnpaidAmount)
	} else {
		_ = database.DB.QueryRow(countQuery).Scan(&stats.ActiveOutlets, &stats.TotalProducts, &stats.UnpaidOrders, &stats.UnpaidAmount)
	}

	// ── 2+3. Revenue & order trend (satu scan untuk dua seri) ──
	// Batas per-hari dikonversi via zona waktu LITERAL (bukan tz_day_start yang
	// baca app_settings): 'YYYY-MM-DD'::timestamp AT TIME ZONE '<tz>' → instant UTC.
	// Hanya dievaluasi per baris deret (≤ rentang hari), bukan per transaksi.
	trendQuery := fmt.Sprintf(`
		SELECT TO_CHAR(d.dt, 'YYYY-MM-DD'), COALESCE(SUM(t.total_amount), 0), COALESCE(COUNT(t.id), 0)
		FROM generate_series(%s, %s, '1 day'::interval) d(dt)
		LEFT JOIN cloud_transactions t ON (
			t.created_at >= ((d.dt::date::timestamp AT TIME ZONE %s) AT TIME ZONE 'UTC') AND
			t.created_at <  (((d.dt::date + 1)::timestamp AT TIME ZONE %s) AT TIME ZONE 'UTC'))%s
		GROUP BY d.dt ORDER BY d.dt
	`, b.fromDate, b.toDate, tzLit(b.tz), tzLit(b.tz), outletFilter)

	// Pax per hari (jumlah tamu, per hari lokal berdasarkan waktu bayar terakhir
	// order) — untuk menghitung basket size (pendapatan/pax) di grafik tren.
	paxTrendQuery := fmt.Sprintf(`
		WITH pay AS (SELECT order_id AS oid, MAX(created_at) AS last_paid FROM cloud_transactions GROUP BY order_id)
		SELECT TO_CHAR(((pay.last_paid AT TIME ZONE 'UTC') AT TIME ZONE %[1]s)::date, 'YYYY-MM-DD'),
			COALESCE(SUM(o2.pax),0)::int
		FROM cloud_orders o2 JOIN pay ON pay.oid = TRIM(o2.id)
		WHERE COALESCE(o2.pax,0) > 0 AND pay.last_paid >= %[2]s AND pay.last_paid < %[3]s%[4]s
		GROUP BY 1`, tzLit(b.tz), b.rangeStart, b.rangeEnd, func() string {
		if scopeIDs != nil {
			return " AND o2.outlet_id = ANY($1)"
		}
		return ""
	}())
	paxByDay := map[string]int{}
	var ptRows *sql.Rows
	if scopeParam != nil {
		ptRows, _ = database.DB.Query(paxTrendQuery, scopeParam)
	} else {
		ptRows, _ = database.DB.Query(paxTrendQuery)
	}
	if ptRows != nil {
		for ptRows.Next() {
			var d string
			var px int
			if ptRows.Scan(&d, &px) == nil {
				paxByDay[d] = px
			}
		}
		ptRows.Close()
	}

	stats.RevenueTrend = []models.DailyPoint{}
	stats.OrderTrend = []models.DailyPoint{}
	stats.PaxTrend = []models.DailyPoint{}
	var trendRows *sql.Rows
	var trendErr error
	if scopeParam != nil {
		trendRows, trendErr = database.DB.Query(trendQuery, scopeParam)
	} else {
		trendRows, trendErr = database.DB.Query(trendQuery)
	}
	if trendErr == nil {
		defer trendRows.Close()
		for trendRows.Next() {
			var date string
			var revenue, count float64
			if err := trendRows.Scan(&date, &revenue, &count); err == nil {
				stats.RevenueTrend = append(stats.RevenueTrend, models.DailyPoint{Date: date, Value: revenue})
				stats.OrderTrend = append(stats.OrderTrend, models.DailyPoint{Date: date, Value: count})
				stats.PaxTrend = append(stats.PaxTrend, models.DailyPoint{Date: date, Value: float64(paxByDay[date])})
			}
		}
	}

	// ── 4. Hourly sales pattern (today) ────────────────────
	hourlyQuery := fmt.Sprintf(`
		SELECT h.hour, COALESCE(SUM(t.total_amount), 0), COALESCE(COUNT(t.id), 0)
		FROM generate_series(0, 23) h(hour)
		LEFT JOIN cloud_transactions t ON tz_hour(t.created_at) = h.hour
		  AND `+inRange+`%s
		GROUP BY h.hour ORDER BY h.hour
	`, outletFilter)

	stats.HourlySales = []models.HourlyPoint{}
	var hRows *sql.Rows
	var hErr error
	if scopeParam != nil {
		hRows, hErr = database.DB.Query(hourlyQuery, scopeParam)
	} else {
		hRows, hErr = database.DB.Query(hourlyQuery)
	}
	if hErr == nil {
		defer hRows.Close()
		for hRows.Next() {
			var p models.HourlyPoint
			if err := hRows.Scan(&p.Hour, &p.Value, &p.Count); err == nil {
				stats.HourlySales = append(stats.HourlySales, p)
			}
		}
	}

	// ── 5. Payment methods (range) ─────────────────────────
	// Dari transaction_payments agar transaksi multi-metode (header 'mixed')
	// terpecah benar per metode. outlet_id & created_at didenormalisasi dari transaksi.
	// Komplimen dikecualikan: nilainya Rp0 (gratis) sehingga hanya muncul sebagai
	// "compliment 0%" yang mengganggu di kartu Metode Pembayaran.
	pmQuery := fmt.Sprintf(`
		SELECT COALESCE(payment_method, 'other'), COALESCE(SUM(amount), 0), COUNT(*)
		FROM transaction_payments
		WHERE created_at >= %s AND created_at < %s%s
		  AND lower(COALESCE(payment_method,'')) <> 'compliment'
		GROUP BY payment_method ORDER BY SUM(amount) DESC
	`, b.rangeStart, b.rangeEnd, outletFilter)

	stats.PaymentMethods = []models.PaymentMethodShare{}
	var pmRows *sql.Rows
	var pmErr error
	if scopeParam != nil {
		pmRows, pmErr = database.DB.Query(pmQuery, scopeParam)
	} else {
		pmRows, pmErr = database.DB.Query(pmQuery)
	}
	if pmErr == nil {
		defer pmRows.Close()
		for pmRows.Next() {
			var p models.PaymentMethodShare
			if err := pmRows.Scan(&p.Method, &p.Amount, &p.Count); err == nil {
				stats.PaymentMethods = append(stats.PaymentMethods, p)
			}
		}
	}

	// ── 6. Top 10 products (this month by qty) ─────────────
	// Item penjualan disimpan sebagai JSONB di kolom cloud_transactions.items
	// (bukan tabel terpisah). Setiap elemen: { product_name, quantity, subtotal }.
	topQuery := fmt.Sprintf(`
		SELECT item->>'product_name' AS name,
			COALESCE(SUM((item->>'quantity')::numeric), 0)::int AS qty,
			COALESCE(SUM((item->>'subtotal')::numeric), 0) AS revenue
		FROM cloud_transactions t,
			jsonb_array_elements(t.items) AS item
		WHERE `+inRange+`
		  AND COALESCE(item->>'product_name','') <> ''%s
		GROUP BY item->>'product_name'
		ORDER BY qty DESC LIMIT 10
	`, func() string {
		if scopeIDs != nil {
			return " AND t.outlet_id = ANY($1)"
		}
		return ""
	}())

	stats.TopProducts = []models.TopProductRow{}
	var tpRows *sql.Rows
	var tpErr error
	if scopeParam != nil {
		tpRows, tpErr = database.DB.Query(topQuery, scopeParam)
	} else {
		tpRows, tpErr = database.DB.Query(topQuery)
	}
	if tpErr == nil {
		defer tpRows.Close()
		for tpRows.Next() {
			var p models.TopProductRow
			if err := tpRows.Scan(&p.Name, &p.Quantity, &p.Revenue); err == nil {
				stats.TopProducts = append(stats.TopProducts, p)
			}
		}
	}

	// ── 7. Outlet ranking ──────────────────────────────────
	outletQuery := fmt.Sprintf(`
		SELECT o.id, o.name,
			COALESCE(SUM(CASE WHEN (t.created_at >= `+b.todayStart+` AND t.created_at < `+b.tomorrowStart+`) THEN t.total_amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN t.created_at >= `+b.monthStart+` THEN t.total_amount ELSE 0 END), 0),
			COALESCE(COUNT(CASE WHEN (t.created_at >= `+b.todayStart+` AND t.created_at < `+b.tomorrowStart+`) THEN 1 END), 0)::int,
			COALESCE(COUNT(CASE WHEN t.created_at >= `+b.monthStart+` THEN 1 END), 0)::int,
			COALESCE(SUM(CASE WHEN `+inRange+` THEN t.total_amount ELSE 0 END), 0),
			COALESCE(COUNT(CASE WHEN `+inRange+` THEN 1 END), 0)::int,
			(SELECT COALESCE(SUM(total_amount),0) FROM cloud_orders co
				WHERE co.outlet_id = o.id
				AND COALESCE(co.payment_info->>'payment_status','unpaid') NOT IN ('paid')
				AND NULLIF(co.payment_info->>'voided_at','') IS NULL),
			(SELECT TO_CHAR(MAX(sl.created_at), 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
				FROM sync_logs sl WHERE sl.outlet_id = o.id AND sl.action != 'restore')
		FROM outlets o
		LEFT JOIN cloud_transactions t ON t.outlet_id = o.id
		WHERE o.is_active = true%s
		GROUP BY o.id, o.name
		ORDER BY SUM(CASE WHEN `+inRange+` THEN t.total_amount ELSE 0 END) DESC
	`, func() string {
		// Query ini JOIN outlets + cloud_transactions yang dua-duanya punya
		// kolom `id`, jadi filter scope harus pakai `o.id` (bukan `id` polos)
		// agar tidak error "ambiguous column" yang membuat Performa Outlet kosong.
		if scopeIDs != nil {
			return " AND o.id = ANY($1)"
		}
		return ""
	}())

	// Total pendapatan SELURUH outlet aktif pada rentang (tanpa filter scope).
	// Dipakai frontend untuk persentase kontribusi terhadap keseluruhan perusahaan.
	database.DB.QueryRow(
		`SELECT COALESCE(SUM(t.total_amount),0)
		 FROM cloud_transactions t
		 JOIN outlets o ON o.id = t.outlet_id
		 WHERE o.is_active = true AND `+inRange,
	).Scan(&stats.GlobalRangeRevenue)

	stats.OutletRanking = []models.OutletRankRow{}
	var orRows *sql.Rows
	var orErr error
	if scopeParam != nil {
		orRows, orErr = database.DB.Query(outletQuery, scopeParam)
	} else {
		orRows, orErr = database.DB.Query(outletQuery)
	}
	if orErr == nil {
		defer orRows.Close()
		for orRows.Next() {
			var r models.OutletRankRow
			if err := orRows.Scan(&r.ID, &r.Name, &r.TodayRevenue, &r.MonthRevenue, &r.TodayOrders, &r.MonthOrders, &r.RangeRevenue, &r.RangeOrders, &r.UnpaidAmount, &r.LastSyncAt); err == nil {
				r.RangePax = paxByOutlet[r.ID]
				stats.OutletRanking = append(stats.OutletRanking, r)
			}
		}
	}

	return stats, nil
}
