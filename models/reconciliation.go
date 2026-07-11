package models

// Rekonsiliasi shift: bandingkan penjualan versi kasir (report shift, dari tablet)
// dengan penjualan yang benar-benar masuk ke cloud (cloud_transactions).

type ShiftReconRow struct {
	ShiftID    string `json:"shift_id"`
	OutletID   string `json:"outlet_id"`
	OutletName string `json:"outlet_name"`
	OpenedAt   string `json:"opened_at"` // lokal "YYYY-MM-DD HH:MM"
	ClosedAt   string `json:"closed_at"`
	Cashiers   string `json:"cashiers"` // nama kasir asli dari by_cashier

	CashierSales float64 `json:"cashier_sales"` // sales_total di report shift
	CashierCount int     `json:"cashier_count"`
	CloudSales   float64 `json:"cloud_sales"` // total_amount transaksi cloud (excl komplimen & adjustment)
	CloudCount   int     `json:"cloud_count"`
	Diff         float64 `json:"diff"`   // cashier - cloud (positif = cloud kurang)
	Status       string  `json:"status"` // balanced | short | over

	Adjusted         bool    `json:"adjusted"` // sudah ada penyesuaian aktif
	AdjustmentID     int64   `json:"adjustment_id"`
	AdjustmentAmount float64 `json:"adjustment_amount"`
}

type ShiftReconSummary struct {
	TotalShifts   int     `json:"total_shifts"`
	BalancedCount int     `json:"balanced_count"`
	ShortCount    int     `json:"short_count"` // cloud < kasir (ada yang belum masuk)
	ShortTotal    float64 `json:"short_total"`
	OverCount     int     `json:"over_count"` // cloud > kasir
	OverTotal     float64 `json:"over_total"`
	AdjustedCount int     `json:"adjusted_count"`
	AdjustedTotal float64 `json:"adjusted_total"`
}

type ShiftReconReport struct {
	Summary ShiftReconSummary `json:"summary"`
	Shifts  []ShiftReconRow   `json:"shifts"`
}
