package services

import (
	"fmt"
	"strings"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/lib/pq"
)

// assetDueSoonDays — ambang hari sebuah jadwal perawatan dianggap "segera".
const assetDueSoonDays = 7

// assetNextDueJoin mengambil next_due_date dari catatan perawatan TERBARU tiap
// aset (maintenance_date terbesar, lalu created_at). Catatan terbaru menggantikan
// jadwal lama; bila catatan terbaru tidak mengisi jadwal berikutnya, aset
// dianggap belum terjadwal — bukan mewarisi jadwal catatan yang lebih lama.
const assetNextDueJoin = `
		LEFT JOIN (
			SELECT DISTINCT ON (asset_id) asset_id, next_due_date
			FROM asset_maintenances
			ORDER BY asset_id, maintenance_date DESC, created_at DESC
		) nd ON nd.asset_id = a.id`

// assetDueStatusExpr memetakan next_due_date ke status jatuh tempo.
var assetDueStatusExpr = fmt.Sprintf(`
			CASE
				WHEN nd.next_due_date IS NULL THEN 'none'
				WHEN nd.next_due_date < CURRENT_DATE THEN 'overdue'
				WHEN nd.next_due_date <= CURRENT_DATE + %d THEN 'due_soon'
				ELSE 'scheduled'
			END`, assetDueSoonDays)

// assetDueCond menerjemahkan filter due menjadi kondisi WHERE. String kosong
// (termasuk nilai tak dikenal) berarti tanpa filter jadwal.
func assetDueCond(due string) string {
	switch due {
	case "overdue":
		return "nd.next_due_date < CURRENT_DATE"
	case "due_soon":
		return fmt.Sprintf("nd.next_due_date >= CURRENT_DATE AND nd.next_due_date <= CURRENT_DATE + %d", assetDueSoonDays)
	case "scheduled":
		return fmt.Sprintf("nd.next_due_date > CURRENT_DATE + %d", assetDueSoonDays)
	case "none":
		return "nd.next_due_date IS NULL"
	}
	return ""
}

// ── Nilai & penyusutan ──────────────────────────────────────
//
// Biaya perolehan diambil dari histori perolehan. Aset lama yang belum punya
// histori jatuh ke harga beli × jumlah supaya angkanya tidak nol.
// Penyusutan memakai metode garis lurus: (biaya − nilai residu) ÷ umur ekonomis,
// diakumulasi per bulan penuh sejak perolehan pertama dan dibatasi agar nilai
// buku tidak pernah turun di bawah nilai residu.

const assetCostExpr = `COALESCE(acq.cost, a.purchase_price * GREATEST(a.quantity, 1))`

const assetFirstDateExpr = `COALESCE(acq.first_date, a.purchase_date, a.created_at::date)`

const assetAgeExpr = `GREATEST((EXTRACT(YEAR FROM AGE(CURRENT_DATE, ` + assetFirstDateExpr + `)) * 12 +
			EXTRACT(MONTH FROM AGE(CURRENT_DATE, ` + assetFirstDateExpr + `)))::int, 0)`

const assetDepreciableExpr = `GREATEST(` + assetCostExpr + ` - COALESCE(a.residual_value, 0), 0)`

const assetMonthlyDeprecExpr = `CASE WHEN COALESCE(a.useful_life_months, 0) > 0
			THEN ` + assetDepreciableExpr + ` / a.useful_life_months ELSE 0 END`

const assetAccumDeprecExpr = `LEAST((` + assetMonthlyDeprecExpr + `) * ` + assetAgeExpr + `, ` + assetDepreciableExpr + `)`

const assetBookValueExpr = assetCostExpr + ` - (` + assetAccumDeprecExpr + `)`

// assetFromClause — sumber baris aset beserta seluruh agregat pendukungnya.
// Dipakai bersama oleh daftar dan detail agar keduanya tak pernah beda definisi.
const assetFromClause = `
		FROM assets a
		LEFT JOIN outlets o ON o.id = a.outlet_id
		LEFT JOIN vendors v ON v.id = a.vendor_id
		LEFT JOIN (
			SELECT asset_id, COUNT(*) AS cnt, TO_CHAR(MAX(maintenance_date), 'YYYY-MM-DD') AS last_date
			FROM asset_maintenances GROUP BY asset_id
		) m ON m.asset_id = a.id
		LEFT JOIN (
			SELECT asset_id, SUM(total_cost) AS cost, MIN(acquisition_date) AS first_date
			FROM asset_acquisitions GROUP BY asset_id
		) acq ON acq.asset_id = a.id` + assetNextDueJoin

// assetSelectCols — urutannya WAJIB sejalan dengan scanAsset.
var assetSelectCols = `
		a.id, a.outlet_id, COALESCE(o.name, ''), a.code, a.name, a.category,
		a.quantity, a.unit, a.condition, a.location,
		COALESCE(TO_CHAR(a.purchase_date, 'YYYY-MM-DD'), ''), a.purchase_price, a.notes,
		COALESCE(m.cnt, 0)::int,
		COALESCE(m.last_date, ''),
		COALESCE(TO_CHAR(nd.next_due_date, 'YYYY-MM-DD'), ''),
		` + assetDueStatusExpr + `,
		COALESCE((nd.next_due_date - CURRENT_DATE)::int, 0),
		COALESCE(a.serial_number, ''), COALESCE(a.vendor_id, ''), COALESCE(v.name, ''),
		COALESCE(a.useful_life_months, 0), COALESCE(a.residual_value, 0), COALESCE(a.status, 'aktif'),
		` + assetCostExpr + `,
		` + assetAccumDeprecExpr + `,
		` + assetBookValueExpr + `,
		` + assetMonthlyDeprecExpr + `,
		` + assetAgeExpr + `,
		GREATEST(COALESCE(a.useful_life_months, 0) - ` + assetAgeExpr + `, 0),
		a.created_at, a.updated_at`

// rowScanner menyatukan *sql.Row dan *sql.Rows agar scanAsset dipakai keduanya.
type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanAsset(row rowScanner) (models.Asset, error) {
	var a models.Asset
	err := row.Scan(&a.ID, &a.OutletID, &a.OutletName, &a.Code, &a.Name, &a.Category,
		&a.Quantity, &a.Unit, &a.Condition, &a.Location,
		&a.PurchaseDate, &a.PurchasePrice, &a.Notes,
		&a.MaintenanceCount, &a.LastMaintenance,
		&a.NextDueDate, &a.DueStatus, &a.DueInDays,
		&a.SerialNumber, &a.VendorID, &a.VendorName,
		&a.UsefulLifeMonths, &a.ResidualValue, &a.Status,
		&a.AcquisitionCost, &a.AccumulatedDeprec, &a.BookValue, &a.MonthlyDeprec,
		&a.AgeMonths, &a.RemainingMonths,
		&a.CreatedAt, &a.UpdatedAt)
	return a, err
}

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

// nullableID mengirim NULL alih-alih string kosong untuk kolom relasi (vendor),
// supaya JOIN tidak mencoba mencocokkan ” dan kolom tetap bersih.
func nullableID(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// ListAssets returns assets (optionally filtered by outlet/search/condition/due)
// with a maintenance count, last-maintenance date, and next-due schedule per asset.
func ListAssets(outletID, search, condition, due, status string, outletScope []string) ([]models.Asset, error) {
	conds := []string{"a.is_deleted = false"}
	args := []interface{}{}
	idx := 1

	// Aset yang sudah dihapus (dijual/dimusnahkan) tidak muncul di daftar aktif;
	// riwayatnya tetap dapat dibuka lewat halaman Penghapusan.
	switch status {
	case "":
		conds = append(conds, "COALESCE(a.status, 'aktif') <> 'dihapus'")
	case "all":
		// tanpa filter status
	default:
		conds = append(conds, fmt.Sprintf("COALESCE(a.status, 'aktif') = $%d", idx))
		args = append(args, status)
		idx++
	}

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
	if dueCond := assetDueCond(due); dueCond != "" {
		conds = append(conds, dueCond)
	}
	scopeCond, scopeArgs := assetScopeCond("a", outletScope, idx)
	if scopeCond != "" {
		conds = append(conds, strings.TrimPrefix(scopeCond, " AND "))
		args = append(args, scopeArgs...)
		idx++
	}

	// Saat menyaring jadwal, yang paling mendesak tampil dulu; tanpa filter,
	// urutan tetap seperti semula (terbaru dulu) supaya daftar tidak berubah rasa.
	order := "a.created_at DESC"
	if assetDueCond(due) != "" {
		order = "nd.next_due_date ASC NULLS LAST, a.created_at DESC"
	}

	q := fmt.Sprintf("SELECT %s%s WHERE %s ORDER BY %s",
		assetSelectCols, assetFromClause, strings.Join(conds, " AND "), order)

	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.Asset, 0)
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func GetAsset(id string, outletScope []string) (*models.Asset, error) {
	scopeCond, scopeArgs := assetScopeCond("a", outletScope, 2)
	args := append([]interface{}{id}, scopeArgs...)
	row := database.DB.QueryRow(fmt.Sprintf("SELECT %s%s WHERE a.id = $1 AND a.is_deleted = false%s",
		assetSelectCols, assetFromClause, scopeCond), args...)
	a, err := scanAsset(row)
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
			location, purchase_date, purchase_price, notes, serial_number, vendor_id,
			useful_life_months, residual_value, status, is_deleted, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,'aktif',false,NOW(),NOW())`,
		id, req.OutletID, req.Code, req.Name, req.Category, req.Quantity, req.Unit, req.Condition,
		req.Location, nullableDate(req.PurchaseDate), req.PurchasePrice, req.Notes,
		req.SerialNumber, nullableID(req.VendorID), req.UsefulLifeMonths, req.ResidualValue)
	if err != nil {
		return nil, err
	}

	// Perolehan pertama dicatat otomatis supaya setiap aset punya jejak asal-usul
	// dan penyusutan langsung punya titik mulai — tanpa ini nilai buku akan nol.
	_, err = database.DB.Exec(`
		INSERT INTO asset_acquisitions (id, asset_id, acquisition_date, source, quantity,
			unit_price, total_cost, vendor_id, vendor_name, document_no, notes, created_at)
		VALUES ($1,$2, COALESCE(NULLIF($3,'')::date, CURRENT_DATE), 'pembelian', $4,
			$5, $6, $7, '', '', 'Perolehan awal', NOW())`,
		NewULID(), id, req.PurchaseDate, req.Quantity,
		req.PurchasePrice, req.PurchasePrice*float64(req.Quantity), nullableID(req.VendorID))
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
	scopeCond, scopeArgs := assetScopeCond("", outletScope, 16)
	args := append([]interface{}{
		req.Code, req.Name, req.Category, req.Quantity, req.Unit, req.Condition,
		req.Location, nullableDate(req.PurchaseDate), req.PurchasePrice, req.Notes,
		req.SerialNumber, nullableID(req.VendorID), req.UsefulLifeMonths, req.ResidualValue, id,
	}, scopeArgs...)
	res, err := database.DB.Exec(fmt.Sprintf(`
		UPDATE assets SET code=$1, name=$2, category=$3, quantity=$4, unit=$5, condition=$6,
			location=$7, purchase_date=$8, purchase_price=$9, notes=$10, serial_number=$11,
			vendor_id=$12, useful_life_months=$13, residual_value=$14, updated_at=NOW()
		WHERE id=$15 AND is_deleted=false%s`, scopeCond), args...)
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

// AssetSummaryStats merangkum perlengkapan dalam cakupan akses (dan outlet
// terpilih bila diisi): jumlah, nilai perolehan, serta sebaran status jadwal
// perawatan. Ringkasan sengaja TIDAK ikut filter kondisi/pencarian daftar —
// kartu KPI harus tetap menunjukkan gambaran utuh outlet yang sedang dilihat.
func AssetSummaryStats(outletID string, outletScope []string) (*models.AssetSummary, error) {
	conds := []string{"a.is_deleted = false", "COALESCE(a.status, 'aktif') <> 'dihapus'"}
	args := []interface{}{}
	idx := 1

	if outletID != "" {
		conds = append(conds, fmt.Sprintf("a.outlet_id = $%d", idx))
		args = append(args, outletID)
		idx++
	}
	scopeCond, scopeArgs := assetScopeCond("a", outletScope, idx)
	if scopeCond != "" {
		conds = append(conds, strings.TrimPrefix(scopeCond, " AND "))
		args = append(args, scopeArgs...)
	}

	s := models.AssetSummary{DueSoonDays: assetDueSoonDays}
	err := database.DB.QueryRow(fmt.Sprintf(`
		SELECT COUNT(*)::int,
		       COALESCE(SUM(a.quantity), 0)::int,
		       COALESCE(SUM(%s), 0),
		       COUNT(CASE WHEN nd.next_due_date < CURRENT_DATE THEN 1 END)::int,
		       COUNT(CASE WHEN nd.next_due_date >= CURRENT_DATE AND nd.next_due_date <= CURRENT_DATE + %d THEN 1 END)::int,
		       COUNT(CASE WHEN nd.next_due_date > CURRENT_DATE + %d THEN 1 END)::int,
		       COUNT(CASE WHEN nd.next_due_date IS NULL THEN 1 END)::int,
		       COUNT(CASE WHEN a.condition <> 'baik' THEN 1 END)::int
		%s
		WHERE %s`, assetCostExpr, assetDueSoonDays, assetDueSoonDays, assetFromClause, strings.Join(conds, " AND ")), args...).
		Scan(&s.TotalAssets, &s.TotalQuantity, &s.TotalValue,
			&s.Overdue, &s.DueSoon, &s.Scheduled, &s.Unscheduled, &s.NeedsAttention)
	if err != nil {
		return nil, err
	}
	return &s, nil
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

// ListAllAssetMaintenances mengembalikan catatan perawatan LINTAS aset untuk
// halaman Perawatan — daftar per-aset saja tidak cukup untuk melihat beban
// perawatan seluruh outlet dalam satu rentang waktu.
func ListAllAssetMaintenances(outletID, assetID, mtype, from, to string, outletScope []string) ([]models.AssetMaintenance, error) {
	conds := []string{"a.is_deleted = false"}
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
		add("mm.asset_id = $%d", assetID)
	}
	if mtype != "" {
		add("mm.type = $%d", mtype)
	}
	if from != "" {
		add("mm.maintenance_date >= $%d::date", from)
	}
	if to != "" {
		add("mm.maintenance_date <= $%d::date", to)
	}
	scopeCond, scopeArgs := assetScopeCond("a", outletScope, idx)
	if scopeCond != "" {
		conds = append(conds, strings.TrimPrefix(scopeCond, " AND "))
		args = append(args, scopeArgs...)
	}

	rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT mm.id, mm.asset_id, a.name, COALESCE(a.code, ''), COALESCE(o.name, ''),
		       TO_CHAR(mm.maintenance_date, 'YYYY-MM-DD'), mm.type, mm.description,
		       mm.cost, COALESCE(mm.performed_by, ''), COALESCE(mm.condition_after, ''),
		       COALESCE(TO_CHAR(mm.next_due_date, 'YYYY-MM-DD'), ''), mm.created_at
		FROM asset_maintenances mm
		JOIN assets a ON a.id = mm.asset_id
		LEFT JOIN outlets o ON o.id = a.outlet_id
		WHERE %s
		ORDER BY mm.maintenance_date DESC, mm.created_at DESC`, strings.Join(conds, " AND ")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.AssetMaintenance, 0)
	for rows.Next() {
		var m models.AssetMaintenance
		if err := rows.Scan(&m.ID, &m.AssetID, &m.AssetName, &m.AssetCode, &m.OutletName,
			&m.MaintenanceDate, &m.Type, &m.Description, &m.Cost, &m.PerformedBy,
			&m.ConditionAfter, &m.NextDueDate, &m.CreatedAt); err != nil {
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
