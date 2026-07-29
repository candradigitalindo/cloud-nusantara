package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"fmt"

	"github.com/lib/pq"
)

// Bucket umur batch: expired | d3 (≤3 hari) | d7 (≤7) | d30 (≤30) | safe.
const expiryBucketExpr = `CASE
	WHEN sb.expiry_date < CURRENT_DATE THEN 'expired'
	WHEN sb.expiry_date < CURRENT_DATE + 3 THEN 'd3'
	WHEN sb.expiry_date < CURRENT_DATE + 7 THEN 'd7'
	WHEN sb.expiry_date < CURRENT_DATE + 30 THEN 'd30'
	ELSE 'safe' END`

// GetExpiryMonitor mengembalikan ringkasan bucket + daftar batch ber-expiry
// (FEFO monitor). Filter: gudang, bucket, pencarian item.
func GetExpiryMonitor(warehouseID, bucket, search string, outletIDs []string, page, limit int) (*models.ExpiryResponse, error) {
	scope, scopeArgs := ppicWarehouseScope(outletIDs)

	where := ` WHERE sb.qty_base > 0 AND sb.expiry_date IS NOT NULL AND w.is_active = true` + scope
	args := append([]interface{}{}, scopeArgs...)
	if warehouseID != "" {
		args = append(args, warehouseID)
		where += fmt.Sprintf(" AND sb.warehouse_id = $%d", len(args))
	}
	if search != "" {
		args = append(args, "%"+search+"%")
		where += fmt.Sprintf(" AND (si.name ILIKE $%d OR si.code ILIKE $%d)", len(args), len(args))
	}

	base := `
		FROM stock_batches sb
		JOIN stock_items si ON si.id = sb.item_id
		JOIN warehouses w ON w.id = sb.warehouse_id`

	resp := &models.ExpiryResponse{Summary: []models.ExpiryBucketSummary{}, Data: []models.ExpiryBatchRow{}}

	// ── Ringkasan per bucket (tanpa filter bucket, supaya kartu selalu utuh) ──
	sumRows, err := database.DB.Query(fmt.Sprintf(`
		SELECT %s AS bucket, COUNT(*), COALESCE(SUM(sb.qty_base * sb.cost_per_base), 0)
		%s%s GROUP BY 1`, expiryBucketExpr, base, where), args...)
	if err != nil {
		return nil, err
	}
	defer sumRows.Close()
	for sumRows.Next() {
		var s models.ExpiryBucketSummary
		if err := sumRows.Scan(&s.Bucket, &s.Count, &s.Value); err == nil {
			resp.Summary = append(resp.Summary, s)
		}
	}

	// Batch aktif tanpa tanggal expiry — indikator kelengkapan data.
	noExpWhere := ` WHERE sb.qty_base > 0 AND sb.expiry_date IS NULL AND w.is_active = true` + scope
	noExpArgs := append([]interface{}{}, scopeArgs...)
	if warehouseID != "" {
		noExpArgs = append(noExpArgs, warehouseID)
		noExpWhere += fmt.Sprintf(" AND sb.warehouse_id = $%d", len(noExpArgs))
	}
	database.DB.QueryRow(`SELECT COUNT(*)`+base+noExpWhere, noExpArgs...).Scan(&resp.NoExpiryCount)

	// ── Daftar batch (dengan filter bucket) ──────────────────
	if bucket != "" {
		args = append(args, bucket)
		where += fmt.Sprintf(" AND %s = $%d", expiryBucketExpr, len(args))
	}

	if err := database.DB.QueryRow(`SELECT COUNT(*)`+base+where, args...).Scan(&resp.Total); err != nil {
		return nil, err
	}

	rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT sb.id, si.id, si.code, si.name, si.category, si.base_unit,
			w.id, w.name,
			sb.qty_base, sb.cost_per_base, sb.qty_base * sb.cost_per_base,
			sb.expiry_date::text, (sb.expiry_date - CURRENT_DATE)::int, %s,
			COALESCE(sb.ref_type, ''), sb.created_at::text,
			COALESCE(sb.ppic_ack_at::text, ''), COALESCE(sb.ppic_ack_by, ''), COALESCE(sb.ppic_ack_note, '')
		%s%s
		ORDER BY sb.expiry_date ASC, si.name ASC
		LIMIT %d OFFSET %d`, expiryBucketExpr, base, where, limit, (page-1)*limit), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r models.ExpiryBatchRow
		if err := rows.Scan(&r.BatchID, &r.ItemID, &r.ItemCode, &r.ItemName, &r.Category, &r.BaseUnit,
			&r.WarehouseID, &r.WarehouseName,
			&r.QtyBase, &r.CostPerBase, &r.Value,
			&r.ExpiryDate, &r.DaysLeft, &r.Bucket,
			&r.RefType, &r.CreatedAt,
			&r.AckAt, &r.AckBy, &r.AckNote); err != nil {
			return nil, err
		}
		resp.Data = append(resp.Data, r)
	}
	return resp, nil
}

// AckExpiryBatch menandai satu batch "sudah ditindak" — alert dashboard senyap,
// batch tetap tampil di monitor dengan jejak siapa/kapan/catatan.
func AckExpiryBatch(batchID, note, actor string, outletIDs []string) error {
	// Validasi scope: batch harus berada di gudang yang boleh diakses.
	if outletIDs != nil {
		var ok bool
		database.DB.QueryRow(`
			SELECT EXISTS(
				SELECT 1 FROM stock_batches sb
				JOIN warehouses w ON w.id = sb.warehouse_id
				WHERE sb.id = $1 AND (w.outlet_id = ANY($2) OR w.type = 'central'))`,
			batchID, pq.Array(outletIDs)).Scan(&ok)
		if !ok {
			return fmt.Errorf("batch tidak ditemukan atau di luar scope")
		}
	}
	res, err := database.DB.Exec(`
		UPDATE stock_batches
		SET ppic_ack_at = NOW() AT TIME ZONE 'UTC', ppic_ack_by = $2, ppic_ack_note = $3
		WHERE id = $1`, batchID, actor, note)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("batch tidak ditemukan")
	}
	BroadcastSync("ppic_alert", "")
	return nil
}
