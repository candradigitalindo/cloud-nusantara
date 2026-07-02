package services

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/lib/pq"
)

type shiftReportJSON struct {
	SalesTotal   float64                            `json:"sales_total"`
	SalesCount   int                                `json:"sales_count"`
	OpeningCash  float64                            `json:"opening_cash"`
	CashInTotal  float64                            `json:"cash_in_total"`
	CashOutTotal float64                            `json:"cash_out_total"`
	ExpectedCash float64                            `json:"expected_cash"`
	ByMethod     map[string]models.ShiftMethodTotal `json:"by_method"`
}

// GetCashierShiftReport mengembalikan daftar shift kasir (buka/serah-terima/tutup)
// dengan analisa balance kas (selisih = kas akhir − kas seharusnya) dan ringkasannya.
func GetCashierShiftReport(outletID, status, dateFrom, dateTo string, outletScope []string) (*models.CashierShiftReport, error) {
	conds := []string{"1=1"}
	args := []interface{}{}
	idx := 1
	if outletID != "" {
		conds = append(conds, fmt.Sprintf("s.outlet_id = $%d", idx))
		args = append(args, outletID)
		idx++
	}
	if status != "" {
		conds = append(conds, fmt.Sprintf("s.status = $%d", idx))
		args = append(args, status)
		idx++
	}
	if dateFrom != "" {
		conds = append(conds, fmt.Sprintf("s.created_at >= tz_day_start($%d::date)", idx))
		args = append(args, dateFrom)
		idx++
	}
	if dateTo != "" {
		conds = append(conds, fmt.Sprintf("s.created_at < tz_day_start($%d::date + 1)", idx))
		args = append(args, dateTo)
		idx++
	}
	if outletScope != nil {
		conds = append(conds, fmt.Sprintf("s.outlet_id = ANY($%d::text[])", idx))
		args = append(args, pq.Array(outletScope))
		idx++
	}

	q := fmt.Sprintf(`
		SELECT s.id, s.outlet_id, COALESCE(o.name, s.outlet_id), s.opened_by,
			-- Pakai created_at (waktu cloud, UTC andal) untuk waktu buka; opened_at dari app
			-- naive-local (geser +7) & bisa ter-overwrite jadi closed_at saat tutup.
			to_char((s.created_at AT TIME ZONE 'UTC') AT TIME ZONE COALESCE((SELECT value FROM app_settings WHERE key='timezone'),'Asia/Jakarta'), 'YYYY-MM-DD HH24:MI'),
			COALESCE(s.closed_by, ''),
			CASE WHEN s.status='closed' THEN to_char((s.updated_at AT TIME ZONE 'UTC') AT TIME ZONE COALESCE((SELECT value FROM app_settings WHERE key='timezone'),'Asia/Jakarta'), 'YYYY-MM-DD HH24:MI') ELSE '' END,
			s.opening_cash, COALESCE(s.closing_cash,0), COALESCE(s.closing_card,0),
			COALESCE(s.closing_qris,0), COALESCE(s.closing_transfer,0), COALESCE(s.carry_over_cash,0),
			COALESCE(s.handover_to,''), s.status, COALESCE(s.notes,''),
			COALESCE(s.report::text, '{}'), s.created_at
		FROM cloud_cashier_shifts s
		LEFT JOIN outlets o ON o.id = s.outlet_id
		WHERE %s
		ORDER BY s.created_at DESC`, strings.Join(conds, " AND "))

	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	report := &models.CashierShiftReport{Shifts: []models.CashierShift{}}
	shiftIDs := []string{}
	for rows.Next() {
		var sh models.CashierShift
		var reportJSON string
		if err := rows.Scan(&sh.ID, &sh.OutletID, &sh.OutletName, &sh.OpenedBy,
			&sh.OpenedAt, &sh.ClosedBy, &sh.ClosedAt,
			&sh.OpeningCash, &sh.ClosingCash, &sh.ClosingCard, &sh.ClosingQris, &sh.ClosingTransfer,
			&sh.CarryOverCash, &sh.HandoverTo, &sh.Status, &sh.Notes, &reportJSON, &sh.CreatedAt); err != nil {
			return nil, err
		}
		var rj shiftReportJSON
		json.Unmarshal([]byte(reportJSON), &rj)
		sh.ByMethod = rj.ByMethod
		if sh.ByMethod == nil {
			sh.ByMethod = map[string]models.ShiftMethodTotal{}
		}
		sh.SalesTotal = rj.SalesTotal
		sh.SalesCount = rj.SalesCount
		sh.CashSales = sh.ByMethod["cash"].Total
		sh.Movements = []models.CashMovement{}
		report.Shifts = append(report.Shifts, sh)
		shiftIDs = append(shiftIDs, strings.TrimSpace(sh.ID))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Kas masuk (pendapatan tambahan) & kas keluar (pengeluaran) dari rincian movement.
	movByShift := map[string][]models.CashMovement{}
	if len(shiftIDs) > 0 {
		mrows, mErr := database.DB.Query(`
			SELECT TRIM(shift_id), movement_type, amount, COALESCE(counterpart_name,''), COALESCE(note,''),
				COALESCE(to_char((synced_at AT TIME ZONE 'UTC') AT TIME ZONE COALESCE((SELECT value FROM app_settings WHERE key='timezone'),'Asia/Jakarta'),'YYYY-MM-DD HH24:MI'),'')
			FROM cloud_cash_movements WHERE TRIM(shift_id) = ANY($1::text[])
			ORDER BY synced_at ASC`, pq.Array(shiftIDs))
		if mErr == nil {
			defer mrows.Close()
			for mrows.Next() {
				var sid string
				var m models.CashMovement
				if mrows.Scan(&sid, &m.Type, &m.Amount, &m.CounterpartName, &m.Note, &m.CreatedAt) == nil {
					movByShift[sid] = append(movByShift[sid], m)
				}
			}
		}
	}

	// Finalisasi: kas masuk/keluar dari movement, kas seharusnya, selisih, ringkasan.
	for i := range report.Shifts {
		sh := &report.Shifts[i]
		movs := movByShift[strings.TrimSpace(sh.ID)]
		sh.Movements = movs
		for _, m := range movs {
			if m.Type == "in" {
				sh.CashIn += m.Amount
			} else if m.Type == "out" {
				sh.CashOut += m.Amount
			}
		}
		// Kas seharusnya = kas awal + penjualan tunai + kas masuk − kas keluar.
		sh.ExpectedCash = round2(sh.OpeningCash + sh.CashSales + sh.CashIn - sh.CashOut)
		if sh.Status == "closed" {
			sh.Variance = round2(sh.ClosingCash - sh.ExpectedCash)
			sh.Balanced = math.Abs(sh.Variance) < 0.5
		}

		report.Summary.TotalShifts++
		report.Summary.TotalSales += sh.SalesTotal
		report.Summary.TotalCashIn += sh.CashIn
		report.Summary.TotalCashOut += sh.CashOut
		if sh.Status == "closed" {
			report.Summary.ClosedShifts++
			report.Summary.TotalVariance = round2(report.Summary.TotalVariance + sh.Variance)
			if sh.Balanced {
				report.Summary.BalancedCount++
			} else {
				report.Summary.MissCount++
				if sh.Variance < 0 {
					report.Summary.ShortageTotal = round2(report.Summary.ShortageTotal + sh.Variance)
				} else {
					report.Summary.OverageTotal = round2(report.Summary.OverageTotal + sh.Variance)
				}
			}
		} else {
			report.Summary.OpenShifts++
		}
	}
	return report, nil
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
