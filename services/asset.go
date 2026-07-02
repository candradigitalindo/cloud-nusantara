package services

import (
	"fmt"
	"strings"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/lib/pq"
)

// assetScopeCond restricts assets to the scoped outlets. nil scope = all outlets.
// A sentinel like {"__none__"} naturally matches nothing.
func assetScopeCond(alias string, outletIDs []string, idx int) (string, []interface{}) {
	if outletIDs == nil {
		return "", nil
	}
	col := "outlet_id"
	if alias != "" {
		col = alias + ".outlet_id"
	}
	return fmt.Sprintf(" AND %s = ANY($%d::text[])", col, idx), []interface{}{pq.Array(outletIDs)}
}

func nullableDate(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// ListAssets returns assets (optionally filtered by outlet/search/condition) with
// a maintenance count and last-maintenance date per asset.
func ListAssets(outletID, search, condition string, outletScope []string) ([]models.Asset, error) {
	conds := []string{"a.is_deleted = false"}
	args := []interface{}{}
	idx := 1

	if outletID != "" {
		conds = append(conds, fmt.Sprintf("a.outlet_id = $%d", idx))
		args = append(args, outletID)
		idx++
	}
	if search != "" {
		conds = append(conds, fmt.Sprintf("(a.name ILIKE $%d OR a.code ILIKE $%d OR a.category ILIKE $%d)", idx, idx, idx))
		args = append(args, "%"+search+"%")
		idx++
	}
	if condition != "" {
		conds = append(conds, fmt.Sprintf("a.condition = $%d", idx))
		args = append(args, condition)
		idx++
	}
	scopeCond, scopeArgs := assetScopeCond("a", outletScope, idx)
	if scopeCond != "" {
		conds = append(conds, strings.TrimPrefix(scopeCond, " AND "))
		args = append(args, scopeArgs...)
		idx++
	}

	q := fmt.Sprintf(`
		SELECT a.id, a.outlet_id, COALESCE(o.name, ''), a.code, a.name, a.category,
		       a.quantity, a.unit, a.condition, a.location,
		       COALESCE(TO_CHAR(a.purchase_date, 'YYYY-MM-DD'), ''), a.purchase_price, a.notes,
		       COALESCE(m.cnt, 0)::int,
		       COALESCE(m.last_date, ''),
		       a.created_at, a.updated_at
		FROM assets a
		LEFT JOIN outlets o ON o.id = a.outlet_id
		LEFT JOIN (
			SELECT asset_id, COUNT(*) AS cnt, TO_CHAR(MAX(maintenance_date), 'YYYY-MM-DD') AS last_date
			FROM asset_maintenances GROUP BY asset_id
		) m ON m.asset_id = a.id
		WHERE %s
		ORDER BY a.created_at DESC`, strings.Join(conds, " AND "))

	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.Asset, 0)
	for rows.Next() {
		var a models.Asset
		if err := rows.Scan(&a.ID, &a.OutletID, &a.OutletName, &a.Code, &a.Name, &a.Category,
			&a.Quantity, &a.Unit, &a.Condition, &a.Location,
			&a.PurchaseDate, &a.PurchasePrice, &a.Notes,
			&a.MaintenanceCount, &a.LastMaintenance, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func GetAsset(id string, outletScope []string) (*models.Asset, error) {
	scopeCond, scopeArgs := assetScopeCond("a", outletScope, 2)
	args := append([]interface{}{id}, scopeArgs...)
	var a models.Asset
	err := database.DB.QueryRow(fmt.Sprintf(`
		SELECT a.id, a.outlet_id, COALESCE(o.name, ''), a.code, a.name, a.category,
		       a.quantity, a.unit, a.condition, a.location,
		       COALESCE(TO_CHAR(a.purchase_date, 'YYYY-MM-DD'), ''), a.purchase_price, a.notes,
		       (SELECT COUNT(*) FROM asset_maintenances m WHERE m.asset_id = a.id)::int,
		       COALESCE((SELECT TO_CHAR(MAX(m.maintenance_date), 'YYYY-MM-DD') FROM asset_maintenances m WHERE m.asset_id = a.id), ''),
		       a.created_at, a.updated_at
		FROM assets a
		LEFT JOIN outlets o ON o.id = a.outlet_id
		WHERE a.id = $1 AND a.is_deleted = false%s`, scopeCond), args...).
		Scan(&a.ID, &a.OutletID, &a.OutletName, &a.Code, &a.Name, &a.Category,
			&a.Quantity, &a.Unit, &a.Condition, &a.Location,
			&a.PurchaseDate, &a.PurchasePrice, &a.Notes,
			&a.MaintenanceCount, &a.LastMaintenance, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func CreateAsset(req models.AssetRequest) (*models.Asset, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("nama aset wajib diisi")
	}
	if req.OutletID == "" {
		return nil, fmt.Errorf("outlet wajib dipilih")
	}
	if req.Quantity <= 0 {
		req.Quantity = 1
	}
	if req.Unit == "" {
		req.Unit = "unit"
	}
	if req.Condition == "" {
		req.Condition = "baik"
	}
	id := NewULID()
	_, err := database.DB.Exec(`
		INSERT INTO assets (id, outlet_id, code, name, category, quantity, unit, condition,
			location, purchase_date, purchase_price, notes, is_deleted, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,false,NOW(),NOW())`,
		id, req.OutletID, req.Code, req.Name, req.Category, req.Quantity, req.Unit, req.Condition,
		req.Location, nullableDate(req.PurchaseDate), req.PurchasePrice, req.Notes)
	if err != nil {
		return nil, err
	}
	return GetAsset(id, nil)
}

func UpdateAsset(id string, req models.AssetRequest, outletScope []string) (*models.Asset, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("nama aset wajib diisi")
	}
	if req.Quantity <= 0 {
		req.Quantity = 1
	}
	if req.Unit == "" {
		req.Unit = "unit"
	}
	scopeCond, scopeArgs := assetScopeCond("", outletScope, 12)
	args := append([]interface{}{
		req.Code, req.Name, req.Category, req.Quantity, req.Unit, req.Condition,
		req.Location, nullableDate(req.PurchaseDate), req.PurchasePrice, req.Notes, id,
	}, scopeArgs...)
	res, err := database.DB.Exec(fmt.Sprintf(`
		UPDATE assets SET code=$1, name=$2, category=$3, quantity=$4, unit=$5, condition=$6,
			location=$7, purchase_date=$8, purchase_price=$9, notes=$10, updated_at=NOW()
		WHERE id=$11 AND is_deleted=false%s`, scopeCond), args...)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, fmt.Errorf("aset tidak ditemukan")
	}
	return GetAsset(id, outletScope)
}

func DeleteAsset(id string, outletScope []string) error {
	scopeCond, scopeArgs := assetScopeCond("", outletScope, 2)
	args := append([]interface{}{id}, scopeArgs...)
	res, err := database.DB.Exec(fmt.Sprintf(
		`UPDATE assets SET is_deleted=true, updated_at=NOW() WHERE id=$1 AND is_deleted=false%s`, scopeCond), args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("aset tidak ditemukan")
	}
	return nil
}

// ── Maintenance history ─────────────────────────────────────

func ListAssetMaintenances(assetID string) ([]models.AssetMaintenance, error) {
	rows, err := database.DB.Query(`
		SELECT id, asset_id, TO_CHAR(maintenance_date, 'YYYY-MM-DD'), type, description,
		       cost, performed_by, condition_after,
		       COALESCE(TO_CHAR(next_due_date, 'YYYY-MM-DD'), ''), created_at
		FROM asset_maintenances WHERE asset_id = $1
		ORDER BY maintenance_date DESC, created_at DESC`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.AssetMaintenance, 0)
	for rows.Next() {
		var m models.AssetMaintenance
		if err := rows.Scan(&m.ID, &m.AssetID, &m.MaintenanceDate, &m.Type, &m.Description,
			&m.Cost, &m.PerformedBy, &m.ConditionAfter, &m.NextDueDate, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// AddAssetMaintenance inserts a maintenance record. If condition_after is set,
// the asset's current condition is updated to reflect it.
func AddAssetMaintenance(assetID string, req models.AssetMaintenanceRequest, outletScope []string) (*models.AssetMaintenance, error) {
	if strings.TrimSpace(req.Description) == "" {
		return nil, fmt.Errorf("deskripsi perawatan wajib diisi")
	}
	// Ensure the asset is within scope before adding history.
	if _, err := GetAsset(assetID, outletScope); err != nil {
		return nil, fmt.Errorf("aset tidak ditemukan")
	}
	if req.Type == "" {
		req.Type = "rutin"
	}
	id := NewULID()
	_, err := database.DB.Exec(`
		INSERT INTO asset_maintenances (id, asset_id, maintenance_date, type, description,
			cost, performed_by, condition_after, next_due_date, created_at)
		VALUES ($1,$2, COALESCE(NULLIF($3,'')::date, CURRENT_DATE), $4,$5,$6,$7,$8, NULLIF($9,'')::date, NOW())`,
		id, assetID, req.MaintenanceDate, req.Type, req.Description,
		req.Cost, req.PerformedBy, req.ConditionAfter, req.NextDueDate)
	if err != nil {
		return nil, err
	}
	// Reflect post-maintenance condition on the asset (lenient).
	if strings.TrimSpace(req.ConditionAfter) != "" {
		database.DB.Exec(`UPDATE assets SET condition=$1, updated_at=NOW() WHERE id=$2`, req.ConditionAfter, assetID)
	}
	var m models.AssetMaintenance
	err = database.DB.QueryRow(`
		SELECT id, asset_id, TO_CHAR(maintenance_date,'YYYY-MM-DD'), type, description, cost,
		       performed_by, condition_after, COALESCE(TO_CHAR(next_due_date,'YYYY-MM-DD'),''), created_at
		FROM asset_maintenances WHERE id=$1`, id).
		Scan(&m.ID, &m.AssetID, &m.MaintenanceDate, &m.Type, &m.Description, &m.Cost,
			&m.PerformedBy, &m.ConditionAfter, &m.NextDueDate, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func DeleteAssetMaintenance(assetID, maintenanceID string, outletScope []string) error {
	if _, err := GetAsset(assetID, outletScope); err != nil {
		return fmt.Errorf("aset tidak ditemukan")
	}
	res, err := database.DB.Exec(`DELETE FROM asset_maintenances WHERE id=$1 AND asset_id=$2`, maintenanceID, assetID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("catatan perawatan tidak ditemukan")
	}
	return nil
}
