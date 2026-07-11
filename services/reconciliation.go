package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/lib/pq"
)

// reconShiftJSON — bagian report shift yang dipakai rekonsiliasi.
type reconShiftJSON struct {
	SalesTotal float64 `json:"sales_total"`
	SalesCount int     `json:"sales_count"`
	ByCashier  []struct {
		Cashier string `json:"cashier"`
	} `json:"by_cashier"`
}

// cloudSalesInWindow menghitung penjualan yang benar-benar masuk cloud untuk satu
// shift (jendela buka→tutup), TIDAK termasuk komplimen (bukan penjualan) maupun
// transaksi penyesuaian (agar rekonsiliasi selalu terhadap data asli).
func cloudSalesInWindow(outletID string, buka, tutup time.Time) (float64, int) {
	var total float64
	var count int
	database.DB.QueryRow(`
		SELECT COALESCE(SUM(total_amount),0), COUNT(*)
		FROM cloud_transactions
		WHERE TRIM(outlet_id) = TRIM($1)
		  AND created_at >= $2 AND created_at <= $3
		  AND lower(COALESCE(payment_method,'')) NOT IN ('compliment','adjustment')`,
		outletID, buka, tutup,
	).Scan(&total, &count)
	return round2(total), count
}

// GetShiftReconciliation membandingkan penjualan versi kasir (report shift) dengan
// yang masuk cloud, untuk tiap shift TERTUTUP pada rentang tanggal (scoped).
func GetShiftReconciliation(dateFrom, dateTo, outletID string, outletScope []string) (*models.ShiftReconReport, error) {
	conds := []string{"s.status = 'closed'"}
	args := []interface{}{}
	idx := 1
	if outletID != "" {
		conds = append(conds, fmt.Sprintf("s.outlet_id = $%d", idx))
		args = append(args, outletID)
		idx++
	} else if outletScope != nil {
		conds = append(conds, fmt.Sprintf("s.outlet_id = ANY($%d::text[])", idx))
		args = append(args, pq.Array(outletScope))
		idx++
	}
	if dateFrom != "" {
		conds = append(conds, fmt.Sprintf("tz_date(s.created_at) >= $%d::date", idx))
		args = append(args, dateFrom)
		idx++
	}
	if dateTo != "" {
		conds = append(conds, fmt.Sprintf("tz_date(s.created_at) <= $%d::date", idx))
		args = append(args, dateTo)
		idx++
	}

	q := fmt.Sprintf(`
		SELECT TRIM(s.id), TRIM(s.outlet_id), COALESCE(o.name, s.outlet_id),
			to_char((s.created_at AT TIME ZONE 'UTC') AT TIME ZONE %[1]s, 'YYYY-MM-DD HH24:MI'),
			to_char((s.updated_at AT TIME ZONE 'UTC') AT TIME ZONE %[1]s, 'YYYY-MM-DD HH24:MI'),
			s.created_at, s.updated_at,
			COALESCE(s.report::text, '{}')
		FROM cloud_cashier_shifts s
		LEFT JOIN outlets o ON o.id = s.outlet_id
		WHERE %[2]s
		ORDER BY s.created_at DESC`, tzExpr, strings.Join(conds, " AND "))

	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	report := &models.ShiftReconReport{Shifts: []models.ShiftReconRow{}}
	type pending struct {
		row         models.ShiftReconRow
		buka, tutup time.Time
	}
	var list []pending
	shiftIDs := []string{}
	for rows.Next() {
		var r models.ShiftReconRow
		var buka, tutup time.Time
		var reportJSON string
		if err := rows.Scan(&r.ShiftID, &r.OutletID, &r.OutletName,
			&r.OpenedAt, &r.ClosedAt, &buka, &tutup, &reportJSON); err != nil {
			return nil, err
		}
		var rj reconShiftJSON
		json.Unmarshal([]byte(reportJSON), &rj)
		r.CashierSales = round2(rj.SalesTotal)
		r.CashierCount = rj.SalesCount
		names := make([]string, 0, len(rj.ByCashier))
		for _, c := range rj.ByCashier {
			if strings.TrimSpace(c.Cashier) != "" {
				names = append(names, c.Cashier)
			}
		}
		r.Cashiers = strings.Join(names, ", ")
		list = append(list, pending{r, buka, tutup})
		shiftIDs = append(shiftIDs, r.ShiftID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Penyesuaian aktif per shift.
	adjByShift := map[string]struct {
		id     int64
		amount float64
	}{}
	if len(shiftIDs) > 0 {
		arows, aerr := database.DB.Query(`
			SELECT TRIM(shift_id), id, adjustment_amount
			FROM shift_revenue_adjustments
			WHERE status='applied' AND TRIM(shift_id) = ANY($1::text[])`, pq.Array(shiftIDs))
		if aerr == nil {
			defer arows.Close()
			for arows.Next() {
				var sid string
				var id int64
				var amt float64
				if arows.Scan(&sid, &id, &amt) == nil {
					adjByShift[sid] = struct {
						id     int64
						amount float64
					}{id, amt}
				}
			}
		}
	}

	for _, p := range list {
		r := p.row
		r.CloudSales, r.CloudCount = cloudSalesInWindow(r.OutletID, p.buka, p.tutup)
		r.Diff = round2(r.CashierSales - r.CloudSales)
		if a, ok := adjByShift[r.ShiftID]; ok {
			r.Adjusted = true
			r.AdjustmentID = a.id
			r.AdjustmentAmount = a.amount
		}
		switch {
		case math.Abs(r.Diff) < 1:
			r.Status = "balanced"
			report.Summary.BalancedCount++
		case r.Diff > 0:
			r.Status = "short"
			report.Summary.ShortCount++
			report.Summary.ShortTotal = round2(report.Summary.ShortTotal + r.Diff)
		default:
			r.Status = "over"
			report.Summary.OverCount++
			report.Summary.OverTotal = round2(report.Summary.OverTotal + r.Diff)
		}
		if r.Adjusted {
			report.Summary.AdjustedCount++
			report.Summary.AdjustedTotal = round2(report.Summary.AdjustedTotal + r.AdjustmentAmount)
		}
		report.Summary.TotalShifts++
		report.Shifts = append(report.Shifts, r)
	}

	return report, nil
}

// ApplyShiftAdjustment menyamakan pendapatan cloud dengan versi kasir untuk satu
// shift: menambahkan satu transaksi penyesuaian (payment_method='adjustment')
// sebesar selisih, dan mencatatnya di shift_revenue_adjustments agar bisa dibatalkan.
func ApplyShiftAdjustment(shiftID, adminName string, outletScope []string) error {
	shiftID = strings.TrimSpace(shiftID)

	var outletID, outletCode, reportJSON, status string
	var buka, tutup time.Time
	err := database.DB.QueryRow(`
		SELECT TRIM(s.outlet_id), COALESCE(o.code,''), COALESCE(s.report::text,'{}'), s.status, s.created_at, s.updated_at
		FROM cloud_cashier_shifts s LEFT JOIN outlets o ON o.id = s.outlet_id
		WHERE TRIM(s.id) = TRIM($1)`, shiftID).
		Scan(&outletID, &outletCode, &reportJSON, &status, &buka, &tutup)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("shift tidak ditemukan")
		}
		return err
	}
	if status != "closed" {
		return fmt.Errorf("hanya shift tertutup yang bisa disesuaikan")
	}
	if outletScope != nil {
		allowed := false
		for _, id := range outletScope {
			if id == outletID {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("akses outlet tidak diizinkan")
		}
	}

	var rj reconShiftJSON
	json.Unmarshal([]byte(reportJSON), &rj)
	cloudSales, _ := cloudSalesInWindow(outletID, buka, tutup)
	diff := round2(rj.SalesTotal - cloudSales)
	if diff <= 0 {
		return fmt.Errorf("tidak ada kekurangan untuk disesuaikan (cloud sudah ≥ versi kasir)")
	}

	// Cegah dobel penyesuaian aktif.
	var exists bool
	database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM shift_revenue_adjustments WHERE TRIM(shift_id)=TRIM($1) AND status='applied')`, shiftID).Scan(&exists)
	if exists {
		return fmt.Errorf("shift ini sudah punya penyesuaian aktif")
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}

	// Transaksi penyesuaian: dicatat pada hari (buka) shift agar dashboard-per-hari
	// mengikuti versi kasir. Tanpa order & tanpa transaction_payments, sehingga tak
	// mengotori kartu Metode Pembayaran.
	txnID := NewULID()
	localID := "ADJ-" + shiftID
	if _, err := tx.Exec(`
		INSERT INTO cloud_transactions (id, local_id, outlet_id, outlet_code, order_id,
			subtotal, total_amount, tax_amount, payment_method, cash_amount, change_amount,
			cashier_name, items, created_at, synced_at)
		VALUES ($1,$2,$3,$4,'',$5,$5,0,'adjustment',0,0,$6,'[]',$7, NOW())
		ON CONFLICT (outlet_id, local_id) DO UPDATE SET
			total_amount = EXCLUDED.total_amount, subtotal = EXCLUDED.subtotal,
			created_at = EXCLUDED.created_at, synced_at = NOW()`,
		txnID, localID, outletID, outletCode, diff, "Penyesuaian (versi kasir)", buka,
	); err != nil {
		tx.Rollback()
		return err
	}

	if _, err := tx.Exec(`
		INSERT INTO shift_revenue_adjustments
			(outlet_id, shift_id, shift_date, cashier_sales, cloud_sales, adjustment_amount,
			 txn_id, txn_local_id, note, status, created_by)
		VALUES ($1,$2, tz_date($3), $4,$5,$6, $7,$8,$9,'applied',$10)`,
		outletID, shiftID, buka, round2(rj.SalesTotal), cloudSales, diff,
		txnID, localID,
		fmt.Sprintf("Ikuti versi kasir: +%s (kasir %s vs cloud %s)", fmtRp(diff), fmtRp(rj.SalesTotal), fmtRp(cloudSales)),
		adminName,
	); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// RevertShiftAdjustment membatalkan penyesuaian: menghapus transaksi penyesuaian
// dan menandai catatan sebagai reverted (nilai kembali ke data asli cloud).
func RevertShiftAdjustment(adjID int64, adminName string, outletScope []string) error {
	var outletID, txnID, status string
	err := database.DB.QueryRow(`SELECT TRIM(outlet_id), COALESCE(TRIM(txn_id),''), status FROM shift_revenue_adjustments WHERE id=$1`, adjID).
		Scan(&outletID, &txnID, &status)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("penyesuaian tidak ditemukan")
		}
		return err
	}
	if status != "applied" {
		return fmt.Errorf("penyesuaian sudah dibatalkan")
	}
	if outletScope != nil {
		allowed := false
		for _, id := range outletScope {
			if id == outletID {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("akses outlet tidak diizinkan")
		}
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	if txnID != "" {
		if _, err := tx.Exec(`DELETE FROM cloud_transactions WHERE TRIM(id)=TRIM($1)`, txnID); err != nil {
			tx.Rollback()
			return err
		}
	}
	if _, err := tx.Exec(`UPDATE shift_revenue_adjustments SET status='reverted', reverted_by=$2, reverted_at=now() AT TIME ZONE 'UTC' WHERE id=$1`, adjID, adminName); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// fmtRp — format rupiah ringkas untuk catatan.
func fmtRp(v float64) string {
	s := fmt.Sprintf("%.0f", v)
	// sisipkan pemisah ribuan
	n := len(s)
	if n <= 3 {
		return "Rp" + s
	}
	var b strings.Builder
	b.WriteString("Rp")
	pre := n % 3
	if pre > 0 {
		b.WriteString(s[:pre])
		if n > pre {
			b.WriteString(".")
		}
	}
	for i := pre; i < n; i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < n {
			b.WriteString(".")
		}
	}
	return b.String()
}
