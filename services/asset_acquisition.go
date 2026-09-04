package services

import (
	"fmt"
	"strings"

	"cloud-pos/database"
	"cloud-pos/models"
)

// Histori perolehan aset — satu aset bisa bertambah unit di kemudian hari
// (mis. beli 10 kursi tahun lalu, tambah 5 lagi tahun ini). Total biaya di
// tabel ini menjadi dasar perhitungan penyusutan di services/asset.go.

const acqSelectCols = `
		ac.id, ac.asset_id, a.name, COALESCE(a.code, ''), a.outlet_id, COALESCE(o.name, ''),
		TO_CHAR(ac.acquisition_date, 'YYYY-MM-DD'), ac.source, ac.quantity,
		ac.unit_price, ac.total_cost, COALESCE(ac.vendor_id, ''),
		COALESCE(NULLIF(ac.vendor_name, ''), COALESCE(v.name, '')),
		COALESCE(ac.document_no, ''), COALESCE(ac.notes, ''), ac.created_at`

const acqFromClause = `
		FROM asset_acquisitions ac
		JOIN assets a ON a.id = ac.asset_id AND a.is_deleted = false
		LEFT JOIN outlets o ON o.id = a.outlet_id
		LEFT JOIN vendors v ON v.id = ac.vendor_id`

func scanAcquisition(row rowScanner) (models.AssetAcquisition, error) {
	var x models.AssetAcquisition
	err := row.Scan(&x.ID, &x.AssetID, &x.AssetName, &x.AssetCode, &x.OutletID, &x.OutletName,
		&x.AcquisitionDate, &x.Source, &x.Quantity, &x.UnitPrice, &x.TotalCost,
		&x.VendorID, &x.VendorName, &x.DocumentNo, &x.Notes, &x.CreatedAt)
	return x, err
}

// ListAssetAcquisitions mengembalikan histori perolehan lintas aset, tersaring
// outlet/aset/sumber/rentang tanggal dan selalu dibatasi cakupan outlet pemakai.
func ListAssetAcquisitions(outletID, assetID, source, from, to, search string, outletScope []string) ([]models.AssetAcquisition, error) {
	conds := []string{"1=1"}
	args := []interface{}{}
	idx := 1

	add := func(cond string, val interface{}) {
		conds = append(conds, fmt.Sprintf(cond, idx))
		args = append(args, val)
		idx++
	}
	if outletID != "" {
		add("a.outlet_id = $%d", outletID)
	}
	if assetID != "" {
		add("ac.asset_id = $%d", assetID)
	}
	if source != "" {
		add("ac.source = $%d", source)
	}
	if from != "" {
		add("ac.acquisition_date >= $%d::date", from)
	}
	if to != "" {
		add("ac.acquisition_date <= $%d::date", to)
	}
	if search != "" {
		conds = append(conds, fmt.Sprintf("(a.name ILIKE $%d OR a.code ILIKE $%d OR ac.document_no ILIKE $%d OR ac.vendor_name ILIKE $%d)", idx, idx, idx, idx))
		args = append(args, "%"+search+"%")
		idx++
	}
	scopeCond, scopeArgs := assetScopeCond("a", outletScope, idx)
	if scopeCond != "" {
		conds = append(conds, strings.TrimPrefix(scopeCond, " AND "))
		args = append(args, scopeArgs...)
	}

	rows, err := database.DB.Query(fmt.Sprintf("SELECT %s%s WHERE %s ORDER BY ac.acquisition_date DESC, ac.created_at DESC",
		acqSelectCols, acqFromClause, strings.Join(conds, " AND ")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.AssetAcquisition, 0)
	for rows.Next() {
		x, err := scanAcquisition(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// AddAssetAcquisition mencatat perolehan baru. Bila AddToQuantity aktif, jumlah
// unit di baris aset ikut bertambah — dipakai saat benar-benar ada unit baru
// masuk, bukan saat merapikan catatan perolehan lama.
func AddAssetAcquisition(req models.AssetAcquisitionRequest, outletScope []string) (*models.AssetAcquisition, error) {
	if req.AssetID == "" {
		return nil, fmt.Errorf("aset wajib dipilih")
	}
	if _, err := GetAsset(req.AssetID, outletScope); err != nil {
		return nil, fmt.Errorf("aset tidak ditemukan")
	}
	if req.Quantity <= 0 {
		req.Quantity = 1
	}
	if req.Source == "" {
		req.Source = "pembelian"
	}
	if req.UnitPrice < 0 {
		return nil, fmt.Errorf("harga satuan tidak boleh negatif")
	}

	id := NewULID()
	total := req.UnitPrice * float64(req.Quantity)
	_, err := database.DB.Exec(`
		INSERT INTO asset_acquisitions (id, asset_id, acquisition_date, source, quantity,
			unit_price, total_cost, vendor_id, vendor_name, document_no, notes, created_at)
		VALUES ($1,$2, COALESCE(NULLIF($3,'')::date, CURRENT_DATE), $4,$5,$6,$7,$8,$9,$10,$11, NOW())`,
		id, req.AssetID, req.AcquisitionDate, req.Source, req.Quantity,
		req.UnitPrice, total, nullableID(req.VendorID), req.VendorName, req.DocumentNo, req.Notes)
	if err != nil {
		return nil, err
	}

	if req.AddToQuantity {
		database.DB.Exec(`UPDATE assets SET quantity = quantity + $1, updated_at = NOW() WHERE id = $2`,
			req.Quantity, req.AssetID)
	}

	row := database.DB.QueryRow(fmt.Sprintf("SELECT %s%s WHERE ac.id = $1", acqSelectCols, acqFromClause), id)
	x, err := scanAcquisition(row)
	if err != nil {
		return nil, err
	}
	return &x, nil
}

// DeleteAssetAcquisition menghapus satu catatan perolehan. Jumlah unit aset
// TIDAK ikut dikurangi otomatis — pengurangan unit adalah keputusan terpisah
// (penghapusan aset), dan menebaknya di sini bisa membuat stok unit melenceng.
func DeleteAssetAcquisition(id string, outletScope []string) error {
	var assetID string
	if err := database.DB.QueryRow(`SELECT asset_id FROM asset_acquisitions WHERE id = $1`, id).Scan(&assetID); err != nil {
		return fmt.Errorf("catatan perolehan tidak ditemukan")
	}
	if _, err := GetAsset(assetID, outletScope); err != nil {
		return fmt.Errorf("aset di luar akses Anda")
	}
	res, err := database.DB.Exec(`DELETE FROM asset_acquisitions WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("catatan perolehan tidak ditemukan")
	}
	return nil
}
