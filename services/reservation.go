package services

import (
	"encoding/json"
	"fmt"
	"strings"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/lib/pq"
)

func resvScopeCond(alias string, outletIDs []string, idx int) (string, []interface{}) {
	if outletIDs == nil {
		return "", nil
	}
	col := "outlet_id"
	if alias != "" {
		col = alias + ".outlet_id"
	}
	return fmt.Sprintf(" AND %s = ANY($%d::text[])", col, idx), []interface{}{pq.Array(outletIDs)}
}

// resolveReservationItems looks up each item's authoritative name & price from
// cloud_products (within the outlet) to prevent tampering, returning items + subtotal.
func resolveReservationItems(outletID string, items []models.ReservationItem) ([]models.ReservationItem, float64) {
	out := make([]models.ReservationItem, 0, len(items))
	var subtotal float64
	for _, it := range items {
		if it.Qty <= 0 {
			continue
		}
		name, price := it.ProductName, it.Price
		if it.ProductID != "" {
			var n string
			var p float64
			if err := database.DB.QueryRow(
				`SELECT name, price FROM cloud_products WHERE id = $1 AND outlet_id = $2 AND is_deleted = false`,
				it.ProductID, outletID).Scan(&n, &p); err == nil {
				name, price = n, p
			}
		}
		line := models.ReservationItem{ProductID: it.ProductID, ProductName: name, Qty: it.Qty, Price: price, Subtotal: price * float64(it.Qty)}
		subtotal += line.Subtotal
		out = append(out, line)
	}
	return out, subtotal
}

func scanReservation(scan func(dest ...interface{}) error) (*models.Reservation, error) {
	var r models.Reservation
	var itemsJSON string
	if err := scan(&r.ID, &r.OutletID, &r.OutletName, &r.CustomerName, &r.CustomerPhone, &r.Pax,
		&itemsJSON, &r.Subtotal, &r.DownPayment, &r.Total, &r.ReservationDate, &r.ReservationTime,
		&r.Status, &r.Notes, &r.Source, &r.CreatedAt, &r.UpdatedAt); err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(itemsJSON), &r.Items)
	if r.Items == nil {
		r.Items = []models.ReservationItem{}
	}
	r.Remaining = r.Total - r.DownPayment
	return &r, nil
}

const reservationCols = `r.id, r.outlet_id, COALESCE(o.name,''), r.customer_name, r.customer_phone, r.pax,
	COALESCE(r.items::text,'[]'), r.subtotal, r.down_payment, r.total,
	COALESCE(TO_CHAR(r.reservation_date,'YYYY-MM-DD'),''), r.reservation_time, r.status, r.notes, r.source,
	r.created_at, r.updated_at`

func ListReservations(outletID, status, dateFrom, dateTo string, outletScope []string) ([]models.Reservation, error) {
	conds := []string{"1=1"}
	args := []interface{}{}
	idx := 1
	if outletID != "" {
		conds = append(conds, fmt.Sprintf("r.outlet_id = $%d", idx))
		args = append(args, outletID)
		idx++
	}
	if status != "" {
		conds = append(conds, fmt.Sprintf("r.status = $%d", idx))
		args = append(args, status)
		idx++
	}
	if dateFrom != "" {
		conds = append(conds, fmt.Sprintf("r.reservation_date >= $%d::date", idx))
		args = append(args, dateFrom)
		idx++
	}
	if dateTo != "" {
		conds = append(conds, fmt.Sprintf("r.reservation_date <= $%d::date", idx))
		args = append(args, dateTo)
		idx++
	}
	if sc, sa := resvScopeCond("r", outletScope, idx); sc != "" {
		conds = append(conds, strings.TrimPrefix(sc, " AND "))
		args = append(args, sa...)
	}
	q := fmt.Sprintf(`SELECT %s FROM reservations r LEFT JOIN outlets o ON o.id = r.outlet_id
		WHERE %s ORDER BY r.reservation_date DESC NULLS LAST, r.reservation_time DESC, r.created_at DESC`,
		reservationCols, strings.Join(conds, " AND "))
	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.Reservation, 0)
	for rows.Next() {
		r, err := scanReservation(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

func GetReservation(id string, outletScope []string) (*models.Reservation, error) {
	sc, sa := resvScopeCond("r", outletScope, 2)
	args := append([]interface{}{id}, sa...)
	q := fmt.Sprintf(`SELECT %s FROM reservations r LEFT JOIN outlets o ON o.id = r.outlet_id WHERE r.id = $1%s`, reservationCols, sc)
	return scanReservation(database.DB.QueryRow(q, args...).Scan)
}

func saveReservation(id string, req models.ReservationRequest, source string) (*models.Reservation, error) {
	if strings.TrimSpace(req.CustomerName) == "" {
		return nil, fmt.Errorf("nama pemesan wajib diisi")
	}
	if req.OutletID == "" {
		return nil, fmt.Errorf("outlet wajib dipilih")
	}
	if req.Pax <= 0 {
		req.Pax = 1
	}
	items, subtotal := resolveReservationItems(req.OutletID, req.Items)
	itemsJSON, _ := json.Marshal(items)
	total := subtotal
	status := req.Status
	if status == "" {
		status = "pending"
	}
	if id == "" {
		id = NewULID()
		_, err := database.DB.Exec(`
			INSERT INTO reservations (id, outlet_id, customer_name, customer_phone, pax, items,
				subtotal, down_payment, total, reservation_date, reservation_time, status, notes, source, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9, NULLIF($10,'')::date, $11,$12,$13,$14, NOW(), NOW())`,
			id, req.OutletID, req.CustomerName, req.CustomerPhone, req.Pax, string(itemsJSON),
			subtotal, req.DownPayment, total, req.ReservationDate, req.ReservationTime, status, req.Notes, source)
		if err != nil {
			return nil, err
		}
	} else {
		_, err := database.DB.Exec(`
			UPDATE reservations SET customer_name=$1, customer_phone=$2, pax=$3, items=$4,
				subtotal=$5, down_payment=$6, total=$7, reservation_date=NULLIF($8,'')::date,
				reservation_time=$9, status=$10, notes=$11, updated_at=NOW()
			WHERE id=$12`,
			req.CustomerName, req.CustomerPhone, req.Pax, string(itemsJSON),
			subtotal, req.DownPayment, total, req.ReservationDate, req.ReservationTime, status, req.Notes, id)
		if err != nil {
			return nil, err
		}
	}
	return GetReservation(id, nil)
}

func CreateReservation(req models.ReservationRequest, outletScope []string) (*models.Reservation, error) {
	return saveReservation("", req, "admin")
}

func UpdateReservation(id string, req models.ReservationRequest, outletScope []string) (*models.Reservation, error) {
	if _, err := GetReservation(id, outletScope); err != nil {
		return nil, fmt.Errorf("reservasi tidak ditemukan")
	}
	existing, _ := GetReservation(id, outletScope)
	req.OutletID = existing.OutletID // outlet immutable on edit
	return saveReservation(id, req, existing.Source)
}

func UpdateReservationStatus(id, status string, outletScope []string) (*models.Reservation, error) {
	r, err := GetReservation(id, outletScope)
	if err != nil {
		return nil, fmt.Errorf("reservasi tidak ditemukan")
	}
	if _, err := database.DB.Exec(`UPDATE reservations SET status=$1, updated_at=NOW() WHERE id=$2`, status, id); err != nil {
		return nil, err
	}
	r.Status = status
	return r, nil
}

func DeleteReservation(id string, outletScope []string) error {
	if _, err := GetReservation(id, outletScope); err != nil {
		return fmt.Errorf("reservasi tidak ditemukan")
	}
	_, err := database.DB.Exec(`DELETE FROM reservations WHERE id=$1`, id)
	return err
}

// ── Public (no auth) ────────────────────────────────────────

// GetOutletBySlug resolves an active outlet id/name by its public slug.
func GetOutletBySlug(slug string) (string, string, error) {
	var id, name string
	err := database.DB.QueryRow(`SELECT id, name FROM outlets WHERE slug = $1 AND is_active = true`, slug).Scan(&id, &name)
	return id, name, err
}

// GetPublicMenu returns the outlet's products grouped by category (with photos),
// for the public reservation page.
func GetPublicMenu(slug string) (*models.PublicMenu, error) {
	outletID, outletName, err := GetOutletBySlug(slug)
	if err != nil {
		return nil, fmt.Errorf("outlet tidak ditemukan")
	}
	rows, err := database.DB.Query(`
		SELECT COALESCE(NULLIF(category_name,''),'Lainnya') AS cat, id, name, price, COALESCE(photo_url,'')
		FROM cloud_products
		WHERE outlet_id = $1 AND is_deleted = false
		ORDER BY cat ASC, name ASC`, outletID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	menu := &models.PublicMenu{OutletID: outletID, OutletName: outletName, Slug: slug, Categories: []models.PublicCategory{}}
	idxByCat := map[string]int{}
	for rows.Next() {
		var cat string
		var p models.PublicProduct
		if err := rows.Scan(&cat, &p.ID, &p.Name, &p.Price, &p.PhotoURL); err != nil {
			return nil, err
		}
		i, ok := idxByCat[cat]
		if !ok {
			i = len(menu.Categories)
			idxByCat[cat] = i
			menu.Categories = append(menu.Categories, models.PublicCategory{Name: cat, Products: []models.PublicProduct{}})
		}
		menu.Categories[i].Products = append(menu.Categories[i].Products, p)
	}
	return menu, rows.Err()
}

// CreatePublicReservation creates a pending reservation from the public page.
func CreatePublicReservation(slug string, req models.ReservationRequest) (*models.Reservation, error) {
	outletID, _, err := GetOutletBySlug(slug)
	if err != nil {
		return nil, fmt.Errorf("outlet tidak ditemukan")
	}
	req.OutletID = outletID
	req.Status = "pending"
	return saveReservation("", req, "public")
}
