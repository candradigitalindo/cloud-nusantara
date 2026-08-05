package models

import (
	"encoding/json"
	"time"
)

// Customer adalah master pelanggan yang terbentuk otomatis dari order kasir
// (customer_name/customer_phone di cloud_orders). Identitas utama = nomor HP.
type Customer struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Phone        string     `json:"phone"`
	VisitCount   int        `json:"visit_count"`
	OutletNames  string     `json:"outlet_names"`
	TotalSpent   float64    `json:"total_spent"`
	FirstVisitAt *time.Time `json:"first_visit_at"`
	LastVisitAt  *time.Time `json:"last_visit_at"`
}

// CustomerVisit = satu kunjungan (order) pelanggan di sebuah outlet.
type CustomerVisit struct {
	OrderID     string          `json:"order_id"`
	OutletID    string          `json:"outlet_id"`
	OutletCode  string          `json:"outlet_code"`
	OutletName  string          `json:"outlet_name"`
	TableNumber string          `json:"table_number"`
	Pax         int             `json:"pax"`
	Status      string          `json:"status"`
	TotalAmount float64         `json:"total_amount"`
	Items       json.RawMessage `json:"items"`
	CreatedAt   time.Time       `json:"created_at"`
}

// CustomerOutletSummary = rekap kunjungan pelanggan per outlet.
type CustomerOutletSummary struct {
	OutletID    string     `json:"outlet_id"`
	OutletName  string     `json:"outlet_name"`
	VisitCount  int        `json:"visit_count"`
	TotalSpent  float64    `json:"total_spent"`
	LastVisitAt *time.Time `json:"last_visit_at"`
}

type CustomerDetail struct {
	Customer Customer                `json:"customer"`
	Outlets  []CustomerOutletSummary `json:"outlets"`
	Visits   []CustomerVisit         `json:"visits"`
}
