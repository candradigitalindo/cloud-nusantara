package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"
)

// Batas wajar satu pesanan tamu — mencegah satu permintaan iseng membuat
// ratusan baris item yang harus ditarik dan dicetak POS.
const (
	maxOnlineOrderItems = 50
	maxOnlineOrderQty   = 99
)

// CreatePublicOrder mencatat pesanan dari halaman QR tamu.
//
// Nama menu dan harga TIDAK diambil dari kiriman tamu: hanya product_local_id
// dan qty yang dipercaya, sisanya dibaca ulang dari master menu outlet. Tanpa
// ini, badan permintaan bisa dipalsukan untuk memesan menu dengan harga sendiri.
func CreatePublicOrder(slug string, req models.PublicOrderRequest) (*models.OnlineOrder, error) {
	outletID, _, err := GetOutletBySlug(slug)
	if err != nil {
		return nil, fmt.Errorf("outlet tidak ditemukan")
	}

	if strings.TrimSpace(req.TableNumber) == "" {
		return nil, fmt.Errorf("nomor meja wajib diisi")
	}
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("pesanan masih kosong")
	}
	if len(req.Items) > maxOnlineOrderItems {
		return nil, fmt.Errorf("terlalu banyak jenis menu dalam satu pesanan")
	}

	// Master menu & add-on dibaca sekali untuk memvalidasi seluruh baris.
	validProducts, err := productNamesByLocalID(outletID)
	if err != nil {
		return nil, err
	}
	addonsByProduct, err := GetProductAddons(outletID)
	if err != nil {
		return nil, err
	}

	items := make([]models.OnlineOrderItem, 0, len(req.Items))
	for _, it := range req.Items {
		name, ok := validProducts[it.ProductLocalID]
		if !ok {
			return nil, fmt.Errorf("menu tidak tersedia")
		}
		if it.Qty < 1 || it.Qty > maxOnlineOrderQty {
			return nil, fmt.Errorf("jumlah untuk %s tidak valid", name)
		}

		// Hanya add-on yang benar-benar terdaftar pada menu itu yang diterima.
		allowed := map[string]models.CloudProductAddon{}
		for _, a := range addonsByProduct[it.ProductLocalID] {
			allowed[a.LocalID] = a
		}
		picked := make([]models.OnlineOrderAddon, 0, len(it.Addons))
		for _, a := range it.Addons {
			master, ok := allowed[a.ID]
			if !ok {
				return nil, fmt.Errorf("tambahan tidak tersedia untuk %s", name)
			}
			picked = append(picked, models.OnlineOrderAddon{
				ID: master.LocalID, Name: master.Name, Price: master.Price,
			})
		}

		items = append(items, models.OnlineOrderItem{
			ProductLocalID: it.ProductLocalID,
			ProductName:    name,
			Qty:            it.Qty,
			Notes:          trimTo(it.Notes, 120),
			Addons:         picked,
		})
	}

	payload, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}

	id := newChargeID()
	_, err = database.DB.Exec(
		`INSERT INTO online_orders (id, outlet_id, table_number, customer_name,
			customer_phone, notes, items, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'new', NOW(), NOW())`,
		id, outletID, trimTo(req.TableNumber, 20), trimTo(req.CustomerName, 100),
		trimTo(req.CustomerPhone, 30), trimTo(req.Notes, 200), payload,
	)
	if err != nil {
		return nil, err
	}

	go logSync(outletID, "online_order_created", "online_order", 1, "success", "")

	return &models.OnlineOrder{
		ID: id, OutletID: outletID, TableNumber: req.TableNumber,
		CustomerName: req.CustomerName, Items: items,
		Status: models.OnlineOrderNew,
	}, nil
}

func productNamesByLocalID(outletID string) (map[string]string, error) {
	rows, err := database.DB.Query(
		`SELECT local_id, name FROM cloud_products
		WHERE outlet_id = $1 AND is_deleted = false`, outletID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

func trimTo(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// PendingOnlineOrders mengembalikan pesanan yang menunggu ditarik POS.
//
// Termasuk pesanan yang klaimnya basi: perangkat yang mengambilnya tidak pernah
// mengonfirmasi (mati/kehilangan jaringan), jadi pesanan itu harus bisa diambil
// perangkat lain daripada menggantung selamanya.
func PendingOnlineOrders(outletID string) ([]models.OnlineOrder, error) {
	rows, err := database.DB.Query(
		`SELECT id, table_number, COALESCE(customer_name,''), COALESCE(customer_phone,''),
			COALESCE(notes,''), items, status, created_at
		FROM online_orders
		WHERE outlet_id = $1
			AND (status = 'new'
				OR (status = 'claimed' AND claimed_at < NOW() - INTERVAL '5 minutes'))
		ORDER BY created_at ASC LIMIT 50`,
		outletID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.OnlineOrder, 0)
	for rows.Next() {
		var o models.OnlineOrder
		var raw []byte
		var createdAt time.Time
		if err := rows.Scan(&o.ID, &o.TableNumber, &o.CustomerName, &o.CustomerPhone,
			&o.Notes, &raw, &o.Status, &createdAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &o.Items); err != nil {
			// Baris rusak dilewati, bukan menggagalkan seluruh penarikan —
			// satu pesanan cacat tidak boleh memblokir sisanya.
			continue
		}
		o.OutletID = outletID
		o.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		out = append(out, o)
	}
	return out, rows.Err()
}

// ClaimOnlineOrder menandai pesanan sedang diambil satu perangkat.
//
// Atomik: hanya SATU perangkat yang bisa menang, sehingga dua kasir yang
// menarik bersamaan tidak menghasilkan order meja dobel. Perangkat yang sama
// boleh mengklaim ulang (idempoten) agar percobaan ulang setelah gagal jaringan
// tidak buntu.
func ClaimOnlineOrder(outletID, orderID, deviceID string) (bool, error) {
	res, err := database.DB.Exec(
		`UPDATE online_orders
		SET status = 'claimed', claimed_by = $3, claimed_at = NOW(), updated_at = NOW()
		WHERE outlet_id = $1 AND id = $2
			AND (status = 'new'
				OR (status = 'claimed'
					AND (claimed_by = $3 OR claimed_at < NOW() - INTERVAL '5 minutes')))`,
		outletID, orderID, deviceID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ConfirmOnlineOrder menutup siklus: POS sudah berhasil membuat order meja.
// Setelah ini pesanan tidak pernah ditarik lagi, walau klaimnya lewat waktu.
func ConfirmOnlineOrder(outletID, orderID, localOrderID string) error {
	_, err := database.DB.Exec(
		`UPDATE online_orders
		SET status = 'confirmed', local_order_id = $3, updated_at = NOW()
		WHERE outlet_id = $1 AND id = $2 AND status <> 'rejected'`,
		outletID, orderID, nullStr(localOrderID),
	)
	return err
}

// RejectOnlineOrder dipakai saat pesanan tak bisa diproses (mis. mejanya sudah
// dipakai tamu lain). Disimpan, bukan dihapus, agar tetap terlihat di riwayat.
func RejectOnlineOrder(outletID, orderID, reason string) error {
	_, err := database.DB.Exec(
		`UPDATE online_orders
		SET status = 'rejected', reject_reason = $3, updated_at = NOW()
		WHERE outlet_id = $1 AND id = $2 AND status <> 'confirmed'`,
		outletID, orderID, nullStr(trimTo(reason, 200)),
	)
	return err
}

// OnlineOrderStatus dipakai halaman tamu untuk menampilkan perkembangan
// pesanannya tanpa perlu login.
func OnlineOrderStatus(slug, orderID string) (string, error) {
	outletID, _, err := GetOutletBySlug(slug)
	if err != nil {
		return "", fmt.Errorf("outlet tidak ditemukan")
	}
	var status string
	err = database.DB.QueryRow(
		`SELECT status FROM online_orders WHERE outlet_id = $1 AND id = $2`,
		outletID, orderID,
	).Scan(&status)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("pesanan tidak ditemukan")
	}
	return status, err
}
