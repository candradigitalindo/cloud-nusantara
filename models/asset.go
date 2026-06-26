package models

import "time"

// Asset — barang inventaris outlet (meja, kursi, elektronik, dll).
type Asset struct {
	ID               string    `json:"id"`
	OutletID         string    `json:"outlet_id"`
	OutletName       string    `json:"outlet_name"`
	Code             string    `json:"code"`
	Name             string    `json:"name"`
	Category         string    `json:"category"`
	Quantity         int       `json:"quantity"`
	Unit             string    `json:"unit"`
	Condition        string    `json:"condition"` // baik | rusak_ringan | rusak_berat | perbaikan
	Location         string    `json:"location"`
	PurchaseDate     string    `json:"purchase_date"` // YYYY-MM-DD ('' bila kosong)
	PurchasePrice    float64   `json:"purchase_price"`
	Notes            string    `json:"notes"`
	MaintenanceCount int       `json:"maintenance_count"`
	LastMaintenance  string    `json:"last_maintenance"` // YYYY-MM-DD ('' bila belum ada)
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// AssetMaintenance — satu catatan perawatan/perbaikan sebuah aset.
type AssetMaintenance struct {
	ID              string    `json:"id"`
	AssetID         string    `json:"asset_id"`
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
	OutletID      string  `json:"outlet_id"`
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Category      string  `json:"category"`
	Quantity      int     `json:"quantity"`
	Unit          string  `json:"unit"`
	Condition     string  `json:"condition"`
	Location      string  `json:"location"`
	PurchaseDate  string  `json:"purchase_date"`
	PurchasePrice float64 `json:"purchase_price"`
	Notes         string  `json:"notes"`
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
