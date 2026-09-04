package services

import (
	"fmt"
	"strings"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/lib/pq"
)

// Mutasi aset antar outlet.
//
// Catatan cakupan: mutasi memindahkan SELURUH baris aset ke outlet tujuan.
// Memindahkan sebagian unit (mis. 5 dari 20 kursi) memerlukan pemecahan baris
// aset dan belum didukung — pemecahan otomatis mudah membuat jejak perolehan
// dan penyusutan tidak konsisten, jadi sengaja tidak ditebak di sini.

const trfSelectCols = `
		t.id, t.asset_id, a.name, COALESCE(a.code, ''),
		t.from_outlet_id, COALESCE(fo.name, ''), t.to_outlet_id, COALESCE(too.name, ''),
		TO_CHAR(t.transfer_date, 'YYYY-MM-DD'), t.quantity,
		COALESCE(t.reason, ''), COALESCE(t.performed_by, ''), COALESCE(t.notes, ''), t.created_at`

const trfFromClause = `
		FROM asset_transfers t
		JOIN assets a ON a.id = t.asset_id AND a.is_deleted = false
		LEFT JOIN outlets fo ON fo.id = t.from_outlet_id
		LEFT JOIN outlets too ON too.id = t.to_outlet_id`

func scanTransfer(row rowScanner) (models.AssetTransfer, error) {
	var x models.AssetTransfer
	err := row.Scan(&x.ID, &x.AssetID, &x.AssetName, &x.AssetCode,
		&x.FromOutletID, &x.FromOutletName, &x.ToOutletID, &x.ToOutletName,
		&x.TransferDate, &x.Quantity, &x.Reason, &x.PerformedBy, &x.Notes, &x.CreatedAt)
	return x, err
}

// ListAssetTransfers menampilkan riwayat mutasi. Cakupan outlet dicek pada
// KEDUA sisi: pemakai berhak melihat mutasi bila outlet asal ATAU tujuan
// termasuk dalam cakupannya — kalau hanya sisi asal, aset yang sudah pindah
// keluar akan hilang dari riwayat pemakai outlet penerima.
func ListAssetTransfers(outletID, assetID, from, to string, outletScope []string) ([]models.AssetTransfer, error) {
	conds := []string{"1=1"}
	args := []interface{}{}
	idx := 1

	if outletID != "" {
		conds = append(conds, fmt.Sprintf("(t.from_outlet_id = $%d OR t.to_outlet_id = $%d)", idx, idx))
		args = append(args, outletID)
		idx++
	}
	if assetID != "" {
		conds = append(conds, fmt.Sprintf("t.asset_id = $%d", idx))
		args = append(args, assetID)
		idx++
	}
	if from != "" {
		conds = append(conds, fmt.Sprintf("t.transfer_date >= $%d::date", idx))
		args = append(args, from)
		idx++
	}
	if to != "" {
		conds = append(conds, fmt.Sprintf("t.transfer_date <= $%d::date", idx))
		args = append(args, to)
		idx++
	}
	if outletScope != nil {
		conds = append(conds, fmt.Sprintf("(t.from_outlet_id = ANY($%d::text[]) OR t.to_outlet_id = ANY($%d::text[]))", idx, idx))
		args = append(args, pq.Array(outletScope))
		idx++
	}

	rows, err := database.DB.Query(fmt.Sprintf("SELECT %s%s WHERE %s ORDER BY t.transfer_date DESC, t.created_at DESC",
		trfSelectCols, trfFromClause, strings.Join(conds, " AND ")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.AssetTransfer, 0)
	for rows.Next() {
		x, err := scanTransfer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// TransferAsset memindahkan aset ke outlet lain dan mencatat jejaknya.
func TransferAsset(req models.AssetTransferRequest, outletScope []string) (*models.AssetTransfer, error) {
	if req.AssetID == "" || req.ToOutletID == "" {
		return nil, fmt.Errorf("aset dan outlet tujuan wajib dipilih")
	}
	asset, err := GetAsset(req.AssetID, outletScope)
	if err != nil {
		return nil, fmt.Errorf("aset tidak ditemukan")
	}
	if asset.Status == "dihapus" {
		return nil, fmt.Errorf("aset sudah dihapus, tidak bisa dimutasi")
	}
	if asset.OutletID == req.ToOutletID {
		return nil, fmt.Errorf("outlet tujuan sama dengan outlet asal")
	}
	var exists int
	database.DB.QueryRow(`SELECT COUNT(*) FROM outlets WHERE id = $1`, req.ToOutletID).Scan(&exists)
	if exists == 0 {
		return nil, fmt.Errorf("outlet tujuan tidak ditemukan")
	}

	id := NewULID()
	_, err = database.DB.Exec(`
		INSERT INTO asset_transfers (id, asset_id, from_outlet_id, to_outlet_id, transfer_date,
			quantity, reason, performed_by, notes, created_at)
		VALUES ($1,$2,$3,$4, COALESCE(NULLIF($5,'')::date, CURRENT_DATE), $6,$7,$8,$9, NOW())`,
		id, req.AssetID, asset.OutletID, req.ToOutletID, req.TransferDate,
		asset.Quantity, req.Reason, req.PerformedBy, req.Notes)
	if err != nil {
		return nil, err
	}

	if _, err := database.DB.Exec(`UPDATE assets SET outlet_id = $1, updated_at = NOW() WHERE id = $2`,
		req.ToOutletID, req.AssetID); err != nil {
		return nil, err
	}

	row := database.DB.QueryRow(fmt.Sprintf("SELECT %s%s WHERE t.id = $1", trfSelectCols, trfFromClause), id)
	x, err := scanTransfer(row)
	if err != nil {
		return nil, err
	}
	return &x, nil
}
