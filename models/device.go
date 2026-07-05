package models

// ── Heartbeat perangkat (App POS → Cloud) ──────────────────────────────────
// Telemetri kondisi tablet kasir: baterai, penyimpanan, printer, jaringan.
// App mengirim snapshot tiap akhir siklus sync; cloud menyimpan snapshot
// terakhir per outlet (upsert) + histori ringkas untuk tren.

// DevicePrinter — status satu printer yang terdaftar di perangkat.
type DevicePrinter struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	IP        string `json:"ip"`
	Type      string `json:"type"`  // lan | bluetooth
	Roles     string `json:"roles"` // peran cetak (Dapur, Kasir, ...)
	Connected bool   `json:"connected"`
	Online    bool   `json:"online"`
}

// DeviceHeartbeatRequest — payload heartbeat dari App POS.
type DeviceHeartbeatRequest struct {
	Device struct {
		AppVersion     string `json:"app_version"`
		Battery        int    `json:"battery"`
		BatteryState   string `json:"battery_state"`
		Model          string `json:"model"`
		OS             string `json:"os"`
		StorageTotalMB int64  `json:"storage_total_mb"`
		StorageFreeMB  int64  `json:"storage_free_mb"`

		// RAM (Android; nullable karena tidak dikirim di iOS/desktop)
		RamTotalMB     *int  `json:"ram_total_mb"`
		RamFreeMB      *int  `json:"ram_free_mb"`
		RamUsedPercent *int  `json:"ram_used_percent"`
		RamLow         *bool `json:"ram_low"`

		// CPU (Android; cpu_cores selalu ada di Android, sisanya best-effort)
		CPUCores       *int     `json:"cpu_cores"`
		CPUUsedPercent *float64 `json:"cpu_used_percent"`
		CPULoad1m      *float64 `json:"cpu_load_1m"`
		CPULoad5m      *float64 `json:"cpu_load_5m"`
		CPULoad15m     *float64 `json:"cpu_load_15m"`
	} `json:"device"`
	Printers []DevicePrinter `json:"printers"`
	Network  struct {
		Online      bool   `json:"online"`
		PendingSync int    `json:"pending_sync"`
		LastSyncAt  string `json:"last_sync_at"`
	} `json:"network"`
	ReportedAt string `json:"reported_at"`
}

// DeviceStatus — kondisi perangkat satu outlet untuk halaman monitoring.
type DeviceStatus struct {
	OutletID   string `json:"outlet_id"`
	OutletName string `json:"outlet_name"`
	OutletCode string `json:"outlet_code"`

	HasData      bool   `json:"has_data"`      // pernah mengirim heartbeat
	Status       string `json:"status"`        // online | idle | offline | no_data
	StaleMinutes int    `json:"stale_minutes"` // menit sejak heartbeat terakhir

	AppVersion     string  `json:"app_version"`
	Battery        int     `json:"battery"`
	BatteryState   string  `json:"battery_state"`
	Model          string  `json:"model"`
	OS             string  `json:"os"`
	StorageTotalMB int64   `json:"storage_total_mb"`
	StorageFreeMB  int64   `json:"storage_free_mb"`
	StorageUsedPct float64 `json:"storage_used_pct"`

	// RAM & CPU — nullable (null = perangkat tidak melaporkan; mis. iOS atau CPU diblok)
	RamTotalMB     *int     `json:"ram_total_mb"`
	RamFreeMB      *int     `json:"ram_free_mb"`
	RamUsedPercent *int     `json:"ram_used_percent"`
	RamLow         *bool    `json:"ram_low"`
	CPUCores       *int     `json:"cpu_cores"`
	CPUUsedPercent *float64 `json:"cpu_used_percent"`
	CPULoad1m      *float64 `json:"cpu_load_1m"`
	CPULoad5m      *float64 `json:"cpu_load_5m"`
	CPULoad15m     *float64 `json:"cpu_load_15m"`

	NetworkOnline bool   `json:"network_online"`
	PendingSync   int    `json:"pending_sync"`
	LastSyncAt    string `json:"last_sync_at"` // lokal "YYYY-MM-DD HH:MM"
	ReportedAt    string `json:"reported_at"`  // lokal "YYYY-MM-DD HH:MM"
	ReceivedAt    string `json:"received_at"`  // lokal "YYYY-MM-DD HH:MM"

	Printers       []DevicePrinter `json:"printers"`
	PrintersTotal  int             `json:"printers_total"`
	PrintersOnline int             `json:"printers_online"`
}

// DeviceMonitorSummary — ringkasan armada perangkat.
type DeviceMonitorSummary struct {
	TotalOutlets    int `json:"total_outlets"`
	OnlineCount     int `json:"online_count"`
	IdleCount       int `json:"idle_count"`
	OfflineCount    int `json:"offline_count"`
	NoDataCount     int `json:"no_data_count"`
	LowBattery      int `json:"low_battery"`        // baterai < 20% & tidak charging
	PrinterIssues   int `json:"printer_issues"`     // ada printer terputus
	ResourceIssues  int `json:"resource_issues"`    // RAM/CPU perangkat kritis
	PendingSyncTotal int `json:"pending_sync_total"` // total antrian sync semua outlet
}

type DeviceMonitorReport struct {
	Summary DeviceMonitorSummary `json:"summary"`
	Devices []DeviceStatus       `json:"devices"`
}

// DeviceHeartbeatPoint — satu titik histori untuk tren baterai/jaringan.
type DeviceHeartbeatPoint struct {
	Battery        int    `json:"battery"`
	BatteryState   string `json:"battery_state"`
	NetworkOnline  bool   `json:"network_online"`
	PendingSync    int    `json:"pending_sync"`
	PrintersOnline int    `json:"printers_online"`
	PrintersTotal  int    `json:"printers_total"`
	ReportedAt     string `json:"reported_at"` // lokal "MM-DD HH:MM"
}
