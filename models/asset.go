package models

import "time"

// Asset — barang inventaris outlet (meja, kursi, elektronik, dll).
type Asset struct {
	ID               string  `json:"id"`
	OutletID         string  `json:"outlet_id"`
	OutletName       string  `json:"outlet_name"`
	Code             string  `json:"code"`
	Name             string  `json:"name"`
	Category         string  `json:"category"`
	Quantity         int     `json:"quantity"`
	Unit             string  `json:"unit"`
	Condition        string  `json:"condition"` // baik | rusak_ringan | rusak_berat | perbaikan
	Location         string  `json:"location"`
	PurchaseDate     string  `json:"purchase_date"` // YYYY-MM-DD ('' bila kosong)
	PurchasePrice    float64 `json:"purchase_price"`
	Notes            string  `json:"notes"`
	MaintenanceCount int     `json:"maintenance_count"`
	LastMaintenance  string  `json:"last_maintenance"` // YYYY-MM-DD ('' bila belum ada)
	// Jadwal perawatan berikutnya — diambil dari catatan perawatan TERBARU.
	NextDueDate string `json:"next_due_date"` // YYYY-MM-DD ('' bila belum dijadwalkan)
	DueStatus   string `json:"due_status"`    // overdue | due_soon | scheduled | none
	DueInDays   int    `json:"due_in_days"`   // negatif = terlambat; 0 bila belum dijadwalkan

	// Identitas & parameter penyusutan.
	SerialNumber     string  `json:"serial_number"`
	VendorID         string  `json:"vendor_id"`
	VendorName       string  `json:"vendor_name"`
	UsefulLifeMonths int     `json:"useful_life_months"` // 0 = tidak disusutkan
	ResidualValue    float64 `json:"residual_value"`
	Status           string  `json:"status"` // aktif | dihapus

	// Nilai — dihitung dari histori perolehan + penyusutan garis lurus.
	AcquisitionCost   float64 `json:"acquisition_cost"`   // Σ total_cost perolehan
	AccumulatedDeprec float64 `json:"accumulated_deprec"` // akumulasi penyusutan
	BookValue         float64 `json:"book_value"`         // perolehan − akumulasi
	MonthlyDeprec     float64 `json:"monthly_deprec"`
	AgeMonths         int     `json:"age_months"`       // umur sejak perolehan pertama
	RemainingMonths   int     `json:"remaining_months"` // sisa umur ekonomis

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AssetAcquisition — satu peristiwa perolehan (pembelian/hibah/dll) sebuah aset.
type AssetAcquisition struct {
	ID              string    `json:"id"`
	AssetID         string    `json:"asset_id"`
	AssetName       string    `json:"asset_name"`
	AssetCode       string    `json:"asset_code"`
	OutletID        string    `json:"outlet_id"`
	OutletName      string    `json:"outlet_name"`
	AcquisitionDate string    `json:"acquisition_date"` // YYYY-MM-DD
	Source          string    `json:"source"`           // pembelian | hibah | sewa | produksi_sendiri
	Quantity        int       `json:"quantity"`
	UnitPrice       float64   `json:"unit_price"`
	TotalCost       float64   `json:"total_cost"`
	VendorID        string    `json:"vendor_id"`
	VendorName      string    `json:"vendor_name"`
	DocumentNo      string    `json:"document_no"`
	Notes           string    `json:"notes"`
	CreatedAt       time.Time `json:"created_at"`
}

type AssetAcquisitionRequest struct {
	AssetID         string  `json:"asset_id"`
	AcquisitionDate string  `json:"acquisition_date"`
	Source          string  `json:"source"`
	Quantity        int     `json:"quantity"`
	UnitPrice       float64 `json:"unit_price"`
	VendorID        string  `json:"vendor_id"`
	VendorName      string  `json:"vendor_name"`
	DocumentNo      string  `json:"document_no"`
	Notes           string  `json:"notes"`
	// AddToQuantity menambah jumlah unit di baris aset. Dimatikan saat mencatat
	// perolehan lama yang unitnya sudah termasuk di jumlah aset sekarang.
	AddToQuantity bool `json:"add_to_quantity"`
}

// AssetTransfer — mutasi aset antar outlet.
type AssetTransfer struct {
	ID             string    `json:"id"`
	AssetID        string    `json:"asset_id"`
	AssetName      string    `json:"asset_name"`
	AssetCode      string    `json:"asset_code"`
	FromOutletID   string    `json:"from_outlet_id"`
	FromOutletName string    `json:"from_outlet_name"`
	ToOutletID     string    `json:"to_outlet_id"`
	ToOutletName   string    `json:"to_outlet_name"`
	TransferDate   string    `json:"transfer_date"` // YYYY-MM-DD
	Quantity       int       `json:"quantity"`
	Reason         string    `json:"reason"`
	PerformedBy    string    `json:"performed_by"`
	Notes          string    `json:"notes"`
	CreatedAt      time.Time `json:"created_at"`
}

type AssetTransferRequest struct {
	AssetID      string `json:"asset_id"`
	ToOutletID   string `json:"to_outlet_id"`
	TransferDate string `json:"transfer_date"`
	Reason       string `json:"reason"`
	PerformedBy  string `json:"performed_by"`
	Notes        string `json:"notes"`
}

// AssetDisposal — penghapusan aset (dijual/dimusnahkan/hibah/hilang).
type AssetDisposal struct {
	ID                  string    `json:"id"`
	AssetID             string    `json:"asset_id"`
	AssetName           string    `json:"asset_name"`
	AssetCode           string    `json:"asset_code"`
	OutletID            string    `json:"outlet_id"`
	OutletName          string    `json:"outlet_name"`
	DisposalDate        string    `json:"disposal_date"` // YYYY-MM-DD
	Method              string    `json:"method"`        // dijual | dimusnahkan | hibah | hilang
	Quantity            int       `json:"quantity"`
	Proceeds            float64   `json:"proceeds"`
	BookValueAtDisposal float64   `json:"book_value_at_disposal"`
	GainLoss            float64   `json:"gain_loss"` // hasil − nilai buku
	Reason              string    `json:"reason"`
	ApprovedBy          string    `json:"approved_by"`
	Notes               string    `json:"notes"`
	CreatedAt           time.Time `json:"created_at"`
}

type AssetDisposalRequest struct {
	AssetID      string  `json:"asset_id"`
	DisposalDate string  `json:"disposal_date"`
	Method       string  `json:"method"`
	Proceeds     float64 `json:"proceeds"`
	Reason       string  `json:"reason"`
	ApprovedBy   string  `json:"approved_by"`
	Notes        string  `json:"notes"`
}

// AssetSummary — ringkasan perlengkapan untuk kartu KPI di halaman Perlengkapan.
type AssetSummary struct {
	TotalAssets    int     `json:"total_assets"`
	TotalQuantity  int     `json:"total_quantity"`
	TotalValue     float64 `json:"total_value"` // Σ harga beli × jumlah
	Overdue        int     `json:"overdue"`
	DueSoon        int     `json:"due_soon"`
	Scheduled      int     `json:"scheduled"`
	Unscheduled    int     `json:"unscheduled"`
	NeedsAttention int     `json:"needs_attention"` // kondisi selain 'baik'
	DueSoonDays    int     `json:"due_soon_days"`   // ambang "segera" (hari)
}

// ── Dashboard Aset ──────────────────────────────────────────

// AssetDashboard — ringkasan modul aset untuk halaman dashboard.
type AssetDashboard struct {
	// Nilai
	TotalAssets       int     `json:"total_assets"`
	TotalQuantity     int     `json:"total_quantity"`
	AcquisitionCost   float64 `json:"acquisition_cost"`
	AccumulatedDeprec float64 `json:"accumulated_deprec"`
	BookValue         float64 `json:"book_value"`
	MonthlyDeprec     float64 `json:"monthly_deprec"`

	// Perawatan
	Overdue         int     `json:"overdue"`
	DueSoon         int     `json:"due_soon"`
	DueSoonDays     int     `json:"due_soon_days"`
	Unscheduled     int     `json:"unscheduled"`
	MaintCostMTD    float64 `json:"maint_cost_mtd"`
	MaintCost12M    float64 `json:"maint_cost_12m"`
	NeedsAttention  int     `json:"needs_attention"`
	EndingSoonCount int     `json:"ending_soon_count"` // sisa umur ekonomis ≤3 bulan

	// Penghapusan tahun berjalan
	DisposedYTD int     `json:"disposed_ytd"`
	ProceedsYTD float64 `json:"proceeds_ytd"`
	GainLossYTD float64 `json:"gain_loss_ytd"`

	ConditionBreakdown []AssetBucket  `json:"condition_breakdown"`
	ByOutlet           []AssetBucket  `json:"by_outlet"`
	ByCategory         []AssetBucket  `json:"by_category"`
	AcquisitionTrend   []AssetMonthly `json:"acquisition_trend"` // 12 bulan
	MaintenanceTrend   []AssetMonthly `json:"maintenance_trend"` // 12 bulan
	UpcomingMaint      []Asset        `json:"upcoming_maint"`    // paling mendesak
}

// AssetBucket — satu irisan (kondisi/outlet/kategori) beserta nilainya.
type AssetBucket struct {
	Key      string  `json:"key"`
	Label    string  `json:"label"`
	Count    int     `json:"count"`
	Quantity int     `json:"quantity"`
	Value    float64 `json:"value"`
}

// AssetMonthly — satu titik pada grafik 12 bulan.
type AssetMonthly struct {
	Month string  `json:"month"` // YYYY-MM
	Count int     `json:"count"`
	Value float64 `json:"value"`
}

// AssetMaintenance — satu catatan perawatan/perbaikan sebuah aset.
type AssetMaintenance struct {
	ID      string `json:"id"`
	AssetID string `json:"asset_id"`
	// Terisi hanya pada daftar lintas aset (halaman Perawatan).
	AssetName       string    `json:"asset_name,omitempty"`
	AssetCode       string    `json:"asset_code,omitempty"`
	OutletName      string    `json:"outlet_name,omitempty"`
	MaintenanceDate string    `json:"maintenance_date"` // YYYY-MM-DD
	Type            string    `json:"type"`             // rutin | perbaikan | penggantian | inspeksi
	Description     string    `json:"description"`
	Cost            float64   `json:"cost"`
	PerformedBy     string    `json:"performed_by"`
	ConditionAfter  string    `json:"condition_after"`
	NextDueDate     string    `json:"next_due_date"` // YYYY-MM-DD ('' bila kosong)
	CreatedAt       time.Time `json:"created_at"`
}

type AssetRequest struct {
	OutletID         string  `json:"outlet_id"`
	Code             string  `json:"code"`
	Name             string  `json:"name"`
	Category         string  `json:"category"`
	Quantity         int     `json:"quantity"`
	Unit             string  `json:"unit"`
	Condition        string  `json:"condition"`
	Location         string  `json:"location"`
	PurchaseDate     string  `json:"purchase_date"`
	PurchasePrice    float64 `json:"purchase_price"`
	Notes            string  `json:"notes"`
	SerialNumber     string  `json:"serial_number"`
	VendorID         string  `json:"vendor_id"`
	UsefulLifeMonths int     `json:"useful_life_months"`
	ResidualValue    float64 `json:"residual_value"`
}

type AssetMaintenanceRequest struct {
	MaintenanceDate string  `json:"maintenance_date"`
	Type            string  `json:"type"`
	Description     string  `json:"description"`
	Cost            float64 `json:"cost"`
	PerformedBy     string  `json:"performed_by"`
	ConditionAfter  string  `json:"condition_after"`
	NextDueDate     string  `json:"next_due_date"`
}
