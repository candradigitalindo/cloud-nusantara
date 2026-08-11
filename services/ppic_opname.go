package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"fmt"
	"time"
)

func generateOpnameNumber() string {
	now := time.Now()
	prefix := "OPN-" + now.Format("20060102") + "-"
	var seq int
	database.DB.QueryRow(`SELECT COUNT(*)+1 FROM stock_opnames WHERE opname_number LIKE $1`, prefix+"%").Scan(&seq)
	return fmt.Sprintf("%s%03d", prefix, seq)
}

// CreateStockOpname membuka sesi opname untuk satu gudang: qty sistem di-snapshot
// SAAT SESI DIBUAT (basis blind count). Cakupan: semua item ber-ledger di gudang
// itu, atau dibatasi satu kategori.
func CreateStockOpname(req models.StockOpnameCreateRequest, actor string) (*models.StockOpname, error) {
	if req.WarehouseID == "" {
		return nil, fmt.Errorf("warehouse_id wajib diisi")
	}

	// Satu sesi aktif per gudang — mencegah snapshot ganda yang saling menimpa.
	var active int
	database.DB.QueryRow(`SELECT COUNT(*) FROM stock_opnames WHERE warehouse_id = $1 AND status IN ('counting','review')`,
		req.WarehouseID).Scan(&active)
	if active > 0 {
		return nil, fmt.Errorf("masih ada sesi opname aktif untuk gudang ini — selesaikan atau batalkan dulu")
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	id := NewULID()
	number := generateOpnameNumber()
	if _, err := tx.Exec(`
		INSERT INTO stock_opnames (id, opname_number, warehouse_id, category, status, notes, created_by)
		VALUES ($1, $2, $3, $4, 'counting', $5, $6)`,
		id, number, req.WarehouseID, req.Category, req.Notes, actor); err != nil {
		return nil, err
	}

	// Snapshot item: seluruh baris ledger gudang ini (termasuk qty 0 — selisih
	// bisa positif), item aktif saja, opsional filter kategori.
	rows, err := tx.Query(`
		SELECT sl.item_id, sl.qty_base, sl.avg_cost
		FROM stock_ledger sl
		JOIN stock_items si ON si.id = sl.item_id AND si.is_active = true
		WHERE sl.warehouse_id = $1 AND ($2 = '' OR si.category = $2)
		ORDER BY si.name`, req.WarehouseID, req.Category)
	if err != nil {
		return nil, err
	}
	type snap struct {
		itemID string
		qty    float64
		cost   float64
	}
	var snaps []snap
	for rows.Next() {
		var s snap
		if err := rows.Scan(&s.itemID, &s.qty, &s.cost); err != nil {
			rows.Close()
			return nil, err
		}
		snaps = append(snaps, s)
	}
	rows.Close()
	if len(snaps) == 0 {
		return nil, fmt.Errorf("tidak ada item untuk dihitung di gudang/kategori ini")
	}
	for _, s := range snaps {
		if _, err := tx.Exec(`
			INSERT INTO stock_opname_items (id, opname_id, item_id, qty_system_base, cost_per_base)
			VALUES ($1, $2, $3, $4, $5)`,
			NewULID(), id, s.itemID, s.qty, s.cost); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetStockOpname(id)
}

// ListStockOpnames — daftar sesi + agregat kemajuan/selisih, dalam scope outlet.
func ListStockOpnames(warehouseID, status string, outletIDs []string, page, limit int) ([]models.StockOpname, int, error) {
	scope, scopeArgs := ppicWarehouseScope(outletIDs)

	where := ` WHERE w.is_active = true` + scope
	args := append([]interface{}{}, scopeArgs...)
	if warehouseID != "" {
		args = append(args, warehouseID)
		where += fmt.Sprintf(" AND so.warehouse_id = $%d", len(args))
	}
	if status != "" {
		args = append(args, status)
		where += fmt.Sprintf(" AND so.status = $%d", len(args))
	}

	base := `
		FROM stock_opnames so
		JOIN warehouses w ON w.id = so.warehouse_id
		LEFT JOIN outlets o ON o.id = w.outlet_id`

	var total int
	if err := database.DB.QueryRow(`SELECT COUNT(*)`+base+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT so.id, so.opname_number, so.warehouse_id, w.name, w.type, COALESCE(o.name, '—'),
			COALESCE(so.category, ''), so.status, COALESCE(so.notes, ''),
			COALESCE(so.created_by, ''), COALESCE(so.approved_by, ''), so.approved_at::text,
			so.created_at::text, so.updated_at::text,
			(SELECT COUNT(*) FROM stock_opname_items i WHERE i.opname_id = so.id),
			(SELECT COUNT(*) FROM stock_opname_items i WHERE i.opname_id = so.id AND i.qty_counted_base IS NOT NULL),
			(SELECT COUNT(*) FROM stock_opname_items i WHERE i.opname_id = so.id AND i.qty_counted_base IS NOT NULL AND i.qty_counted_base <> i.qty_system_base),
			COALESCE((SELECT SUM((i.qty_counted_base - i.qty_system_base) * i.cost_per_base)
				FROM stock_opname_items i WHERE i.opname_id = so.id AND i.qty_counted_base IS NOT NULL), 0)
		%s%s ORDER BY so.created_at DESC LIMIT %d OFFSET %d`, base, where, limit, (page-1)*limit), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := []models.StockOpname{}
	for rows.Next() {
		var so models.StockOpname
		if err := rows.Scan(&so.ID, &so.OpnameNumber, &so.WarehouseID, &so.WarehouseName, &so.WarehouseType, &so.OutletName,
			&so.Category, &so.Status, &so.Notes,
			&so.CreatedBy, &so.ApprovedBy, &so.ApprovedAt,
			&so.CreatedAt, &so.UpdatedAt,
			&so.ItemsTotal, &so.ItemsCounted, &so.ItemsDiff, &so.DiffValue); err != nil {
			return nil, 0, err
		}
		if so.ItemsCounted > 0 {
			so.AccuracyPct = float64(so.ItemsCounted-so.ItemsDiff) / float64(so.ItemsCounted) * 100
		}
		list = append(list, so)
	}
	return list, total, nil
}

// GetStockOpname — detail sesi lengkap dengan item & selisih.
func GetStockOpname(id string) (*models.StockOpname, error) {
	var so models.StockOpname
	err := database.DB.QueryRow(`
		SELECT so.id, so.opname_number, so.warehouse_id, w.name, w.type, COALESCE(o.name, '—'),
			COALESCE(so.category, ''), so.status, COALESCE(so.notes, ''),
			COALESCE(so.created_by, ''), COALESCE(so.approved_by, ''), so.approved_at::text,
			so.created_at::text, so.updated_at::text
		FROM stock_opnames so
		JOIN warehouses w ON w.id = so.warehouse_id
		LEFT JOIN outlets o ON o.id = w.outlet_id
		WHERE so.id = $1`, id).Scan(
		&so.ID, &so.OpnameNumber, &so.WarehouseID, &so.WarehouseName, &so.WarehouseType, &so.OutletName,
		&so.Category, &so.Status, &so.Notes,
		&so.CreatedBy, &so.ApprovedBy, &so.ApprovedAt,
		&so.CreatedAt, &so.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("sesi opname tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}

	rows, err := database.DB.Query(`
		SELECT i.id, i.item_id, si.code, si.name, si.category, si.base_unit,
			si.dist_unit, si.dist_ratio, si.dist_unit_label,
			i.qty_system_base, i.qty_counted_base, i.cost_per_base, COALESCE(i.reason, '')
		FROM stock_opname_items i
		JOIN stock_items si ON si.id = i.item_id
		WHERE i.opname_id = $1 ORDER BY si.name`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	so.Items = []models.StockOpnameItem{}
	for rows.Next() {
		var it models.StockOpnameItem
		if err := rows.Scan(&it.ID, &it.ItemID, &it.ItemCode, &it.ItemName, &it.Category, &it.BaseUnit,
			&it.DistUnit, &it.DistRatio, &it.DistUnitLabel,
			&it.QtySystemBase, &it.QtyCountedBase, &it.CostPerBase, &it.Reason); err != nil {
			return nil, err
		}
		so.ItemsTotal++
		if it.QtyCountedBase != nil {
			so.ItemsCounted++
			it.DiffBase = *it.QtyCountedBase - it.QtySystemBase
			it.DiffValue = it.DiffBase * it.CostPerBase
			if it.DiffBase != 0 {
				so.ItemsDiff++
			}
			so.DiffValue += it.DiffValue
		}
		so.Items = append(so.Items, it)
	}
	if so.ItemsCounted > 0 {
		so.AccuracyPct = float64(so.ItemsCounted-so.ItemsDiff) / float64(so.ItemsCounted) * 100
	}
	return &so, nil
}

// OpnameInScope memeriksa sesi opname berada di gudang yang boleh diakses.
func OpnameInScope(id string, outletIDs []string) bool {
	var whID string
	if err := database.DB.QueryRow(`SELECT warehouse_id FROM stock_opnames WHERE id = $1`, id).Scan(&whID); err != nil {
		return false
	}
	return WarehouseMutableInScope(whID, outletIDs)
}

// UpdateOpnameCounts mengisi hasil hitung fisik (parsial boleh, berkali-kali
// boleh) selama sesi masih counting/review.
func UpdateOpnameCounts(id string, req models.OpnameCountRequest, actor string) error {
	var status string
	if err := database.DB.QueryRow(`SELECT status FROM stock_opnames WHERE id = $1`, id).Scan(&status); err != nil {
		return fmt.Errorf("sesi opname tidak ditemukan")
	}
	if status != "counting" && status != "review" {
		return fmt.Errorf("sesi berstatus %s — hitungan tidak bisa diubah", status)
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, it := range req.Items {
		if it.QtyCountedBase != nil && *it.QtyCountedBase < 0 {
			return fmt.Errorf("qty fisik tidak boleh negatif")
		}
		res, err := tx.Exec(`
			UPDATE stock_opname_items SET qty_counted_base = $1, reason = $2
			WHERE opname_id = $3 AND item_id = $4`,
			it.QtyCountedBase, it.Reason, id, it.ItemID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("item %s tidak ada di sesi ini", it.ItemID)
		}
	}
	if _, err := tx.Exec(`UPDATE stock_opnames SET updated_at = NOW() AT TIME ZONE 'UTC' WHERE id = $1`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// SubmitStockOpname: counting → review. Minimal satu item sudah dihitung.
func SubmitStockOpname(id string) error {
	var status string
	var counted int
	err := database.DB.QueryRow(`
		SELECT so.status, (SELECT COUNT(*) FROM stock_opname_items i WHERE i.opname_id = so.id AND i.qty_counted_base IS NOT NULL)
		FROM stock_opnames so WHERE so.id = $1`, id).Scan(&status, &counted)
	if err != nil {
		return fmt.Errorf("sesi opname tidak ditemukan")
	}
	if status != "counting" {
		return fmt.Errorf("hanya sesi counting yang bisa diajukan review")
	}
	if counted == 0 {
		return fmt.Errorf("belum ada item yang dihitung")
	}
	_, err = database.DB.Exec(`UPDATE stock_opnames SET status = 'review', updated_at = NOW() AT TIME ZONE 'UTC' WHERE id = $1`, id)
	return err
}

// ApproveStockOpname memposting penyesuaian: untuk setiap item yang dihitung,
// stok gudang DISETEL ke qty fisik (selisih dihitung ulang terhadap qty ledger
// SAAT APPROVE, bukan snapshot — supaya hasil akhir = angka fisik meski ada
// pergerakan selama sesi berjalan). Item yang tidak dihitung dilewati.
func ApproveStockOpname(id, actor string) error {
	so, err := GetStockOpname(id)
	if err != nil {
		return err
	}
	if so.Status != "review" {
		return fmt.Errorf("hanya sesi berstatus review yang bisa di-approve")
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Kunci sesi & validasi ulang status di dalam transaksi: cek di atas berjalan
	// tanpa lock, dua approve bersamaan bisa sama-sama lolos dan memposting
	// penyesuaian dua kali.
	var curStatus string
	if err := tx.QueryRow(`SELECT status FROM stock_opnames WHERE id = $1 FOR UPDATE`, id).Scan(&curStatus); err != nil {
		return err
	}
	if curStatus != "review" {
		return fmt.Errorf("hanya sesi berstatus review yang bisa di-approve")
	}

	for _, it := range so.Items {
		if it.QtyCountedBase == nil {
			continue
		}
		// FOR UPDATE: selisih dihitung dari qty yang terkunci sampai commit, agar
		// semantik "setel ke qty fisik" tidak digeser movement lain di sela-sela.
		var curQty float64
		if err := tx.QueryRow(`SELECT COALESCE(qty_base, 0) FROM stock_ledger WHERE item_id = $1 AND warehouse_id = $2 FOR UPDATE`,
			it.ItemID, so.WarehouseID).Scan(&curQty); err != nil && err != sql.ErrNoRows {
			return err
		}
		diff := *it.QtyCountedBase - curQty
		if diff == 0 {
			continue
		}
		qtyDist := diff
		if it.DistRatio > 0 {
			qtyDist = diff / it.DistRatio
		}
		notes := "Stock opname " + so.OpnameNumber
		if it.Reason != "" {
			notes += " — " + it.Reason
		}
		// Selisih plus masuk sebagai batch baru dengan biaya snapshot; selisih
		// minus memotong batch FIFO seperti pengeluaran biasa.
		if err := applyMovement(tx, it.ItemID, so.WarehouseID, "adjustment", it.BaseUnit,
			so.ID, "stock_opname", so.OpnameNumber, notes, actor,
			diff, qtyDist, it.CostPerBase, ""); err != nil {
			return fmt.Errorf("%s: %w", it.ItemName, err)
		}
	}

	if _, err := tx.Exec(`
		UPDATE stock_opnames SET status = 'approved', approved_by = $2,
			approved_at = NOW() AT TIME ZONE 'UTC', updated_at = NOW() AT TIME ZONE 'UTC'
		WHERE id = $1`, id, actor); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	BroadcastSync("ppic_alert", "")
	return nil
}

// CancelStockOpname membatalkan sesi yang belum di-approve.
func CancelStockOpname(id string) error {
	res, err := database.DB.Exec(`
		UPDATE stock_opnames SET status = 'cancelled', updated_at = NOW() AT TIME ZONE 'UTC'
		WHERE id = $1 AND status IN ('counting','review')`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("sesi tidak ditemukan atau sudah selesai")
	}
	return nil
}
