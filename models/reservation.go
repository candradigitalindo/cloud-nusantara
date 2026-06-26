package models

import "time"

type ReservationItem struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	Qty         int     `json:"qty"`
	Price       float64 `json:"price"`
	Subtotal    float64 `json:"subtotal"`
}

type Reservation struct {
	ID              string            `json:"id"`
	OutletID        string            `json:"outlet_id"`
	OutletName      string            `json:"outlet_name"`
	CustomerName    string            `json:"customer_name"`
	CustomerPhone   string            `json:"customer_phone"`
	Pax             int               `json:"pax"`
	Items           []ReservationItem `json:"items"`
	Subtotal        float64           `json:"subtotal"`
	DownPayment     float64           `json:"down_payment"`
	Total           float64           `json:"total"`
	Remaining       float64           `json:"remaining"`
	ReservationDate string            `json:"reservation_date"` // YYYY-MM-DD
	ReservationTime string            `json:"reservation_time"` // HH:MM
	Status          string            `json:"status"`           // pending | confirmed | cancelled | done
	Notes           string            `json:"notes"`
	Source          string            `json:"source"` // admin | public
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

type ReservationRequest struct {
	OutletID        string            `json:"outlet_id"`
	CustomerName    string            `json:"customer_name"`
	CustomerPhone   string            `json:"customer_phone"`
	Pax             int               `json:"pax"`
	Items           []ReservationItem `json:"items"`
	DownPayment     float64           `json:"down_payment"`
	ReservationDate string            `json:"reservation_date"`
	ReservationTime string            `json:"reservation_time"`
	Status          string            `json:"status"`
	Notes           string            `json:"notes"`
}

// ── Public reservation page payloads ────────────────────────

type PublicProduct struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	PhotoURL string  `json:"photo_url"`
}

type PublicCategory struct {
	Name     string          `json:"name"`
	Products []PublicProduct `json:"products"`
}

type PublicMenu struct {
	OutletID   string           `json:"outlet_id"`
	OutletName string           `json:"outlet_name"`
	Slug       string           `json:"slug"`
	Categories []PublicCategory `json:"categories"`
}
