package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"sort"
	"strings"

	"github.com/lib/pq"
)

// GetPpicHppReport — kartu HPP per menu (costing card), padanan sheet
// "Food/Beverage Cost" pada laporan manual PPIC: resep produk di-explode
// (termasuk komponen WIP), dihargai dengan avg cost gudang, lalu dibandingkan
// dengan harga jual → HPP %, margin, dan flag di atas ambang ideal.
//
// Biaya komponen WIP: avg_cost item (sudah ter-update HPP aktual tiap produksi
// WO); bila belum pernah diproduksi (avg 0), dihitung teoretis dari resep
// internalnya (rekursif, maksimal 3 tingkat).
func GetPpicHppReport(outletID string, idealPct float64, outletScope []string) (*models.PpicHppReport, error) {
	if idealPct <= 0 || idealPct > 100 {
		idealPct = 35
	}
	rep := &models.PpicHppReport{IdealPct: idealPct, Rows: []models.PpicHppRow{}}

	// ── 1. Daftar produk dalam scope ─────────────────────────
	q := `
		SELECT p.id, p.outlet_id, o.name, p.name, COALESCE(p.category_name, ''), COALESCE(p.price, 0),
			COALESCE(p.stock_type, ''), COALESCE(p.linked_stock_item_id, ''), COALESCE(p.recipe_master_id, '')
		FROM cloud_products p
		JOIN outlets o ON o.id = p.outlet_id
		WHERE COALESCE(p.is_deleted, false) = false AND o.is_active = true`
	args := []interface{}{}
	if outletID != "" {
		args = append(args, outletID)
		q += ` AND p.outlet_id = $1`
	} else if outletScope != nil {
		args = append(args, pq.Array(outletScope))
		q += ` AND p.outlet_id = ANY($1)`
	}
	q += ` ORDER BY o.name, p.name`

	type prod struct {
		id, outletID, outletName, name, category string
		price                                    float64
		stockType, linked, master                string
	}
	var prods []prod
	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var p prod
		if rows.Scan(&p.id, &p.outletID, &p.outletName, &p.name, &p.category, &p.price,
			&p.stockType, &p.linked, &p.master) == nil {
			p.outletID = strings.TrimSpace(p.outletID)
			prods = append(prods, p)
		}
	}
	rows.Close()
	if len(prods) == 0 {
		return rep, nil
	}

	// ── 2. Muat resep (master & legacy) sekali jalan ─────────
	masterIDs, legacyIDs := []string{}, []string{}
	for _, p := range prods {
		if p.master != "" {
			masterIDs = append(masterIDs, p.master)
		} else if p.stockType == "recipe" {
			legacyIDs = append(legacyIDs, p.id)
		}
	}
	type mat struct {
		itemID  string
		qtyBase float64
	}
	masterRecipes := map[string][]mat{}
	if len(masterIDs) > 0 {
		r, err := database.DB.Query(`SELECT recipe_master_id, item_id, qty_base FROM recipe_items WHERE recipe_master_id = ANY($1)`, pq.Array(masterIDs))
		if err != nil {
			return nil, err
		}
		for r.Next() {
			var mid string
			var m mat
			if r.Scan(&mid, &m.itemID, &m.qtyBase) == nil {
				masterRecipes[mid] = append(masterRecipes[mid], m)
			}
		}
		r.Close()
	}
	legacyRecipes := map[string][]mat{}
	if len(legacyIDs) > 0 {
		r, err := database.DB.Query(`SELECT product_id, item_id, qty_base FROM product_recipes WHERE product_id = ANY($1)`, pq.Array(legacyIDs))
		if err != nil {
			return nil, err
		}
		for r.Next() {
			var pid string
			var m mat
			if r.Scan(&pid, &m.itemID, &m.qtyBase) == nil {
				legacyRecipes[pid] = append(legacyRecipes[pid], m)
			}
		}
		r.Close()
	}

	// ── 3. Info item + resep internal (untuk cost WIP), tutup transitif ──
	type itemInfo struct {
		name, unit string
		avgCost    float64
	}
	infos := map[string]itemInfo{}
	wipRecipes := map[string][]mat{}

	need := map[string]bool{}
	for _, ms := range masterRecipes {
		for _, m := range ms {
			need[m.itemID] = true
		}
	}
	for _, ms := range legacyRecipes {
		for _, m := range ms {
			need[m.itemID] = true
		}
	}
	for _, p := range prods {
		if p.linked != "" {
			need[strings.TrimSpace(p.linked)] = true
		}
	}
	// Maksimal 3 tingkat WIP-dalam-WIP.
	for depth := 0; depth < 3 && len(need) > 0; depth++ {
		ids := make([]string, 0, len(need))
		for id := range need {
			if _, ok := infos[id]; !ok {
				ids = append(ids, id)
			}
		}
		need = map[string]bool{}
		if len(ids) == 0 {
			break
		}
		// avg_cost dari ledger (tertimbang lintas gudang) — kolom stock_items.avg_cost
		// tidak pernah ditulis (selalu 0), jadi HPP WIP selalu jatuh ke costing teoretis.
		r, err := database.DB.Query(`
			SELECT si.id, si.name, si.base_unit,
				(SELECT COALESCE(SUM(l.qty_base * l.avg_cost) / NULLIF(SUM(l.qty_base), 0), 0)
				 FROM stock_ledger l WHERE l.item_id = si.id AND l.qty_base > 0)
			FROM stock_items si WHERE si.id = ANY($1)`, pq.Array(ids))
		if err != nil {
			return nil, err
		}
		for r.Next() {
			var id string
			var inf itemInfo
			if r.Scan(&id, &inf.name, &inf.unit, &inf.avgCost) == nil {
				infos[strings.TrimSpace(id)] = inf
			}
		}
		r.Close()
		wr, err := database.DB.Query(`SELECT parent_item_id, child_item_id, qty_base FROM stock_item_recipes WHERE parent_item_id = ANY($1)`, pq.Array(ids))
		if err != nil {
			return nil, err
		}
		for wr.Next() {
			var pid string
			var m mat
			if wr.Scan(&pid, &m.itemID, &m.qtyBase) == nil {
				pid = strings.TrimSpace(pid)
				wipRecipes[pid] = append(wipRecipes[pid], m)
				need[m.itemID] = true
			}
		}
		wr.Close()
	}

	// costOf: avg cost bila ada; WIP tanpa avg cost → teoretis dari resep anak.
	memo := map[string]float64{}
	var costOf func(id string, depth int) float64
	costOf = func(id string, depth int) float64 {
		if c, ok := memo[id]; ok {
			return c
		}
		inf := infos[id]
		c := inf.avgCost
		if c == 0 && depth < 3 {
			for _, ch := range wipRecipes[id] {
				c += ch.qtyBase * costOf(ch.itemID, depth+1)
			}
		}
		memo[id] = c
		return c
	}

	// ── 4. Susun baris per menu ──────────────────────────────
	noRecipe := []string{}
	var sumPct float64
	pctCount := 0
	for _, p := range prods {
		row := models.PpicHppRow{
			ProductID: p.id, OutletID: p.outletID, OutletName: p.outletName,
			ProductName: p.name, Category: p.category, Price: p.price,
		}
		var mats []mat
		switch {
		case p.stockType == "single" && p.linked != "":
			mats = []mat{{itemID: strings.TrimSpace(p.linked), qtyBase: 1}}
		case p.master != "" && len(masterRecipes[p.master]) > 0:
			mats = masterRecipes[p.master]
		case len(legacyRecipes[p.id]) > 0:
			mats = legacyRecipes[p.id]
		}
		if len(mats) == 0 {
			noRecipe = append(noRecipe, p.name)
			rep.Rows = append(rep.Rows, row)
			continue
		}
		row.HasRecipe = true
		for _, m := range mats {
			inf := infos[m.itemID]
			cpb := costOf(m.itemID, 0)
			ing := models.PpicHppIngredient{
				ItemID: m.itemID, ItemName: inf.name, Unit: inf.unit,
				IsWip:   len(wipRecipes[m.itemID]) > 0,
				QtyBase: round4(m.qtyBase), CostPerBase: round4(cpb), Cost: round4(m.qtyBase * cpb),
			}
			row.Hpp += ing.Cost
			row.Ingredients = append(row.Ingredients, ing)
		}
		row.Hpp = round4(row.Hpp)
		if p.price > 0 {
			row.HppPct = round4(row.Hpp / p.price * 100)
			row.Margin = round4(p.price - row.Hpp)
			row.MarginPct = round4(100 - row.HppPct)
			row.IdealCost = round4(p.price * idealPct / 100)
			row.IsOver = row.HppPct > idealPct
			sumPct += row.HppPct
			pctCount++
			if row.IsOver {
				rep.OverCount++
			}
		}
		rep.WithRecipe++
		rep.Rows = append(rep.Rows, row)
	}

	rep.ProductCount = len(prods)
	if pctCount > 0 {
		rep.AvgHppPct = round4(sumPct / float64(pctCount))
	}
	rep.NoRecipeSample = strings.Join(uniqueHead(noRecipe, 10), ", ")

	// Ber-resep dulu, HPP% tertinggi di atas (yang paling perlu ditindak).
	sort.SliceStable(rep.Rows, func(i, j int) bool {
		a, b := rep.Rows[i], rep.Rows[j]
		if a.HasRecipe != b.HasRecipe {
			return a.HasRecipe
		}
		return a.HppPct > b.HppPct
	})
	return rep, nil
}
