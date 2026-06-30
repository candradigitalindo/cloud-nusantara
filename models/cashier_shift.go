package models

import "time"

type ShiftMethodTotal struct {
	Count int     `json:"count"`
	Total float64 `json:"total"`
}

// CashMovement — kas masuk (pendapatan tambahan di luar transaksi) / kas keluar (pengeluaran).
type CashMovement struct {
	Type            string  `json:"type"` // in | out
	Amount          float64 `json:"amount"`
	CounterpartName string  `json:"counterpart_name"`
	Note            string  `json:"note"`
	CreatedAt       string  `json:"created_at"`
}

// CashierShift — satu sesi kasir (buka → [serah terima] → tutup) beserta analisa balance kas.
type CashierShift struct {
	ID              string                      `json:"id"`
	OutletID        string                      `json:"outlet_id"`
	OutletName      string                      `json:"outlet_name"`
	OpenedBy        string                      `json:"opened_by"`
	OpenedAt        string                      `json:"opened_at"` // lokal "YYYY-MM-DD HH:MM"
	ClosedBy        string                      `json:"closed_by"`
	ClosedAt        string                      `json:"closed_at"`
	OpeningCash     float64                     `json:"opening_cash"`
	ClosingCash     float64                     `json:"closing_cash"`
	ClosingCard     float64                     `json:"closing_card"`
	ClosingQris     float64                     `json:"closing_qris"`
	ClosingTransfer float64                     `json:"closing_transfer"`
	CarryOverCash   float64                     `json:"carry_over_cash"`
	HandoverTo      string                      `json:"handover_to"`
	Status          string                      `json:"status"` // open | closed
	Notes           string                      `json:"notes"`
	SalesTotal      float64                     `json:"sales_total"`
	SalesCount      int                         `json:"sales_count"`
	CashSales       float64                     `json:"cash_sales"`
	CashIn          float64                     `json:"cash_in"`
	CashOut         float64                     `json:"cash_out"`
	ExpectedCash    float64                     `json:"expected_cash"` // kas seharusnya
	ByMethod        map[string]ShiftMethodTotal `json:"by_method"`
	Movements       []CashMovement              `json:"movements"` // rincian kas masuk/keluar
	Variance        float64                     `json:"variance"`  // closing_cash - expected (selisih); + lebih, - kurang
	Balanced        bool                        `json:"balanced"`  // true bila shift tertutup & selisih = 0
	CreatedAt       time.Time                   `json:"created_at"`
}

type CashierShiftSummary struct {
	TotalShifts   int     `json:"total_shifts"`
	ClosedShifts  int     `json:"closed_shifts"`
	OpenShifts    int     `json:"open_shifts"`
	BalancedCount int     `json:"balanced_count"`
	MissCount     int     `json:"miss_count"`      // shift tertutup dengan selisih != 0
	TotalVariance float64 `json:"total_variance"`  // jumlah selisih semua shift tertutup
	ShortageTotal float64 `json:"shortage_total"`  // total kekurangan (selisih negatif)
	OverageTotal  float64 `json:"overage_total"`   // total kelebihan (selisih positif)
	TotalSales    float64 `json:"total_sales"`
	TotalCashIn   float64 `json:"total_cash_in"`   // total pendapatan tambahan di luar transaksi
	TotalCashOut  float64 `json:"total_cash_out"`  // total pengeluaran
}

type CashierShiftReport struct {
	Summary CashierShiftSummary `json:"summary"`
	Shifts  []CashierShift      `json:"shifts"`
}
