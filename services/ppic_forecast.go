package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"fmt"
	"log"

	"github.com/lib/pq"
)

// Penjualan harian per (outlet, produk) dari cloud_orders.items (JSONB), pola
// yang sama dengan laporan Penjualan Produk: order void & titipan dikecualikan,
// tanggal dalam zona Asia/Jakarta, produk dikenali per NAMA (konsisten dengan
// laporan; payload order tidak selalu membawa product_id).
const forecastSalesCTE = `
	SELECT o.outlet_id,
		COALESCE(NULLIF(item->>'product_name', ''), 'Unknown') AS product_name,
		DATE(o.created_at AT TIME ZONE 'Asia/Jakarta') AS sale_date,
		SUM(COALESCE((item->>'qty')::numeric, 0)) AS qty
	FROM cloud_orders o,
		jsonb_array_elements(COALESCE(o.items, '[]'::jsonb)) AS item
	WHERE o.created_at >= NOW() - INTERVAL '35 days'
		AND NULLIF(o.payment_info->>'voided_at', '') IS NULL
		AND COALESCE(o.is_holding, false) = false
	GROUP BY 1, 2, 3`

// GenerateForecasts menghitung forecast qty_system per produk per outlet per hari
// untuk `horizon` hari ke depan, metode weighted moving average per hari-dalam-
// minggu (4 minggu terakhir, bobot 40/30/20/10 — pola mingguan khas F&B).
// qty_manual yang sudah diisi PPIC TIDAK ditimpa. Produk = yang pernah terjual
// dalam 28 hari terakhir di outlet tersebut.
func GenerateForecasts(outletIDs []string, horizon int, actor string) (int, error) {
	if horizon <= 0 {
		horizon = 7
	}
	if horizon > 28 {
		horizon = 28
	}

	scopeCond := ""
	args := []interface{}{horizon}
	if outletIDs != nil {
		args = append(args, pq.Array(outletIDs))
		scopeCond = " WHERE p.outlet_id = ANY($2)"
	}

	q := fmt.Sprintf(`
		WITH sales AS (%s),
		products AS (
			SELECT DISTINCT outlet_id, product_name FROM sales
			WHERE sale_date >= CURRENT_DATE - 28
		),
		targets AS (
			SELECT (CURRENT_DATE + g.i)::date AS fdate FROM generate_series(0, $1 - 1) g(i)
		)
		INSERT INTO demand_forecasts (id, outlet_id, product_name, forecast_date, qty_system, method, updated_by, updated_at)
		SELECT %s, p.outlet_id, p.product_name, t.fdate,
			ROUND(0.4*COALESCE(s1.qty,0) + 0.3*COALESCE(s2.qty,0) + 0.2*COALESCE(s3.qty,0) + 0.1*COALESCE(s4.qty,0), 0),
			'wma_dow', '%s', NOW() AT TIME ZONE 'UTC'
		FROM products p
		CROSS JOIN targets t
		LEFT JOIN sales s1 ON s1.outlet_id = p.outlet_id AND s1.product_name = p.product_name AND s1.sale_date = t.fdate - 7
		LEFT JOIN sales s2 ON s2.outlet_id = p.outlet_id AND s2.product_name = p.product_name AND s2.sale_date = t.fdate - 14
		LEFT JOIN sales s3 ON s3.outlet_id = p.outlet_id AND s3.product_name = p.product_name AND s3.sale_date = t.fdate - 21
		LEFT JOIN sales s4 ON s4.outlet_id = p.outlet_id AND s4.product_name = p.product_name AND s4.sale_date = t.fdate - 28
		%s
		ON CONFLICT (outlet_id, product_name, forecast_date) DO UPDATE SET
			qty_system = EXCLUDED.qty_system, method = EXCLUDED.method,
			updated_at = NOW() AT TIME ZONE 'UTC'`,
		forecastSalesCTE,
		// ULID per baris tidak bisa dari Go di INSERT..SELECT — pakai fungsi acak
		// 26 char (bukan ULID asli, cukup unik & panjang kolom sama).
		`UPPER(SUBSTR(MD5(random()::text || clock_timestamp()::text), 1, 26))`,
		actor, scopeCond)

	res, err := database.DB.Exec(q, args...)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// FillForecastActuals mengisi qty_actual untuk tanggal yang sudah lewat (dipanggil
// scheduler harian). Baris tanpa penjualan diisi 0 supaya evaluasi jujur.
func FillForecastActuals() error {
	if _, err := database.DB.Exec(fmt.Sprintf(`
		WITH sales AS (%s)
		UPDATE demand_forecasts df SET qty_actual = s.qty
		FROM sales s
		WHERE df.outlet_id = s.outlet_id AND df.product_name = s.product_name
		AND df.forecast_date = s.sale_date
		AND df.forecast_date < CURRENT_DATE AND df.qty_actual IS NULL`, forecastSalesCTE)); err != nil {
		return err
	}
	_, err := database.DB.Exec(`
		UPDATE demand_forecasts SET qty_actual = 0
		WHERE forecast_date < CURRENT_DATE AND forecast_date >= CURRENT_DATE - 35
		AND qty_actual IS NULL`)
	return err
}

// forecastQtyExpr — angka forecast efektif: manual bila diisi, selain itu sistem.
const forecastQtyExpr = `COALESCE(df.qty_manual, df.qty_system)`

// ListForecasts mengembalikan matriks produk × tanggal + evaluasi akurasi.
func ListForecasts(outletID, search, category, dateFrom, dateTo string, outletScope []string, page, limit int) (*models.ForecastResponse, error) {
	where := ` WHERE df.forecast_date >= $1::date AND df.forecast_date <= $2::date`
	args := []interface{}{dateFrom, dateTo}
	if outletID != "" {
		args = append(args, outletID)
		where += fmt.Sprintf(" AND df.outlet_id = $%d", len(args))
	} else if outletScope != nil {
		args = append(args, pq.Array(outletScope))
		where += fmt.Sprintf(" AND df.outlet_id = ANY($%d)", len(args))
	}
	if search != "" {
		args = append(args, "%"+search+"%")
		where += fmt.Sprintf(" AND df.product_name ILIKE $%d", len(args))
	}
	if category != "" {
		args = append(args, category)
		where += fmt.Sprintf(` AND EXISTS (
			SELECT 1 FROM cloud_products p
			WHERE p.outlet_id = df.outlet_id AND p.name = df.product_name
			AND COALESCE(p.is_deleted, false) = false AND p.category_name = $%d)`, len(args))
	}

	resp := &models.ForecastResponse{Dates: []string{}, Rows: []models.ForecastRow{}, Categories: []string{}}

	// Daftar kategori dalam scope — untuk dropdown filter.
	catArgs := []interface{}{}
	catCond := ""
	if outletID != "" {
		catArgs = append(catArgs, outletID)
		catCond = " AND df.outlet_id = $1"
	} else if outletScope != nil {
		catArgs = append(catArgs, pq.Array(outletScope))
		catCond = " AND df.outlet_id = ANY($1)"
	}
	if catRows, err := database.DB.Query(fmt.Sprintf(`
		SELECT DISTINCT p.category_name
		FROM demand_forecasts df
		JOIN cloud_products p ON p.outlet_id = df.outlet_id AND p.name = df.product_name
		WHERE COALESCE(p.category_name, '') <> '' AND COALESCE(p.is_deleted, false) = false%s
		ORDER BY 1`, catCond), catArgs...); err == nil {
		for catRows.Next() {
			var c string
			if catRows.Scan(&c) == nil {
				resp.Categories = append(resp.Categories, c)
			}
		}
		catRows.Close()
	}

	// Daftar tanggal pada rentang (kolom matriks).
	dRows, err := database.DB.Query(`SELECT TO_CHAR(d.dt, 'YYYY-MM-DD') FROM generate_series($1::date, $2::date, '1 day') d(dt)`, dateFrom, dateTo)
	if err != nil {
		return nil, err
	}
	for dRows.Next() {
		var s string
		if dRows.Scan(&s) == nil {
			resp.Dates = append(resp.Dates, s)
		}
	}
	dRows.Close()

	// Produk terpaginasi (baris matriks).
	if err := database.DB.QueryRow(`SELECT COUNT(DISTINCT (df.outlet_id, df.product_name)) FROM demand_forecasts df`+where, args...).
		Scan(&resp.Total); err != nil {
		return nil, err
	}
	prodQ := fmt.Sprintf(`
		SELECT df.outlet_id, COALESCE(o.name, df.outlet_id), df.product_name,
			COALESCE((SELECT p.category_name FROM cloud_products p
				WHERE p.outlet_id = df.outlet_id AND p.name = df.product_name
				AND COALESCE(p.is_deleted, false) = false
				AND COALESCE(p.category_name, '') <> '' LIMIT 1), '')
		FROM demand_forecasts df
		LEFT JOIN outlets o ON o.id = df.outlet_id
		%s
		GROUP BY df.outlet_id, o.name, df.product_name
		ORDER BY SUM(%s) DESC, df.product_name
		LIMIT %d OFFSET %d`, where, forecastQtyExpr, limit, (page-1)*limit)
	pRows, err := database.DB.Query(prodQ, args...)
	if err != nil {
		return nil, err
	}
	type key struct{ outlet, product string }
	index := map[key]int{}
	for pRows.Next() {
		var r models.ForecastRow
		if err := pRows.Scan(&r.OutletID, &r.OutletName, &r.ProductName, &r.Category); err != nil {
			pRows.Close()
			return nil, err
		}
		// Sel default per tanggal (baris tanpa data tetap tampil utuh).
		for _, d := range resp.Dates {
			r.Cells = append(r.Cells, models.ForecastCell{Date: d})
		}
		index[key{r.OutletID, r.ProductName}] = len(resp.Rows)
		resp.Rows = append(resp.Rows, r)
	}
	pRows.Close()

	if len(resp.Rows) > 0 {
		// Isi sel untuk produk yang tampil.
		cRows, err := database.DB.Query(fmt.Sprintf(`
			SELECT df.outlet_id, df.product_name, TO_CHAR(df.forecast_date, 'YYYY-MM-DD'),
				df.qty_system, df.qty_manual, df.qty_actual, COALESCE(df.event_note, '')
			FROM demand_forecasts df %s`, where), args...)
		if err != nil {
			return nil, err
		}
		datePos := map[string]int{}
		for i, d := range resp.Dates {
			datePos[d] = i
		}
		for cRows.Next() {
			var outlet, product, date, note string
			var qs float64
			var qm, qa *float64
			if cRows.Scan(&outlet, &product, &date, &qs, &qm, &qa, &note) != nil {
				continue
			}
			if ri, ok := index[key{outlet, product}]; ok {
				if di, ok := datePos[date]; ok {
					resp.Rows[ri].Cells[di] = models.ForecastCell{Date: date, QtySystem: qs, QtyManual: qm, QtyActual: qa, EventNote: note}
				}
			}
		}
		cRows.Close()

		// Akurasi per produk, 14 hari terakhir (aktual > 0).
		accArgs := []interface{}{}
		accScope := ""
		if outletID != "" {
			accArgs = append(accArgs, outletID)
			accScope = " AND df.outlet_id = $1"
		} else if outletScope != nil {
			accArgs = append(accArgs, pq.Array(outletScope))
			accScope = " AND df.outlet_id = ANY($1)"
		}
		aRows, err := database.DB.Query(fmt.Sprintf(`
			SELECT df.outlet_id, df.product_name, COUNT(*),
				AVG(ABS(df.qty_actual - %s) / df.qty_actual)
			FROM demand_forecasts df
			WHERE df.qty_actual > 0 AND df.forecast_date >= CURRENT_DATE - 14 AND df.forecast_date < CURRENT_DATE%s
			GROUP BY df.outlet_id, df.product_name`, forecastQtyExpr, accScope), accArgs...)
		if err == nil {
			for aRows.Next() {
				var outlet, product string
				var n int
				var mape float64
				if aRows.Scan(&outlet, &product, &n, &mape) != nil {
					continue
				}
				if ri, ok := index[key{outlet, product}]; ok {
					acc := (1 - mape) * 100
					if acc < 0 {
						acc = 0
					}
					resp.Rows[ri].AccuracyPct = acc
					resp.Rows[ri].EvalRows = n
				}
			}
			aRows.Close()
		}
	}

	// Akurasi agregat 14 hari (untuk kartu ringkasan halaman).
	resp.Accuracy14d, resp.EvalRows14d = forecastAccuracy(14, outletScope)
	return resp, nil
}

// forecastAccuracy menghitung 100 − MAPE untuk `days` hari terakhir.
func forecastAccuracy(days int, outletScope []string) (float64, int) {
	cond := ""
	args := []interface{}{days}
	if outletScope != nil {
		args = append(args, pq.Array(outletScope))
		cond = " AND df.outlet_id = ANY($2)"
	}
	var mape float64
	var n int
	err := database.DB.QueryRow(fmt.Sprintf(`
		SELECT COALESCE(AVG(ABS(df.qty_actual - %s) / df.qty_actual), 0), COUNT(*)
		FROM demand_forecasts df
		WHERE df.qty_actual > 0 AND df.forecast_date >= CURRENT_DATE - $1 AND df.forecast_date < CURRENT_DATE%s`,
		forecastQtyExpr, cond), args...).Scan(&mape, &n)
	if err != nil || n == 0 {
		return 0, 0
	}
	acc := (1 - mape) * 100
	if acc < 0 {
		acc = 0
	}
	return acc, n
}

// UpdateForecasts menyimpan penyesuaian manual PPIC (bulk). qty_manual null
// mengembalikan sel ke angka sistem.
func UpdateForecasts(req models.ForecastUpdateRequest, actor string) (int, error) {
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
		if r.OutletID == "" || r.ProductName == "" || r.Date == "" {
			return 0, fmt.Errorf("outlet, produk, dan tanggal wajib diisi")
		}
		if r.QtyManual != nil && *r.QtyManual < 0 {
			return 0, fmt.Errorf("qty forecast tidak boleh negatif")
		}
		if _, err := tx.Exec(`
			INSERT INTO demand_forecasts (id, outlet_id, product_name, forecast_date, qty_system, qty_manual, event_note, method, updated_by, updated_at)
			VALUES ($1, $2, $3, $4::date, 0, $5, $6, 'manual', $7, NOW() AT TIME ZONE 'UTC')
			ON CONFLICT (outlet_id, product_name, forecast_date) DO UPDATE SET
				qty_manual = EXCLUDED.qty_manual, event_note = EXCLUDED.event_note,
				updated_by = EXCLUDED.updated_by, updated_at = NOW() AT TIME ZONE 'UTC'`,
			NewULID(), r.OutletID, r.ProductName, r.Date, r.QtyManual, r.EventNote, actor); err != nil {
			return 0, err
		}
		n++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return n, nil
}

// GetForecastSalesHistory — penjualan harian 35 hari terakhir untuk satu produk
// di satu outlet (hari tanpa penjualan = 0). Dipakai UI untuk menampilkan
// histori yang menjadi bahan rumus WMA per hari-dalam-minggu.
func GetForecastSalesHistory(outletID, productName string) ([]models.ForecastHistoryPoint, error) {
	if outletID == "" || productName == "" {
		return nil, fmt.Errorf("outlet_id dan product wajib diisi")
	}
	rows, err := database.DB.Query(`
		WITH sales AS (
			SELECT DATE(o.created_at AT TIME ZONE 'Asia/Jakarta') AS d,
				SUM(COALESCE((item->>'qty')::numeric, 0)) AS qty
			FROM cloud_orders o,
				jsonb_array_elements(COALESCE(o.items, '[]'::jsonb)) AS item
			WHERE o.outlet_id = $1
				AND COALESCE(NULLIF(item->>'product_name', ''), 'Unknown') = $2
				AND o.created_at >= NOW() - INTERVAL '36 days'
				AND NULLIF(o.payment_info->>'voided_at', '') IS NULL
				AND COALESCE(o.is_holding, false) = false
			GROUP BY 1
		)
		SELECT TO_CHAR(g.dt, 'YYYY-MM-DD'), COALESCE(s.qty, 0)
		FROM generate_series(CURRENT_DATE - 34, CURRENT_DATE, '1 day'::interval) g(dt)
		LEFT JOIN sales s ON s.d = g.dt
		ORDER BY g.dt`, outletID, productName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.ForecastHistoryPoint{}
	for rows.Next() {
		var p models.ForecastHistoryPoint
		if rows.Scan(&p.Date, &p.Qty) == nil {
			out = append(out, p)
		}
	}
	return out, nil
}

// runNightlyForecastJob dipanggil scheduler harian: isi aktual kemarin lalu
// generate ulang horizon 7 hari.
func runNightlyForecastJob() {
	if err := FillForecastActuals(); err != nil {
		log.Printf("[PPIC] Isi aktual forecast gagal: %v", err)
	}
	if n, err := GenerateForecasts(nil, 7, "system"); err != nil {
		log.Printf("[PPIC] Generate forecast gagal: %v", err)
	} else {
		log.Printf("[PPIC] Forecast digenerate: %d baris", n)
	}
}
