package services

import (
	"fmt"
	"strings"

	"cloud-pos/database"
	"cloud-pos/models"
)

// Penghapusan aset (dijual/dimusnahkan/hibah/hilang).
//
// Penghapusan di sini adalah pelepasan akuntansi: aset ditandai status
// 'dihapus' dan keluar dari daftar aktif, tetapi barisnya TIDAK dihapus supaya
// nilai buku saat pelepasan, laba/rugi, dan histori perawatannya tetap bisa
// ditelusuri. Berbeda dengan is_deleted yang dipakai membatalkan salah input.

const dispSelectCols = `
		d.id, d.asset_id, a.name, COALESCE(a.code, ''), a.outlet_id, COALESCE(o.name, ''),
		TO_CHAR(d.disposal_date, 'YYYY-MM-DD'), d.method, d.quantity,
		d.proceeds, d.book_value_at_disposal, (d.proceeds - d.book_value_at_disposal),
		COALESCE(d.reason, ''), COALESCE(d.approved_by, ''), COALESCE(d.notes, ''), d.created_at`

const dispFromClause = `
		FROM asset_disposals d
		JOIN assets a ON a.id = d.asset_id
		LEFT JOIN outlets o ON o.id = a.outlet_id`

func scanDisposal(row rowScanner) (models.AssetDisposal, error) {
	var x models.AssetDisposal
	err := row.Scan(&x.ID, &x.AssetID, &x.AssetName, &x.AssetCode, &x.OutletID, &x.OutletName,
		&x.DisposalDate, &x.Method, &x.Quantity, &x.Proceeds, &x.BookValueAtDisposal, &x.GainLoss,
		&x.Reason, &x.ApprovedBy, &x.Notes, &x.CreatedAt)
	return x, err
}

func ListAssetDisposals(outletID, method, from, to string, outletScope []string) ([]models.AssetDisposal, error) {
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
	if method != "" {
		add("d.method = $%d", method)
	}
	if from != "" {
		add("d.disposal_date >= $%d::date", from)
	}
	if to != "" {
		add("d.disposal_date <= $%d::date", to)
	}
	scopeCond, scopeArgs := assetScopeCond("a", outletScope, idx)
	if scopeCond != "" {
		conds = append(conds, strings.TrimPrefix(scopeCond, " AND "))
		args = append(args, scopeArgs...)
	}

	rows, err := database.DB.Query(fmt.Sprintf("SELECT %s%s WHERE %s ORDER BY d.disposal_date DESC, d.created_at DESC",
		dispSelectCols, dispFromClause, strings.Join(conds, " AND ")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.AssetDisposal, 0)
	for rows.Next() {
		x, err := scanDisposal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// DisposeAsset melepas aset dan mengunci nilai bukunya saat itu, supaya
// laba/rugi pelepasan tidak ikut berubah bila parameter penyusutan diubah nanti.
func DisposeAsset(req models.AssetDisposalRequest, outletScope []string) (*models.AssetDisposal, error) {
	if req.AssetID == "" {
		return nil, fmt.Errorf("aset wajib dipilih")
	}
	asset, err := GetAsset(req.AssetID, outletScope)
	if err != nil {
		return nil, fmt.Errorf("aset tidak ditemukan")
	}
	if asset.Status == "dihapus" {
		return nil, fmt.Errorf("aset sudah dihapus sebelumnya")
	}
	if strings.TrimSpace(req.Reason) == "" {
		return nil, fmt.Errorf("alasan penghapusan wajib diisi")
	}
	if req.Method == "" {
		req.Method = "dijual"
	}

	id := NewULID()
	_, err = database.DB.Exec(`
		INSERT INTO asset_disposals (id, asset_id, disposal_date, method, quantity,
			proceeds, book_value_at_disposal, reason, approved_by, notes, created_at)
		VALUES ($1,$2, COALESCE(NULLIF($3,'')::date, CURRENT_DATE), $4,$5,$6,$7,$8,$9,$10, NOW())`,
		id, req.AssetID, req.DisposalDate, req.Method, asset.Quantity,
		req.Proceeds, asset.BookValue, req.Reason, req.ApprovedBy, req.Notes)
	if err != nil {
		return nil, err
	}

	if _, err := database.DB.Exec(
		`UPDATE assets SET status = 'dihapus', updated_at = NOW() WHERE id = $1`, req.AssetID); err != nil {
		return nil, err
	}

	row := database.DB.QueryRow(fmt.Sprintf("SELECT %s%s WHERE d.id = $1", dispSelectCols, dispFromClause), id)
	x, err := scanDisposal(row)
	if err != nil {
		return nil, err
	}
	return &x, nil
}

// RestoreDisposedAsset membatalkan penghapusan (mis. salah catat) dan menghapus
// catatan pelepasannya agar tidak ada jejak ganda di laporan.
func RestoreDisposedAsset(disposalID string, outletScope []string) error {
	var assetID string
	if err := database.DB.QueryRow(`SELECT asset_id FROM asset_disposals WHERE id = $1`, disposalID).Scan(&assetID); err != nil {
		return fmt.Errorf("catatan penghapusan tidak ditemukan")
	}
	if _, err := GetAsset(assetID, outletScope); err != nil {
		return fmt.Errorf("aset di luar akses Anda")
	}
	if _, err := database.DB.Exec(`DELETE FROM asset_disposals WHERE id = $1`, disposalID); err != nil {
		return err
	}
	_, err := database.DB.Exec(`UPDATE assets SET status = 'aktif', updated_at = NOW() WHERE id = $1`, assetID)
	return err
}
