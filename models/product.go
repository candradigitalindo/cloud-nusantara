package models

import "time"

type PushProductRequest struct {
	LocalID      string  `json:"local_id"`
	OutletID     string  `json:"outlet_id"`
	Name         string  `json:"name"`
	Code         string  `json:"code"`
	Description  string  `json:"description"`
	CategoryID   string  `json:"category_id"`
	CategoryName string  `json:"category_name"`
	Price        float64 `json:"price"`
	Destination  string  `json:"destination"`
	Version      int     `json:"version"`
	UpdatedAt    string  `json:"updated_at"`
}

// PushProductAddonRequest — add-on/modifier menu yang dikirim POS lewat sync
// batch. ProductLocalID menunjuk cloud_products.local_id (POS yang memegang
// kebenaran id-nya), bukan id cloud.
type PushProductAddonRequest struct {
	LocalID        string  `json:"local_id"`
	ProductLocalID string  `json:"product_local_id"`
	GroupName      string  `json:"group_name"`
	Name           string  `json:"name"`
	Price          float64 `json:"price"`
	SortOrder      int     `json:"sort_order"`
	IsActive       int     `json:"is_active"`
	Version        int     `json:"version"`
	UpdatedAt      string  `json:"updated_at"`
}

// CloudProductAddon — bentuk add-on yang dikembalikan API (menu publik & UI).
type CloudProductAddon struct {
	ID             string  `json:"id"`
	LocalID        string  `json:"local_id"`
	ProductLocalID string  `json:"product_local_id"`
	GroupName      string  `json:"group_name"`
	Name           string  `json:"name"`
	Price          float64 `json:"price"`
	SortOrder      int     `json:"sort_order"`
	IsActive       bool    `json:"is_active"`
}

type CloudProduct struct {
	ID           string    `json:"id"`
	LocalID      string    `json:"local_id"`
	OutletID     string    `json:"outlet_id"`
	OutletName   string    `json:"outlet_name,omitempty"`
	Name         string    `json:"name"`
	Code         string    `json:"code"`
	Description  string    `json:"description"`
	CategoryID   string    `json:"category_id"`
	CategoryName string    `json:"category_name"`
	Price        float64   `json:"price"`
	Destination  string    `json:"destination"`
	PhotoURL     string    `json:"photo_url"`
	IsDeleted    bool      `json:"is_deleted"`
	Version      int       `json:"version"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	SyncedAt     time.Time `json:"synced_at"`
}

type PushCategoryRequest struct {
	LocalID     string `json:"local_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CodePrefix  string `json:"code_prefix"`
	Version     int    `json:"version"`
}

type CloudCategory struct {
	ID         string    `json:"id"`
	LocalID    string    `json:"local_id"`
	OutletID   string    `json:"outlet_id"`
	OutletName string    `json:"outlet_name,omitempty"`
	Name       string    `json:"name"`
	CodePrefix string    `json:"code_prefix"`
	PrinterID  string    `json:"printer_id"`
	IsDeleted  bool      `json:"is_deleted"`
	Version    int       `json:"version"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	SyncedAt   time.Time `json:"synced_at"`
}

type AdminCreateProductRequest struct {
	OutletID     string  `json:"outlet_id"`
	Name         string  `json:"name"`
	Code         string  `json:"code"`
	Description  string  `json:"description"`
	CategoryID   string  `json:"category_id"`
	CategoryName string  `json:"category_name"`
	Price        float64 `json:"price"`
	Destination  string  `json:"destination"`
}

type AdminUpdateProductRequest struct {
	Name         string  `json:"name"`
	Code         string  `json:"code"`
	Description  string  `json:"description"`
	CategoryID   string  `json:"category_id"`
	CategoryName string  `json:"category_name"`
	Price        float64 `json:"price"`
	Destination  string  `json:"destination"`
}

type AdminCreateCategoryRequest struct {
	OutletID   string `json:"outlet_id"`
	Name       string `json:"name"`
	CodePrefix string `json:"code_prefix"`
}

type AdminUpdateCategoryRequest struct {
	Name       string `json:"name"`
	CodePrefix string `json:"code_prefix"`
}

// QRISChargeResponse — bentuk tagihan QRIS yang dikirim ke POS. QRString adalah
// payload QRIS mentah; POS yang menggambar kodenya sendiri agar tidak
// bergantung pada URL gambar milik penyedia.
type QRISChargeResponse struct {
	ChargeID  string  `json:"charge_id"`
	Provider  string  `json:"provider"`
	QRString  string  `json:"qr_string"`
	Amount    float64 `json:"amount"`
	Status    string  `json:"status"`
	ExpiresAt string  `json:"expires_at"`
	PaidAt    string  `json:"paid_at,omitempty"`
}

// PushAdditionalChargeRequest — biaya tambahan (pajak/PB1, service charge)
// yang dikirim POS. POS adalah sumber kebenarannya; cloud hanya mencerminkan.
type PushAdditionalChargeRequest struct {
	LocalID    string  `json:"local_id"`
	Name       string  `json:"name"`
	ChargeType string  `json:"charge_type"` // percentage | fixed
	Value      float64 `json:"value"`
	IsActive   int     `json:"is_active"`
	Version    int     `json:"version"`
}

type CloudAdditionalCharge struct {
	LocalID    string  `json:"local_id"`
	Name       string  `json:"name"`
	ChargeType string  `json:"charge_type"`
	Value      float64 `json:"value"`
}

// ChargeLine — satu baris rincian biaya pada ringkasan tagihan tamu.
type ChargeLine struct {
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
}
