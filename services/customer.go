package services

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/lib/pq"
)

// NormalizeCustomerPhone menyamakan format nomor HP agar pencocokan konsisten:
// buang non-digit, prefiks internasional 62 diubah ke 0 (0812… == +62812…).
// Nomor terlalu pendek (<6 digit) dianggap bukan nomor valid → "".
func NormalizeCustomerPhone(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	d := b.String()
	if strings.HasPrefix(d, "62") && len(d) > 9 {
		d = "0" + d[2:]
	}
	if len(d) < 6 {
		return ""
	}
	return d
}

// findCustomerID mencari pelanggan existing: prioritas nomor HP; order tanpa HP
// dicocokkan per nama (case-insensitive) — bila ganda, pilih yang punya HP /
// paling lama terdaftar agar hasilnya deterministik.
func findCustomerID(name, phone string) string {
	var id string
	if phone != "" {
		if err := database.DB.QueryRow(
			`SELECT id FROM customers WHERE phone = $1`, phone).Scan(&id); err == nil {
			return strings.TrimSpace(id)
		}
	}
	if name == "" {
		return ""
	}
	q := `SELECT id FROM customers WHERE LOWER(name) = LOWER($1)
		ORDER BY (phone <> '') DESC, created_at ASC LIMIT 1`
	if phone != "" {
		// HP baru (belum terdaftar): hanya boleh menempel ke pelanggan senama
		// yang BELUM punya HP — jangan membajak pelanggan lain yang HP-nya beda.
		q = `SELECT id FROM customers WHERE phone = '' AND LOWER(name) = LOWER($1)
			ORDER BY created_at ASC LIMIT 1`
	}
	if err := database.DB.QueryRow(q, name).Scan(&id); err == nil {
		return strings.TrimSpace(id)
	}
	return ""
}

// UpsertCustomerForOrder membuat/mencocokkan pelanggan dari data order lalu
// menautkan cloud_orders.customer_id. Dipanggil setiap order tersimpan; aman
// dipanggil berulang (idempotent).
func UpsertCustomerForOrder(orderID, name, phone string) {
	name = strings.TrimSpace(name)
	phoneN := NormalizeCustomerPhone(phone)
	if name == "" && phoneN == "" {
		return
	}

	id := findCustomerID(name, phoneN)
	if id == "" {
		id = NewULID()
		res, err := database.DB.Exec(
			`INSERT INTO customers (id, name, phone) VALUES ($1, $2, $3)
			 ON CONFLICT DO NOTHING`, id, name, phoneN)
		if err != nil {
			log.Printf("insert customer (order=%s): %v", orderID, err)
			return
		}
		// Konflik unique phone (race dgn backfill/order paralel) → pakai baris pemenang.
		if n, _ := res.RowsAffected(); n == 0 {
			if id = findCustomerID(name, phoneN); id == "" {
				return
			}
		}
	} else {
		// Perkaya data: nama terbaru menang (koreksi ejaan kasir), HP diisi
		// bila sebelumnya kosong.
		if _, err := database.DB.Exec(
			`UPDATE customers SET
				name = CASE WHEN $2 <> '' THEN $2 ELSE name END,
				phone = CASE WHEN phone = '' AND $3 <> '' THEN $3 ELSE phone END,
				updated_at = now() AT TIME ZONE 'UTC'
			 WHERE id = $1`, id, name, phoneN); err != nil {
			log.Printf("update customer %s (order=%s): %v", id, orderID, err)
		}
	}

	if _, err := database.DB.Exec(
		`UPDATE cloud_orders SET customer_id = $2 WHERE id = $1`, orderID, id); err != nil {
		log.Printf("link customer %s ke order %s: %v", id, orderID, err)
	}
}

// BackfillCustomers memproses cloud_orders lama (sebelum fitur pelanggan ada)
// SEKALI saat boot: bentuk master pelanggan + tautkan customer_id, urut waktu
// agar identitas mengikuti kronologi kunjungan. Marker di app_settings.
func BackfillCustomers() {
	var done int
	database.DB.QueryRow(`SELECT COUNT(*) FROM app_settings WHERE key = 'mig_customers_backfill'`).Scan(&done)
	if done > 0 {
		return
	}

	rows, err := database.DB.Query(
		`SELECT TRIM(id), COALESCE(customer_name,''), COALESCE(customer_phone,'')
		 FROM cloud_orders
		 WHERE customer_id IS NULL AND (COALESCE(customer_name,'') <> '' OR COALESCE(customer_phone,'') <> '')
		 ORDER BY created_at ASC`)
	if err != nil {
		log.Printf("backfill customers: %v", err)
		return
	}
	type ord struct{ id, name, phone string }
	orders := []ord{}
	for rows.Next() {
		var o ord
		if err := rows.Scan(&o.id, &o.name, &o.phone); err == nil {
			orders = append(orders, o)
		}
	}
	rows.Close()

	n := 0
	for _, o := range orders {
		UpsertCustomerForOrder(o.id, o.name, o.phone)
		n++
	}
	database.DB.Exec(`INSERT INTO app_settings (key, value) VALUES ('mig_customers_backfill', 'done') ON CONFLICT (key) DO NOTHING`)
	log.Printf("Backfill pelanggan selesai: %d order lama diproses", n)
}

func custScopeCond(outletIDs []string, idx int) (string, []interface{}) {
	if outletIDs == nil {
		return "", nil
	}
	return fmt.Sprintf(" AND o.outlet_id = ANY($%d::text[])", idx), []interface{}{pq.Array(outletIDs)}
}

// ListCustomers mengembalikan pelanggan + rekap kunjungan (dihitung dari
// cloud_orders, difilter scope outlet admin). Pelanggan tanpa order di scope
// tidak ikut tampil.
func ListCustomers(search string, outletScope []string, page, limit int) ([]models.Customer, int, error) {
	args := []interface{}{}
	idx := 1

	where := "1=1"
	if s := strings.TrimSpace(search); s != "" {
		// Cari per nomor hanya bila kata kunci mengandung nomor valid — hasil
		// normalisasi kosong akan membuat LIKE '%%' cocok ke semua baris.
		if pn := NormalizeCustomerPhone(s); pn != "" {
			where = fmt.Sprintf("(c.name ILIKE $%d OR c.phone LIKE $%d)", idx, idx+1)
			args = append(args, "%"+s+"%", "%"+pn+"%")
			idx += 2
		} else {
			where = fmt.Sprintf("c.name ILIKE $%d", idx)
			args = append(args, "%"+s+"%")
			idx++
		}
	}
	joinCond := "o.customer_id = c.id"
	if sc, sa := custScopeCond(outletScope, idx); sc != "" {
		joinCond += sc
		args = append(args, sa...)
		idx++
	}

	var total int
	countQ := fmt.Sprintf(
		`SELECT COUNT(DISTINCT c.id) FROM customers c JOIN cloud_orders o ON %s WHERE %s`, joinCond, where)
	if err := database.DB.QueryRow(countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count customers: %w", err)
	}

	q := fmt.Sprintf(
		`SELECT TRIM(c.id), c.name, c.phone,
			COUNT(o.id), COALESCE(SUM(o.total_amount),0),
			MIN(o.created_at), MAX(o.created_at),
			COALESCE(STRING_AGG(DISTINCT ot.name, ', '), '')
		 FROM customers c
		 JOIN cloud_orders o ON %s
		 LEFT JOIN outlets ot ON ot.id = o.outlet_id
		 WHERE %s
		 GROUP BY c.id, c.name, c.phone
		 ORDER BY MAX(o.created_at) DESC
		 LIMIT $%d OFFSET $%d`, joinCond, where, idx, idx+1)
	args = append(args, limit, (page-1)*limit)

	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]models.Customer, 0)
	for rows.Next() {
		var c models.Customer
		var first, last sql.NullTime
		if err := rows.Scan(&c.ID, &c.Name, &c.Phone, &c.VisitCount, &c.TotalSpent,
			&first, &last, &c.OutletNames); err != nil {
			return nil, 0, err
		}
		if first.Valid {
			c.FirstVisitAt = &first.Time
		}
		if last.Valid {
			c.LastVisitAt = &last.Time
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

// GetCustomerDetail mengembalikan profil pelanggan + rekap per outlet + daftar
// kunjungan (order beserta item pesanan), difilter scope outlet admin.
func GetCustomerDetail(id string, outletScope []string) (*models.CustomerDetail, error) {
	var d models.CustomerDetail
	if err := database.DB.QueryRow(
		`SELECT TRIM(id), name, phone FROM customers WHERE id = $1`, id).
		Scan(&d.Customer.ID, &d.Customer.Name, &d.Customer.Phone); err != nil {
		return nil, fmt.Errorf("pelanggan tidak ditemukan")
	}

	args := []interface{}{id}
	scopeSQL := ""
	if sc, sa := custScopeCond(outletScope, 2); sc != "" {
		scopeSQL = sc
		args = append(args, sa...)
	}

	// Rekap per outlet
	rows, err := database.DB.Query(fmt.Sprintf(
		`SELECT o.outlet_id, COALESCE(ot.name, o.outlet_code),
			COUNT(o.id), COALESCE(SUM(o.total_amount),0), MAX(o.created_at)
		 FROM cloud_orders o LEFT JOIN outlets ot ON ot.id = o.outlet_id
		 WHERE o.customer_id = $1%s
		 GROUP BY o.outlet_id, ot.name, o.outlet_code
		 ORDER BY MAX(o.created_at) DESC`, scopeSQL), args...)
	if err != nil {
		return nil, err
	}
	d.Outlets = make([]models.CustomerOutletSummary, 0)
	for rows.Next() {
		var s models.CustomerOutletSummary
		var last sql.NullTime
		if err := rows.Scan(&s.OutletID, &s.OutletName, &s.VisitCount, &s.TotalSpent, &last); err != nil {
			rows.Close()
			return nil, err
		}
		if last.Valid {
			s.LastVisitAt = &last.Time
		}
		d.Outlets = append(d.Outlets, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Daftar kunjungan + item pesanan (terbaru dulu, dibatasi 200 kunjungan)
	rows, err = database.DB.Query(fmt.Sprintf(
		`SELECT TRIM(o.id), o.outlet_id, o.outlet_code, COALESCE(ot.name, o.outlet_code),
			COALESCE(o.table_number,''), o.pax, o.status, o.total_amount,
			COALESCE(o.items::text,'[]'), o.created_at
		 FROM cloud_orders o LEFT JOIN outlets ot ON ot.id = o.outlet_id
		 WHERE o.customer_id = $1%s
		 ORDER BY o.created_at DESC LIMIT 200`, scopeSQL), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	d.Visits = make([]models.CustomerVisit, 0)
	for rows.Next() {
		var v models.CustomerVisit
		var items string
		if err := rows.Scan(&v.OrderID, &v.OutletID, &v.OutletCode, &v.OutletName,
			&v.TableNumber, &v.Pax, &v.Status, &v.TotalAmount, &items, &v.CreatedAt); err != nil {
			return nil, err
		}
		v.Items = []byte(items)
		d.Visits = append(d.Visits, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Agregat profil dari seluruh kunjungan dalam scope
	for _, s := range d.Outlets {
		d.Customer.VisitCount += s.VisitCount
		d.Customer.TotalSpent += s.TotalSpent
	}
	if len(d.Visits) > 0 {
		last := d.Visits[0].CreatedAt
		first := d.Visits[len(d.Visits)-1].CreatedAt
		d.Customer.LastVisitAt = &last
		d.Customer.FirstVisitAt = &first
	}
	if len(d.Outlets) > 0 {
		names := make([]string, 0, len(d.Outlets))
		for _, s := range d.Outlets {
			names = append(names, s.OutletName)
		}
		d.Customer.OutletNames = strings.Join(names, ", ")
	}
	return &d, nil
}
