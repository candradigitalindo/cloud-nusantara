package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func generatePlanNumber() string {
	now := time.Now()
	prefix := "MPS-" + now.Format("20060102") + "-"
	var seq int
	database.DB.QueryRow(`SELECT COUNT(*)+1 FROM production_plans WHERE plan_number LIKE $1`, prefix+"%").Scan(&seq)
	return fmt.Sprintf("%s%03d", prefix, seq)
}

func generateWoNumber() string {
	now := time.Now()
	prefix := "WO-" + now.Format("20060102") + "-"
	var seq int
	database.DB.QueryRow(`SELECT COUNT(*)+1 FROM work_orders WHERE wo_number LIKE $1`, prefix+"%").Scan(&seq)
	return fmt.Sprintf("%s%03d", prefix, seq)
}

// ── Rencana Produksi (MPS) ────────────────────────────────────

// CreateProductionPlan membuat rencana produksi (draft) untuk satu gudang.
// Hanya item ber-resep internal (stock_item_recipes) yang boleh direncanakan.
func CreateProductionPlan(req models.ProductionPlanRequest, actor string) (*models.ProductionPlan, error) {
	if req.WarehouseID == "" {
		return nil, fmt.Errorf("warehouse_id wajib diisi")
	}
	if req.PlanDate == "" {
		req.PlanDate = time.Now().In(GetTimezoneLocation()).Format("2006-01-02")
	}
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("minimal 1 item produksi")
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	id := NewULID()
	if _, err := tx.Exec(`
		INSERT INTO production_plans (id, plan_number, warehouse_id, plan_date, status, notes, created_by)
		VALUES ($1, $2, $3, $4::date, 'draft', $5, $6)`,
		id, generatePlanNumber(), req.WarehouseID, req.PlanDate, req.Notes, actor); err != nil {
		return nil, err
	}
	for i, it := range req.Items {
		if it.ItemID == "" || it.QtyPlannedBase <= 0 {
			return nil, fmt.Errorf("baris %d: item & qty rencana wajib diisi (> 0)", i+1)
		}
		var hasRecipe bool
		tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM stock_item_recipes WHERE parent_item_id = $1)`, it.ItemID).Scan(&hasRecipe)
		if !hasRecipe {
			var name string
			tx.QueryRow(`SELECT name FROM stock_items WHERE id = $1`, it.ItemID).Scan(&name)
			return nil, fmt.Errorf("%s belum punya resep internal — definisikan dulu di Item Stok → Resep", name)
		}
		if _, err := tx.Exec(`
			INSERT INTO production_plan_items (id, plan_id, item_id, qty_planned_base, notes)
			VALUES ($1, $2, $3, $4, $5)`, NewULID(), id, it.ItemID, it.QtyPlannedBase, it.Notes); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetProductionPlan(id)
}

func ListProductionPlans(warehouseID, status string, outletIDs []string, page, limit int) ([]models.ProductionPlan, int, error) {
	scope, scopeArgs := ppicWarehouseScope(outletIDs)
	where := ` WHERE w.is_active = true` + scope
	args := append([]interface{}{}, scopeArgs...)
	if warehouseID != "" {
		args = append(args, warehouseID)
		where += fmt.Sprintf(" AND pp.warehouse_id = $%d", len(args))
	}
	if status != "" {
		args = append(args, status)
		where += fmt.Sprintf(" AND pp.status = $%d", len(args))
	}
	base := ` FROM production_plans pp JOIN warehouses w ON w.id = pp.warehouse_id`

	var total int
	if err := database.DB.QueryRow(`SELECT COUNT(*)`+base+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT pp.id, pp.plan_number, pp.warehouse_id, w.name, pp.plan_date::text, pp.status,
			COALESCE(pp.notes, ''), COALESCE(pp.created_by, ''), COALESCE(pp.approved_by, ''),
			pp.approved_at::text, pp.created_at::text, pp.updated_at::text,
			(SELECT COUNT(*) FROM production_plan_items i WHERE i.plan_id = pp.id),
			(SELECT COUNT(*) FROM work_orders wo WHERE wo.plan_id = pp.id AND wo.status <> 'cancelled'),
			(SELECT COUNT(*) FROM work_orders wo WHERE wo.plan_id = pp.id AND wo.status = 'done')
		%s%s ORDER BY pp.created_at DESC LIMIT %d OFFSET %d`, base, where, limit, (page-1)*limit), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list := []models.ProductionPlan{}
	for rows.Next() {
		var p models.ProductionPlan
		if err := rows.Scan(&p.ID, &p.PlanNumber, &p.WarehouseID, &p.WarehouseName, &p.PlanDate, &p.Status,
			&p.Notes, &p.CreatedBy, &p.ApprovedBy, &p.ApprovedAt, &p.CreatedAt, &p.UpdatedAt,
			&p.ItemCount, &p.WoTotal, &p.WoDone); err != nil {
			return nil, 0, err
		}
		list = append(list, p)
	}
	return list, total, nil
}

func GetProductionPlan(id string) (*models.ProductionPlan, error) {
	var p models.ProductionPlan
	err := database.DB.QueryRow(`
		SELECT pp.id, pp.plan_number, pp.warehouse_id, w.name, pp.plan_date::text, pp.status,
			COALESCE(pp.notes, ''), COALESCE(pp.created_by, ''), COALESCE(pp.approved_by, ''),
			pp.approved_at::text, pp.created_at::text, pp.updated_at::text
		FROM production_plans pp JOIN warehouses w ON w.id = pp.warehouse_id
		WHERE pp.id = $1`, id).Scan(
		&p.ID, &p.PlanNumber, &p.WarehouseID, &p.WarehouseName, &p.PlanDate, &p.Status,
		&p.Notes, &p.CreatedBy, &p.ApprovedBy, &p.ApprovedAt, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("rencana produksi tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}

	rows, err := database.DB.Query(`
		SELECT i.id, i.item_id, si.code, si.name, si.base_unit, i.qty_planned_base, COALESCE(i.notes, ''),
			COALESCE(sl.qty_base, 0), COALESCE(par.par_level, 0),
			COALESCE(wo.id, ''), COALESCE(wo.wo_number, ''), COALESCE(wo.status, '')
		FROM production_plan_items i
		JOIN stock_items si ON si.id = i.item_id
		LEFT JOIN stock_ledger sl ON sl.item_id = i.item_id AND sl.warehouse_id = $2
		LEFT JOIN item_planning_params par ON par.item_id = i.item_id AND par.warehouse_id = $2
		LEFT JOIN work_orders wo ON wo.plan_id = $1 AND wo.item_id = i.item_id AND wo.status <> 'cancelled'
		WHERE i.plan_id = $1 ORDER BY si.name`, id, p.WarehouseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	p.Items = []models.ProductionPlanItem{}
	for rows.Next() {
		var it models.ProductionPlanItem
		if err := rows.Scan(&it.ID, &it.ItemID, &it.ItemCode, &it.ItemName, &it.BaseUnit,
			&it.QtyPlannedBase, &it.Notes, &it.OnHand, &it.ParLevel,
			&it.WoID, &it.WoNumber, &it.WoStatus); err != nil {
			return nil, err
		}
		it.WoID = strings.TrimSpace(it.WoID)
		p.ItemCount++
		if it.WoID != "" {
			p.WoTotal++
			if it.WoStatus == "done" {
				p.WoDone++
			}
		}
		p.Items = append(p.Items, it)
	}
	return &p, nil
}

func ProductionPlanInScope(id string, outletIDs []string) bool {
	var whID string
	if err := database.DB.QueryRow(`SELECT warehouse_id FROM production_plans WHERE id = $1`, id).Scan(&whID); err != nil {
		return false
	}
	return WarehouseMutableInScope(strings.TrimSpace(whID), outletIDs)
}

// ApproveProductionPlan: draft → approved.
func ApproveProductionPlan(id, actor string) error {
	res, err := database.DB.Exec(`
		UPDATE production_plans SET status = 'approved', approved_by = $2,
			approved_at = NOW() AT TIME ZONE 'UTC', updated_at = NOW() AT TIME ZONE 'UTC'
		WHERE id = $1 AND status = 'draft'`, id, actor)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("hanya rencana berstatus draft yang bisa di-approve")
	}
	return nil
}

// ReleaseProductionPlan: approved → released; tiap baris item menjadi Work Order.
func ReleaseProductionPlan(id, actor string) (*models.ProductionPlan, error) {
	plan, err := GetProductionPlan(id)
	if err != nil {
		return nil, err
	}
	if plan.Status != "approved" {
		return nil, fmt.Errorf("hanya rencana berstatus approved yang bisa di-release")
	}
	for _, it := range plan.Items {
		if it.WoID != "" {
			continue
		}
		if _, err := createWorkOrder(plan.WarehouseID, it.ItemID, it.QtyPlannedBase, id,
			fmt.Sprintf("Dari rencana %s", plan.PlanNumber), actor); err != nil {
			return nil, fmt.Errorf("%s: %w", it.ItemName, err)
		}
	}
	if _, err := database.DB.Exec(`UPDATE production_plans SET status = 'released', updated_at = NOW() AT TIME ZONE 'UTC' WHERE id = $1`, id); err != nil {
		return nil, err
	}
	return GetProductionPlan(id)
}

// CancelProductionPlan membatalkan rencana yang belum release.
func CancelProductionPlan(id string) error {
	res, err := database.DB.Exec(`
		UPDATE production_plans SET status = 'cancelled', updated_at = NOW() AT TIME ZONE 'UTC'
		WHERE id = $1 AND status IN ('draft','approved')`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("rencana tidak ditemukan atau sudah release")
	}
	return nil
}

// ── Work Order ────────────────────────────────────────────────

// createWorkOrder membuat WO status planned + snapshot kebutuhan bahan dari
// resep internal (qty_plan = resep × qty rencana, biaya = avg cost saat ini).
func createWorkOrder(warehouseID, itemID string, qtyPlanned float64, planID, notes, actor string) (string, error) {
	if qtyPlanned <= 0 {
		return "", fmt.Errorf("qty rencana harus > 0")
	}
	recipes, err := GetStockItemRecipes(itemID)
	if err != nil || len(recipes) == 0 {
		return "", fmt.Errorf("item belum punya resep internal")
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	id := NewULID()
	var planArg interface{}
	if planID != "" {
		planArg = planID
	}
	if _, err := tx.Exec(`
		INSERT INTO work_orders (id, wo_number, plan_id, warehouse_id, item_id, qty_planned_base, status, notes, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, 'planned', $7, $8)`,
		id, generateWoNumber(), planArg, warehouseID, itemID, qtyPlanned, notes, actor); err != nil {
		return "", err
	}
	for _, r := range recipes {
		var avgCost float64
		tx.QueryRow(`SELECT COALESCE(avg_cost, 0) FROM stock_ledger WHERE item_id = $1 AND warehouse_id = $2`,
			r.ChildItemID, warehouseID).Scan(&avgCost)
		if _, err := tx.Exec(`
			INSERT INTO work_order_materials (id, wo_id, item_id, qty_plan_base, cost_per_base)
			VALUES ($1, $2, $3, $4, $5)`,
			NewULID(), id, r.ChildItemID, r.QtyBase*qtyPlanned, avgCost); err != nil {
			return "", err
		}
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return id, nil
}

func CreateWorkOrder(req models.WorkOrderCreateRequest, actor string) (*models.WorkOrder, error) {
	id, err := createWorkOrder(req.WarehouseID, req.ItemID, req.QtyPlannedBase, "", req.Notes, actor)
	if err != nil {
		return nil, err
	}
	return GetWorkOrder(id)
}

func ListWorkOrders(warehouseID, status string, outletIDs []string, page, limit int) ([]models.WorkOrder, int, error) {
	scope, scopeArgs := ppicWarehouseScope(outletIDs)
	where := ` WHERE w.is_active = true` + scope
	args := append([]interface{}{}, scopeArgs...)
	if warehouseID != "" {
		args = append(args, warehouseID)
		where += fmt.Sprintf(" AND wo.warehouse_id = $%d", len(args))
	}
	if status != "" {
		args = append(args, status)
		where += fmt.Sprintf(" AND wo.status = $%d", len(args))
	}
	base := ` FROM work_orders wo JOIN warehouses w ON w.id = wo.warehouse_id JOIN stock_items si ON si.id = wo.item_id LEFT JOIN production_plans pp ON pp.id = wo.plan_id`

	var total int
	if err := database.DB.QueryRow(`SELECT COUNT(*)`+base+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT wo.id, wo.wo_number, COALESCE(wo.plan_id, ''), COALESCE(pp.plan_number, ''),
			wo.warehouse_id, w.name, wo.item_id, si.code, si.name, si.base_unit, si.shelf_life_days,
			wo.qty_planned_base, wo.qty_actual_base, COALESCE(wo.yield_pct, 0), wo.status,
			wo.started_at::text, wo.finished_at::text, COALESCE(wo.executed_by, ''),
			COALESCE(wo.notes, ''), COALESCE(wo.created_by, ''), wo.created_at::text,
			COALESCE(wo.cost_total, 0), COALESCE(wo.cost_per_unit, 0), COALESCE(wo.expiry_date::text, '')
		%s%s ORDER BY wo.created_at DESC LIMIT %d OFFSET %d`, base, where, limit, (page-1)*limit), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list := []models.WorkOrder{}
	for rows.Next() {
		var wo models.WorkOrder
		if err := rows.Scan(&wo.ID, &wo.WoNumber, &wo.PlanID, &wo.PlanNumber,
			&wo.WarehouseID, &wo.WarehouseName, &wo.ItemID, &wo.ItemCode, &wo.ItemName, &wo.BaseUnit, &wo.ShelfLifeDays,
			&wo.QtyPlannedBase, &wo.QtyActualBase, &wo.YieldPct, &wo.Status,
			&wo.StartedAt, &wo.FinishedAt, &wo.ExecutedBy,
			&wo.Notes, &wo.CreatedBy, &wo.CreatedAt,
			&wo.CostTotal, &wo.CostPerUnit, &wo.ExpiryDate); err != nil {
			return nil, 0, err
		}
		list = append(list, wo)
	}
	return list, total, nil
}

func GetWorkOrder(id string) (*models.WorkOrder, error) {
	var wo models.WorkOrder
	err := database.DB.QueryRow(`
		SELECT wo.id, wo.wo_number, COALESCE(wo.plan_id, ''), COALESCE(pp.plan_number, ''),
			wo.warehouse_id, w.name, wo.item_id, si.code, si.name, si.base_unit, si.shelf_life_days,
			wo.qty_planned_base, wo.qty_actual_base, COALESCE(wo.yield_pct, 0), wo.status,
			wo.started_at::text, wo.finished_at::text, COALESCE(wo.executed_by, ''),
			COALESCE(wo.notes, ''), COALESCE(wo.created_by, ''), wo.created_at::text,
			COALESCE(wo.cost_total, 0), COALESCE(wo.cost_per_unit, 0), COALESCE(wo.expiry_date::text, '')
		FROM work_orders wo
		JOIN warehouses w ON w.id = wo.warehouse_id
		JOIN stock_items si ON si.id = wo.item_id
		LEFT JOIN production_plans pp ON pp.id = wo.plan_id
		WHERE wo.id = $1`, id).Scan(
		&wo.ID, &wo.WoNumber, &wo.PlanID, &wo.PlanNumber,
		&wo.WarehouseID, &wo.WarehouseName, &wo.ItemID, &wo.ItemCode, &wo.ItemName, &wo.BaseUnit, &wo.ShelfLifeDays,
		&wo.QtyPlannedBase, &wo.QtyActualBase, &wo.YieldPct, &wo.Status,
		&wo.StartedAt, &wo.FinishedAt, &wo.ExecutedBy,
		&wo.Notes, &wo.CreatedBy, &wo.CreatedAt,
		&wo.CostTotal, &wo.CostPerUnit, &wo.ExpiryDate)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("work order tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}
	wo.PlanID = strings.TrimSpace(wo.PlanID)

	rows, err := database.DB.Query(`
		SELECT m.id, m.item_id, si.code, si.name, si.base_unit,
			m.qty_plan_base, m.qty_actual_base, m.cost_per_base, COALESCE(sl.qty_base, 0)
		FROM work_order_materials m
		JOIN stock_items si ON si.id = m.item_id
		LEFT JOIN stock_ledger sl ON sl.item_id = m.item_id AND sl.warehouse_id = $2
		WHERE m.wo_id = $1 ORDER BY si.name`, id, wo.WarehouseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	wo.Materials = []models.WorkOrderMaterial{}
	for rows.Next() {
		var m models.WorkOrderMaterial
		if err := rows.Scan(&m.ID, &m.ItemID, &m.ItemCode, &m.ItemName, &m.BaseUnit,
			&m.QtyPlanBase, &m.QtyActualBase, &m.CostPerBase, &m.OnHand); err != nil {
			return nil, err
		}
		if m.QtyActualBase != nil && m.QtyPlanBase > 0 {
			m.VariancePct = (*m.QtyActualBase - m.QtyPlanBase) / m.QtyPlanBase * 100
		}
		wo.Materials = append(wo.Materials, m)
	}
	return &wo, nil
}

func WorkOrderInScope(id string, outletIDs []string) bool {
	var whID string
	if err := database.DB.QueryRow(`SELECT warehouse_id FROM work_orders WHERE id = $1`, id).Scan(&whID); err != nil {
		return false
	}
	return WarehouseMutableInScope(strings.TrimSpace(whID), outletIDs)
}

// StartWorkOrder: planned → in_progress.
func StartWorkOrder(id, actor string) error {
	res, err := database.DB.Exec(`
		UPDATE work_orders SET status = 'in_progress', started_at = NOW() AT TIME ZONE 'UTC', executed_by = $2
		WHERE id = $1 AND status = 'planned'`, id, actor)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("hanya WO berstatus planned yang bisa dimulai")
	}
	return nil
}

// FinishWorkOrder memposting produksi: bahan aktual keluar (FIFO, movement
// production_out ref work_order), hasil masuk sebagai batch baru ber-HPP
// aktual dan ber-expiry (shelf_life_days item). Yield = aktual / rencana.
func FinishWorkOrder(id string, req models.WorkOrderFinishRequest, actor string) (*models.WorkOrder, error) {
	wo, err := GetWorkOrder(id)
	if err != nil {
		return nil, err
	}
	if wo.Status != "in_progress" && wo.Status != "planned" {
		return nil, fmt.Errorf("WO berstatus %s tidak bisa diselesaikan", wo.Status)
	}
	if req.QtyActualBase <= 0 {
		return nil, fmt.Errorf("qty hasil aktual harus > 0")
	}

	// Bahan aktual: dari request, atau default resep × qty aktual.
	actuals := map[string]float64{}
	if len(req.Materials) > 0 {
		for _, m := range req.Materials {
			if m.QtyActualBase < 0 {
				return nil, fmt.Errorf("qty bahan tidak boleh negatif")
			}
			actuals[m.ItemID] = m.QtyActualBase
		}
	} else {
		ratio := req.QtyActualBase / wo.QtyPlannedBase
		for _, m := range wo.Materials {
			actuals[m.ItemID] = round4(m.QtyPlanBase * ratio)
		}
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Kunci WO & validasi ulang status di dalam transaksi: cek di atas tanpa lock,
	// dua finish bersamaan bisa sama-sama lolos dan memposting stok dua kali.
	var curStatus string
	if err := tx.QueryRow(`SELECT status FROM work_orders WHERE id = $1 FOR UPDATE`, id).Scan(&curStatus); err != nil {
		return nil, err
	}
	if curStatus != "in_progress" && curStatus != "planned" {
		return nil, fmt.Errorf("WO berstatus %s tidak bisa diselesaikan", curStatus)
	}

	var totalCost float64
	for _, m := range wo.Materials {
		qty, ok := actuals[m.ItemID]
		if !ok {
			qty = 0 // bahan tidak dipakai
		}
		if qty > 0 {
			if err := applyMovement(tx, m.ItemID, wo.WarehouseID, "production_out", m.BaseUnit,
				wo.ID, "work_order", wo.WoNumber, "Bahan produksi "+wo.ItemName, actor, -qty, -qty, 0, ""); err != nil {
				return nil, fmt.Errorf("%s: %w", m.ItemName, err)
			}
			// Biaya aktual hasil pemotongan FIFO (pola ProduceStockItem). Gagal baca
			// harus jadi error — bila ditelan, biaya bahan ini hilang dari HPP.
			var cost float64
			if err := tx.QueryRow(`
				SELECT cost_per_base FROM stock_movements
				WHERE item_id = $1 AND warehouse_id = $2 AND ref_type = 'work_order' AND ref_id = $3
				  AND movement_type = 'production_out'
				ORDER BY created_at DESC, id DESC LIMIT 1`, m.ItemID, wo.WarehouseID, wo.ID).Scan(&cost); err != nil {
				return nil, fmt.Errorf("gagal baca biaya aktual %s: %w", m.ItemName, err)
			}
			totalCost += cost * qty
			if _, err := tx.Exec(`UPDATE work_order_materials SET cost_per_base = $3, qty_actual_base = $4 WHERE wo_id = $1 AND item_id = $2`,
				wo.ID, m.ItemID, cost, qty); err != nil {
				return nil, err
			}
		} else {
			if _, err := tx.Exec(`UPDATE work_order_materials SET qty_actual_base = 0 WHERE wo_id = $1 AND item_id = $2`, wo.ID, m.ItemID); err != nil {
				return nil, err
			}
		}
	}

	costPerUnit := totalCost / req.QtyActualBase
	expiry := ""
	if wo.ShelfLifeDays > 0 {
		expiry = time.Now().In(GetTimezoneLocation()).AddDate(0, 0, wo.ShelfLifeDays).Format("2006-01-02")
	}
	if err := applyMovement(tx, wo.ItemID, wo.WarehouseID, "production_in", wo.BaseUnit,
		wo.ID, "work_order", wo.WoNumber, "Hasil produksi "+wo.WoNumber, actor,
		req.QtyActualBase, req.QtyActualBase, costPerUnit, expiry); err != nil {
		return nil, err
	}

	yield := req.QtyActualBase / wo.QtyPlannedBase * 100
	var expArg interface{}
	if expiry != "" {
		expArg = expiry
	}
	notes := wo.Notes
	if req.Notes != "" {
		if notes != "" {
			notes += " — "
		}
		notes += req.Notes
	}
	if _, err := tx.Exec(`
		UPDATE work_orders SET status = 'done', qty_actual_base = $2, yield_pct = $3,
			cost_total = $4, cost_per_unit = $5, expiry_date = $6, notes = $7,
			finished_at = NOW() AT TIME ZONE 'UTC', executed_by = $8
		WHERE id = $1`, id, req.QtyActualBase, round4(yield), totalCost, costPerUnit, expArg, notes, actor); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Tutup rencana otomatis bila semua WO-nya selesai/batal.
	if wo.PlanID != "" {
		database.DB.Exec(`
			UPDATE production_plans SET status = 'closed', updated_at = NOW() AT TIME ZONE 'UTC'
			WHERE id = $1 AND status = 'released'
			AND NOT EXISTS (SELECT 1 FROM work_orders x WHERE x.plan_id = $1 AND x.status IN ('planned','in_progress'))`,
			wo.PlanID)
	}
	BroadcastSync("ppic_alert", "")
	return GetWorkOrder(id)
}

// CancelWorkOrder membatalkan WO yang belum selesai.
func CancelWorkOrder(id string) error {
	res, err := database.DB.Exec(`
		UPDATE work_orders SET status = 'cancelled' WHERE id = $1 AND status IN ('planned','in_progress')`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("WO tidak ditemukan atau sudah selesai")
	}
	return nil
}

// ListProducibleItems — item ber-resep internal (kandidat rencana produksi/WO).
func ListProducibleItems(warehouseID string) ([]models.PlanningParamRow, error) {
	rows, err := database.DB.Query(`
		SELECT DISTINCT si.id, si.code, si.name, si.category, si.base_unit,
			COALESCE(sl.qty_base, 0), COALESCE(par.par_level, 0)
		FROM stock_item_recipes sir
		JOIN stock_items si ON si.id = sir.parent_item_id AND si.is_active = true
		LEFT JOIN stock_ledger sl ON sl.item_id = si.id AND sl.warehouse_id = $1
		LEFT JOIN item_planning_params par ON par.item_id = si.id AND par.warehouse_id = $1
		ORDER BY si.name`, warehouseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []models.PlanningParamRow{}
	for rows.Next() {
		var r models.PlanningParamRow
		if err := rows.Scan(&r.ItemID, &r.ItemCode, &r.ItemName, &r.Category, &r.BaseUnit, &r.QtyBase, &r.ParLevel); err != nil {
			return nil, err
		}
		r.WarehouseID = warehouseID
		list = append(list, r)
	}
	return list, nil
}
