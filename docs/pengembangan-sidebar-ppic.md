# Pengembangan Sidebar PPIC — Dashboard & Tools

| | |
|---|---|
| **Status** | **SELESAI — Fase 1, 2 & 3 diimplementasikan** 29 Jul 2026. F1: dashboard, par level/ROP, monitor kedaluwarsa, stock opname. F2: demand forecast (WMA-DOW + MAPE), MRP → draft PR/transfer. F3: rencana produksi (MPS) → work order (yield + HPP aktual + batch ber-expiry), laporan PPIC (variance pemakaian & food cost, produksi & yield, akurasi forecast, OTIF vendor) + export Excel. Catatan implementasi: forecast & on-order PR di-key per nama (estimasi); OTIF berbasis `need_by_date` (terisi otomatis oleh MRP) dengan in-full sebagai proxy status diterima. |
| **Tanggal** | 29 Juli 2026 |
| **Modul terdampak** | Sidebar/UI, permission, database, handlers/services, scheduler |
| **Referensi** | `docs/kontrak-waktu-transaksi-utc.md`, `laporan/Laporan-PPIC_Penjualan-Produk_*.xlsx` |

---

## 1. Latar Belakang

Tim PPIC (Production Planning & Inventory Control) saat ini bekerja **di luar sistem**:
export Excel penjualan produk (`laporan/Laporan-PPIC_Penjualan-Produk_Semua-Outlet_*.xlsx`)
diolah manual untuk memutuskan berapa yang harus diproduksi, dibeli, dan ditransfer.

Padahal hampir semua data mentah yang dibutuhkan PPIC **sudah ada di cloud-pos**:

| Data | Sumber existing |
|---|---|
| Penjualan per produk per outlet per hari (histori demand) | `cloud_orders` + item, halaman Penjualan Produk |
| Stok per gudang, nilai stok, stok minimum | `stock_items`, `stock_ledger`, `stock_movements` |
| Batch FIFO + tanggal kedaluwarsa | `stock_batches` (kolom `expiry_date` sudah ada) |
| Resep produk → bahan (BOM level 1) | `product_recipes` |
| Resep barang setengah jadi (BOM level 2) + produksi | `stock_item_recipes`, endpoint `POST /stock-items/:id/produce` |
| Waste (rusak/expired/hilang) + nilai rupiah | `stock_wastes` |
| Pengadaan: PR → approve → purchasing → terima barang | `purchase_requests`, `goods_receipts`, `vendors` |
| Transfer antar gudang | `stock_transfers` (draft→pending→sent→received) |

Yang **belum ada** adalah lapisan di atasnya:

1. **Perencanaan** — forecast permintaan, rencana produksi, perhitungan kebutuhan bahan (MRP), par level & titik pesan ulang (ROP).
2. **Pengendalian** — monitor kedaluwarsa/FEFO, stock opname siklus, analisis selisih pemakaian teoretis vs aktual.
3. **Satu dashboard** yang merangkum kesehatan rantai pasok dalam KPI standar F&B.

Dokumen ini merancang grup sidebar baru **"PPIC"** yang mengisi ketiga lapisan itu.

---

## 2. Tujuan

1. PPIC bisa mengambil keputusan harian (produksi apa, beli apa, transfer apa, buang apa) **dari satu layar**, tanpa export-olah manual.
2. Semua angka dihitung dengan **formula standar industri F&B** dan bisa diaudit (drill-down sampai ke buku stok / transaksi).
3. Keputusan sistem **menyambung ke modul existing** — saran beli menjadi draft PR di modul Pengadaan, saran transfer menjadi draft di Transfer Stok — bukan membuat alur paralel.
4. Hak akses granular per submenu, konsisten dengan pola `AllPermissions` yang sudah berjalan.

---

## 3. Standar F&B yang Menjadi Acuan

Praktik baku PPIC di industri F&B yang diadopsi modul ini, beserta KPI dan ambang umum industri (ambang final dikonfirmasi manajemen, disimpan sebagai setting):

| Praktik standar | Wujud di modul | KPI | Ambang umum F&B |
|---|---|---|---|
| **FEFO** (First-Expired-First-Out) & kontrol shelf life | Monitor Kedaluwarsa; `shelf_life_days` per item; batch expiry saat terima barang & produksi | Nilai stok berisiko expired | 0 batch lewat expiry di gudang |
| **Par level / Safety stock / ROP** | Halaman Par Level & ROP per gudang-per item; alert otomatis | Item di bawah ROP; stockout count | Stockout bahan kunci = 0 |
| **MRP** (Material Requirements Planning) | Halaman Kebutuhan Bahan: ledakan BOM dari forecast → kebutuhan bersih → saran beli/transfer | Fill rate / service level | ≥ 95% |
| **MPS** (Master Production Schedule) untuk dapur produksi/central kitchen | Rencana Produksi + Work Order dengan pencatatan yield | Plan adherence; yield % | Adherence ≥ 90% |
| **Food cost & usage variance** (teoretis vs aktual) | Analisis Variance: pemakaian menurut resep × penjualan vs pemakaian riil buku stok | Variance %; Food cost % | Variance < 5%; food cost 25–35% omzet |
| **Waste management** | Data `stock_wastes` diagregasi + target | Waste % terhadap pemakaian | < 2–5% |
| **Cycle count / stock opname** | Sesi opname per gudang (blind count) → selisih → approve → penyesuaian ledger | Akurasi stok (IRA) | ≥ 97% |
| **Inventory turnover & coverage** | Dashboard | ITR; Days of Inventory (DOI) | Bahan segar DOI 2–5 hari; kering 14–30 hari |
| **Forecast accuracy** | Forecast vs aktual otomatis dievaluasi tiap hari | MAPE / akurasi | Akurasi ≥ 80% |
| **Kinerja vendor** | Dari `purchase_requests` + `goods_receipts` | OTIF (on-time-in-full) | ≥ 90% |

---

## 4. Posisi & Struktur Sidebar

Grup baru **PPIC** disisipkan di `NAV_ITEMS` ([DashboardLayout.vue](../ui/src/layouts/DashboardLayout.vue)) **setelah "Pengadaan", sebelum "Gudang"** — berurutan secara logika rantai pasok: Pengadaan → PPIC → Gudang. Ikon: clipboard-with-chart.

```
PPIC
├── Dashboard              /ppic/dashboard        ppic.dashboard.view
├── Par Level & ROP        /ppic/planning-params  ppic.planning.view
├── Monitor Kedaluwarsa    /ppic/expiry           ppic.expiry.view
├── Stock Opname           /ppic/opname           ppic.opname.view
├── Demand Forecast        /ppic/forecast         ppic.forecast.view
├── Kebutuhan Bahan (MRP)  /ppic/mrp              ppic.mrp.view
├── Rencana Produksi       /ppic/production-plans ppic.production.view
├── Work Order             /ppic/work-orders      ppic.workorders.view
└── Laporan PPIC           /ppic/reports          ppic.reports.view
```

Urutan submenu = urutan fase rilis (lihat §11), jadi sidebar tumbuh bertahap tanpa pindah-pindah posisi.

Halaman Vue baru di `ui/src/pages/ppic/`: `PpicDashboard.vue`, `PlanningParams.vue`, `ExpiryMonitor.vue`, `StockOpname.vue`, `DemandForecast.vue`, `MrpWorksheet.vue`, `ProductionPlans.vue`, `WorkOrders.vue`, `PpicReports.vue`. Route + `meta.permission` di [router/index.js](../ui/src/router/index.js) mengikuti pola halaman gudang.

---

## 5. Spesifikasi Fitur

Urutan subbab mengikuti prioritas rilis: 5.1–5.4 murni membaca data yang sudah ada (fase 1), 5.5–5.6 lapisan perencanaan (fase 2), 5.7–5.9 lapisan produksi & analitik (fase 3).

### 5.1 Dashboard PPIC (`/ppic/dashboard`)

Satu layar "pagi hari PPIC": kondisi stok, risiko, dan pekerjaan hari ini. Filter global: gudang (default: semua dalam scope), rentang periode KPI (default 30 hari). Semua kartu bisa diklik → drill-down ke halaman tool terkait dengan filter terbawa.

**Baris 1 — KPI cards (8 kartu, angka + sparkline + delta vs periode sebelumnya):**

| # | KPI | Formula | Sumber | Ambang warna |
|---|---|---|---|---|
| 1 | Nilai stok total | Σ `qty × avg_cost` per gudang | `stock_ledger` | info saja |
| 2 | Item di bawah ROP | count(on_hand < reorder_point) | ledger + `item_planning_params` | 0 hijau; 1–5 kuning; >5 merah |
| 3 | Stok habis (stockout) | count(on_hand ≤ 0 & aktif) | ledger | 0 hijau; >0 merah |
| 4 | Coverage (DOI) | on_hand ÷ rata-rata pemakaian harian 28 hari, median per kategori | `stock_movements` (out) | dalam rentang target hijau |
| 5 | Nilai berisiko expired | Σ nilai batch dengan sisa umur ≤ 7 hari | `stock_batches` | 0 hijau; naik = merah |
| 6 | Waste % (30 hari) | Σ nilai waste ÷ Σ nilai pemakaian × 100 | `stock_wastes`, `stock_movements` | <2% hijau; 2–5% kuning; >5% merah |
| 7 | Akurasi forecast (7 hari) | 100% − MAPE | `demand_forecasts` vs aktual | ≥80% hijau (— sebelum fase 2) |
| 8 | Inventory turnover (30 hari) | HPP pemakaian ÷ rata-rata nilai stok | movements + ledger | per target kategori |

**Baris 2 — Pusat Peringatan (alert feed, sisi kiri):** daftar terurut prioritas, realtime via SSE:
- 🔴 Batch **sudah lewat** expiry (aksi cepat: buat Stok Rusak/Hilang alasan `expired`)
- 🔴 Item kunci **stockout** / di bawah safety stock
- 🟡 Batch expired ≤ 3 hari; item menyentuh ROP (aksi cepat: masukkan ke draft MRP/PR)
- 🟡 PR dari MRP belum diproses > X hari; transfer `sent` belum `received` > X hari
- ⚪ Dead stock: ada stok tapi tanpa pergerakan keluar > 30 hari

**Baris 2 kanan + baris 3 — Grafik (ApexCharts, konsisten dengan dashboard existing):**
- Line: pemakaian harian (nilai) vs penerimaan harian, 30 hari — melihat keseimbangan in/out.
- Line: forecast vs aktual penjualan agregat, 14 hari (fase 2).
- Bar horizontal: Top 10 item waste by nilai, 30 hari.
- Heatmap kalender: nilai batch yang akan expired per hari, 30 hari ke depan.
- Tabel ringkas: rekap per gudang (nilai stok, item low, item expired-risk) — versi ringkas dari Dashboard Gudang.

**API:** `GET /api/admin/ppic/dashboard?warehouse_id=&days=` → satu payload agregat (pola `WarehouseDashboardStats`). Wajib difilter scope outlet/work-unit (`getOutletScope`, lihat §9.5).

### 5.2 Par Level & ROP (`/ppic/planning-params`)

Parameter perencanaan **per item per gudang** — melengkapi `min_stock` yang sudah ada dengan parameter standar:

| Parameter | Arti | Pengisian |
|---|---|---|
| `lead_time_days` | Lama pesan → barang datang | Manual; default per item; bisa dihitung dari histori PR→GR |
| `safety_stock` | Penyangga variabilitas | Manual, atau saran sistem = `z × σ(pemakaian harian) × √lead_time` (z=1.65 ≈ service level 95%) |
| `reorder_point` | Titik pesan ulang | `(pemakaian harian rata-rata × lead_time_days) + safety_stock` |
| `par_level` | Stok ideal setelah pesanan datang | `pemakaian harian × (lead_time + siklus pesan) + safety_stock` |
| `moq` | Minimum order quantity vendor | Manual |

UI: tabel gaya `StockLedger.vue` (filter gudang/kategori, pencarian), kolom pemakaian harian aktual 28 hari sebagai pembanding, edit inline + **"Hitung saran"** massal (sistem mengisi kolom saran dari histori `stock_movements`; user meninjau lalu menyimpan). Item di bawah ROP di-highlight.

Kolom `min_stock`/`warehouse_min_stock` existing tetap dipakai sebagai fallback tampilan "stok rendah" lama; alert PPIC memakai ROP bila terisi, jika tidak jatuh ke min_stock.

**API:** `GET/PUT /api/admin/ppic/planning-params` (bulk upsert), `POST /api/admin/ppic/planning-params/suggest`.

### 5.3 Monitor Kedaluwarsa — FEFO (`/ppic/expiry`)

Membuat kolom `expiry_date` di `stock_batches` yang selama ini pasif menjadi alat kontrol harian.

- **Bucket aging:** Sudah lewat | ≤3 hari | ≤7 hari | ≤30 hari | Aman | Tanpa tanggal. Kartu ringkasan qty + nilai per bucket.
- Tabel batch: item, gudang, qty sisa, nilai, tanggal expiry, umur sisa, asal (GR/produksi — dari `ref_type`). Filter gudang/kategori/bucket.
- **Aksi cepat per batch:** Buang (prefill form Stok Rusak/Hilang alasan `expired`), Transfer (prefill draft transfer — mis. tarik ke outlet yang penjualannya cepat), tandai **Sudah ditindak** (catatan, supaya alert senyap).
- **Indikator disiplin FEFO:** daftar kejadian barang keluar yang mengambil batch lebih muda padahal ada batch lebih tua sejenis (dihitung dari urutan pemotongan batch) — bahan coaching tim gudang.
- Kelengkapan data: persentase penerimaan barang tanpa tanggal expiry per gudang (target 100% terisi untuk kategori segar); GR form perlu menjadikan expiry wajib untuk kategori yang ditandai `perishable` (§6.2).

**API:** `GET /api/admin/ppic/expiry?warehouse_id=&bucket=`, `POST /api/admin/ppic/expiry/:batch_id/ack`.

### 5.4 Stock Opname / Cycle Count (`/ppic/opname`)

Saat ini penyesuaian stok hanya lewat adjustment satuan (`stockledger.adjust`). Opname menghadirkan proses baku yang bisa diaudit:

**Alur status:** `draft` → `counting` → `review` → `approved` / `cancelled`.

1. **Buat sesi** per gudang; pilih cakupan: semua item, per kategori, atau ABC-cycle (item nilai tertinggi lebih sering dihitung).
2. **Counting** — petugas mengisi qty fisik (mode *blind count*: qty sistem disembunyikan, standar audit). Qty sistem di-snapshot saat sesi dibuat.
3. **Review** — tabel selisih: qty sistem vs fisik, selisih qty & nilai (`× avg_cost`), wajib isi alasan untuk selisih di atas ambang.
4. **Approve** (permission terpisah `ppic.opname.approve`) → sistem posting `stock_movements` tipe `adjustment` per item selisih, ref_type `stock_opname`. Ledger dan batch mengikuti mekanisme adjustment existing.

Dashboard menampilkan **Inventory Record Accuracy** = item tanpa selisih ÷ item dihitung, per sesi & tren.

**API:** `GET/POST /api/admin/ppic/opnames`, `GET /api/admin/ppic/opnames/:id`, `PUT .../items` (isi hitungan), `POST .../submit|approve|cancel`.

### 5.5 Demand Forecast (`/ppic/forecast`)

Perkiraan penjualan **per produk per outlet per hari** — bahan bakar MRP dan rencana produksi.

- **Metode default (pragmatis, tanpa ML):** *weighted moving average per hari-dalam-minggu* — penjualan F&B sangat berpola mingguan. Forecast Sabtu depan = rata-rata tertimbang 4 Sabtu terakhir (bobot 40/30/20/10), dihitung dari `cloud_orders`. Hari libur/event bisa dikecualikan dari histori.
- **Penyesuaian manual:** PPIC bisa menimpa angka per produk/tanggal (kolom `qty_manual`, angka sistem tetap tersimpan untuk evaluasi) + catatan event ("long weekend", "promo").
- **Evaluasi otomatis:** tiap hari, aktual kemarin diisi ke baris forecast → MAPE per produk/outlet/total tampil di halaman ini dan dashboard. Produk dengan akurasi buruk di-highlight untuk ditinjau.
- Horizon default 7 hari ke depan, digenerate ulang tiap malam oleh scheduler (§9.4); bisa regenerate manual.
- UI: matriks produk × 7 hari (angka sistem abu-abu, angka manual biru), grafik line forecast vs aktual 30 hari terakhir, filter outlet/kategori produk.
- Jalur upgrade: kolom `method` per baris memungkinkan A/B metode lain (exponential smoothing, dsb.) tanpa ubah skema.

**API:** `GET /api/admin/ppic/forecasts?outlet_id=&from=&to=`, `PUT /api/admin/ppic/forecasts` (bulk penyesuaian manual), `POST /api/admin/ppic/forecasts/generate`.

### 5.6 Kebutuhan Bahan — MRP (`/ppic/mrp`)

Menjawab "**besok/minggu ini harus beli & transfer apa, berapa**" — worksheet harian PPIC.

**Kalkulasi (per run, tersimpan sebagai snapshot untuk audit):**

```
1. Kebutuhan kotor  = Σ forecast produk × product_recipes (bahan level 1)
                      + ledakan stock_item_recipes untuk bahan setengah jadi (level 2)
                      + buffer par level (isi ulang sampai par untuk item non-resep)
2. Tersedia         = on-hand per gudang (stock_ledger)
3. Dalam perjalanan = PR status approved/purchasing yang belum diterima penuh (on-order)
                      + transfer masuk status sent
4. Kebutuhan bersih = kotor − tersedia − dalam perjalanan + safety_stock
5. Saran            : per item per gudang tujuan —
                      → TRANSFER bila gudang pusat punya stok cukup
                      → BELI (dibulatkan ke MOQ & kelipatan dist_unit) bila tidak
```

**UI worksheet:** parameter run (horizon hari, gudang tujuan, sumber: forecast / par-only), tabel hasil per item: kebutuhan kotor → bersih (kolom bisa di-expand melihat perhitungan), saran qty & jenis, vendor terakhir + harga terakhir (dari histori PR), checkbox per baris.

**Eksekusi — inti integrasinya:** tombol **"Buat Draft PR"** mengelompokkan baris terpilih per vendor → membuat `purchase_requests` status awal alur existing dengan `origin='mrp'`, lalu mengikuti alur approve existing (`procurement.requests.submit/approve/purchasing`). Tombol **"Buat Draft Transfer"** → `stock_transfers` status `draft`. PPIC menyarankan; keputusan akhir tetap di alur persetujuan yang sudah ada.

**API:** `POST /api/admin/ppic/mrp/run`, `GET /api/admin/ppic/mrp/runs/:id`, `POST /api/admin/ppic/mrp/runs/:id/create-pr`, `POST .../create-transfer`.

### 5.7 Rencana Produksi (MPS) & Work Order (`/ppic/production-plans`, `/ppic/work-orders`)

Untuk dapur produksi / central kitchen yang membuat barang setengah jadi. Saat ini produksi lewat `POST /stock-items/:id/produce` bersifat instan tanpa perencanaan & tanpa pencatatan yield. Modul ini menambah lapisan rencana di atasnya:

**Rencana Produksi (header per gudang per tanggal):** `draft` → `approved` → `released` → `closed`.
- Baris item setengah jadi + qty rencana; kolom bantu: forecast kebutuhan, on-hand, par. Saat `released`, tiap baris menjadi Work Order.

**Work Order:** `planned` → `in_progress` → `done` / `cancelled`.
- Saat mulai: sistem menampilkan kebutuhan bahan dari `stock_item_recipes` × qty rencana + cek ketersediaan.
- Saat selesai: input **qty aktual jadi** dan qty bahan aktual terpakai (prefill sesuai resep, bisa dikoreksi). Sistem posting: bahan keluar (`stock_movements` ref_type `work_order`, pemotongan batch FIFO existing) dan hasil masuk sebagai **batch baru dengan `expiry_date = hari ini + shelf_life_days`** item tersebut.
- **Yield %** = qty aktual ÷ qty rencana; **variance bahan per WO** = pemakaian aktual vs resep. Dua angka pengendalian produksi standar yang selama ini tidak tercatat.
- WO mandiri (tanpa rencana) tetap boleh — menggantikan tombol "produce" lama secara bertahap.

**API:** CRUD `/api/admin/ppic/production-plans` + `POST .../release`; `GET/POST /api/admin/ppic/work-orders`, `POST .../:id/start|finish|cancel`.

### 5.8 Analisis Variance Pemakaian (bagian dari Laporan PPIC)

Kontrol food cost standar F&B: **pemakaian teoretis vs aktual** per item per periode.

```
Teoretis = Σ (qty produk terjual × product_recipes) + Σ (qty WO × stock_item_recipes)
Aktual   = Σ stock_movements keluar (sale/produksi/waste) periode yang sama
Variance = (Aktual − Teoretis) ÷ Teoretis × 100%
```

- Tabel per item: teoretis, aktual, selisih qty & nilai, variance %; merah bila > ambang (default 5%). Drill-down ke buku stok.
- Penyebab umum yang langsung kelihatan: porsi tidak sesuai resep, resep belum lengkap (item tanpa resep dilaporkan terpisah — *bukan* dicampur ke variance), kebocoran/waste tak tercatat.
- **Food cost % aktual** = nilai pemakaian ÷ omzet penjualan periode (omzet dari laporan penjualan existing).
- Prasyarat yang jujur: laporan menampilkan **coverage resep** (persen produk terjual yang punya resep). Di bawah 80%, angka variance ditandai "belum representatif".

### 5.9 Laporan PPIC & Export (`/ppic/reports`)

Satu halaman tab (pola `VoidReport.vue`), semua bisa export Excel via infra `services/report_export.go` (excelize v2.10.0):

1. **Pemakaian & Variance** (§5.8) — per item/kategori/gudang.
2. **Waste** — per alasan, per gudang, tren, % terhadap pemakaian.
3. **Pergerakan & Coverage** — pemakaian harian, DOI, turnover per kategori.
4. **Kedaluwarsa** — batch dibuang karena expired (nilai kerugian), kelengkapan data expiry.
5. **Produksi** — realisasi vs rencana, yield per item per periode (fase 3).
6. **Forecast accuracy** — MAPE per produk/outlet (fase 2).
7. **Kinerja vendor (OTIF)** — dari PR/GR: tepat waktu (≤ tanggal butuh) & lengkap (qty terima = qty pesan).
8. **HPP Menu (costing card)** — ditambahkan setelah membandingkan dengan laporan manual PPIC (`HPP SEKAR AYU (JUNI 2026) FIXED.xlsx`): per menu, resep di-explode (termasuk komponen WIP) → HPP teoretis, HPP % vs harga jual, margin, ambang ideal yang bisa diatur (default 35%); biaya WIP memakai HPP produksi aktual (avg cost) atau teoretis dari resep bila belum pernah diproduksi. Export Excel meniru format sheet "Food Cost" manual. Gap yang disadari dan belum dibangun: pemisahan lokasi Kitchen/Bar dalam satu outlet (workaround: opname per kategori).

Export ini menggantikan file manual `Laporan-PPIC_Penjualan-Produk_*.xlsx` — format kolomnya disamakan dulu dengan file itu agar transisi tim mulus, lalu diperkaya.

---

## 6. Perubahan Skema Database

Semua lewat `database/migrations.go` (idempotent, `CREATE TABLE IF NOT EXISTS` / `ALTER ... ADD COLUMN IF NOT EXISTS`).

### 6.1 Tabel baru

```sql
-- Parameter perencanaan per item per gudang (§5.2)
item_planning_params (
  item_id CHAR(26) REFERENCES stock_items, warehouse_id CHAR(26) REFERENCES warehouses,
  lead_time_days INT DEFAULT 0, safety_stock NUMERIC DEFAULT 0,
  reorder_point NUMERIC DEFAULT 0, par_level NUMERIC DEFAULT 0, moq NUMERIC DEFAULT 0,
  updated_by, updated_at, PRIMARY KEY (item_id, warehouse_id))

-- Forecast per produk per outlet per hari (§5.5)
demand_forecasts (
  id, outlet_id, product_id, forecast_date DATE,
  qty_system NUMERIC, qty_manual NUMERIC NULL, qty_actual NUMERIC NULL,
  method VARCHAR DEFAULT 'wma_dow', event_note TEXT, updated_by, updated_at,
  UNIQUE (outlet_id, product_id, forecast_date))

-- Snapshot kalkulasi MRP (§5.6) — auditable
mrp_runs (id, run_number, horizon_days, source VARCHAR, warehouse_id, status, notes, created_by, created_at)
mrp_run_items (
  id, run_id, item_id, warehouse_id,
  gross_req, on_hand, on_order, in_transit, safety_stock, net_req,
  suggestion VARCHAR,           -- purchase | transfer | none
  qty_suggested, vendor_id NULL, last_price NUMERIC,
  action_ref_type VARCHAR NULL, action_ref_id CHAR(26) NULL)  -- PR/transfer yang dibuat

-- Opname (§5.4)
stock_opnames (id, opname_number, warehouse_id, scope VARCHAR, status, notes,
  created_by, counted_by, approved_by, started_at, finished_at, created_at)
stock_opname_items (id, opname_id, item_id, qty_system_base, qty_counted_base NULL,
  diff_base, cost_per_base, diff_value, reason, notes)

-- Produksi (§5.7)
production_plans (id, plan_number, warehouse_id, plan_date DATE, status, notes,
  created_by, approved_by, approved_at, created_at, updated_at)
production_plan_items (id, plan_id, item_id, qty_planned_base, qty_forecast_base, notes)
work_orders (id, wo_number, plan_id NULL, warehouse_id, item_id,
  qty_planned_base, qty_actual_base NULL, yield_pct NUMERIC NULL,
  status, started_at, finished_at, executed_by, notes, created_by, created_at)
work_order_materials (id, wo_id, item_id, qty_plan_base, qty_actual_base NULL, cost_per_base)
```

### 6.2 Perubahan tabel existing

| Tabel | Kolom baru | Untuk |
|---|---|---|
| `stock_items` | `shelf_life_days INT DEFAULT 0`, `default_lead_time_days INT DEFAULT 0` | Expiry otomatis hasil produksi; fallback lead time |
| `stock_item_categories` | `is_perishable BOOL DEFAULT false`, `target_doi_min/max INT` | Wajib-expiry di GR; ambang coverage per kategori |
| `purchase_requests` | `origin VARCHAR DEFAULT 'manual'`, `mrp_run_id CHAR(26) NULL`, `need_by_date DATE NULL` | Jejak PR hasil MRP; basis OTIF |
| `stock_transfers` | `origin VARCHAR DEFAULT 'manual'`, `mrp_run_id CHAR(26) NULL` | Jejak transfer hasil MRP |

Tidak ada perubahan pada tabel ledger/batch/movement — semua transaksi PPIC memakai mekanisme posting existing (`ref_type` baru: `stock_opname`, `work_order`).

### 6.3 Catatan migrasi permission (gotcha yang sudah pernah kejadian)

Seeding permission `ppic.*` ke role (mis. superadmin/admin) **wajib ditaruh di paling akhir `RunMigrations`** (tepat sebelum `return nil`) dan **diguard marker `app_settings`** (pola `mig_split_titipan_shiftrecon`) — karena blok re-seed coarse-key berjalan tiap boot dan backfill tanpa guard akan menghidupkan lagi izin yang sudah dicabut admin. Key `ppic.*` bersifat multi-dot → pastikan tetap kompatibel dengan cascade `lastIndexOf('.')` di `Roles.vue`.

---

## 7. Ringkasan Endpoint API

Semua di grup admin (`routes/routes.go`), dibungkus `middleware.RequirePermission`, respons difilter scope outlet/work-unit via helper existing.

| Method & Path (`/api/admin/ppic/...`) | Permission | Keterangan |
|---|---|---|
| `GET dashboard` | `ppic.dashboard.view` | Payload agregat KPI+alert+chart |
| `GET/PUT planning-params`, `POST planning-params/suggest` | `ppic.planning.view` / `.update` | Par/ROP/safety stock |
| `GET expiry`, `POST expiry/:batch_id/ack` | `ppic.expiry.view` / `.ack` | Monitor FEFO |
| `GET/POST opnames`, `PUT opnames/:id/items`, `POST opnames/:id/{submit,approve,cancel}` | `.view`/`.create`/`.approve` | approve posting adjustment |
| `GET/PUT forecasts`, `POST forecasts/generate` | `ppic.forecast.view` / `.manage` | |
| `POST mrp/run`, `GET mrp/runs`, `GET mrp/runs/:id` | `ppic.mrp.view` / `.run` | |
| `POST mrp/runs/:id/create-pr`, `.../create-transfer` | `ppic.mrp.execute` | Membuat draft di modul existing |
| CRUD `production-plans`, `POST production-plans/:id/{approve,release}` | `.view`/`.create`/`.approve` | |
| `GET/POST work-orders`, `POST work-orders/:id/{start,finish,cancel}` | `.view`/`.create`/`.execute` | finish = posting stok |
| `GET reports/{variance,waste,coverage,expiry,production,forecast-accuracy,vendor-otif}` + `/export` | `ppic.reports.view` / `.export` | Export via `report_export` |

Backend baru: `handlers/ppic.go`, `services/ppic_dashboard.go`, `services/ppic_planning.go`, `services/ppic_forecast.go`, `services/ppic_mrp.go`, `services/ppic_production.go`, `services/ppic_opname.go`, `models/ppic.go` — mengikuti pemisahan handler/service existing.

## 8. Permission Baru (`services.AllPermissions`)

```go
// PPIC
"ppic.dashboard.view",
"ppic.planning.view", "ppic.planning.update",
"ppic.expiry.view", "ppic.expiry.ack",
"ppic.opname.view", "ppic.opname.create", "ppic.opname.approve",
"ppic.forecast.view", "ppic.forecast.manage",
"ppic.mrp.view", "ppic.mrp.run", "ppic.mrp.execute",
"ppic.production.view", "ppic.production.create", "ppic.production.approve",
"ppic.workorders.view", "ppic.workorders.create", "ppic.workorders.execute",
"ppic.reports.view", "ppic.reports.export",
```

Aturan prefix `RequirePermission` existing (punya `ppic.opname.*` apa pun → lolos `.view`) otomatis berlaku. `PermissionMatrix.vue` menampilkan kategori sidebar baru "PPIC". Role bawaan yang disarankan: **PPIC Staff** (semua `.view` + opname.create + forecast.manage + mrp.run) dan **PPIC Supervisor** (semua `ppic.*`).

## 9. Integrasi dengan Modul Existing

1. **Pengadaan** — MRP membuat draft `purchase_requests` (`origin='mrp'`) lalu masuk alur `submit → approve → purchasing` existing tanpa perubahan; `need_by_date` + `goods_receipts` menjadi basis OTIF vendor. Kolom asal ditampilkan di halaman Barang (badge "MRP").
2. **Gudang** — opname & WO posting lewat mekanisme movement/batch existing (FIFO tetap satu pintu). Halaman Penerimaan Barang: field expiry menjadi **wajib** bila kategori item `is_perishable`.
3. **Produk/Resep** — MRP & variance bergantung kelengkapan `product_recipes`; laporan menampilkan coverage resep, dan produk terjual tanpa resep dilaporkan eksplisit.
4. **Scheduler (baru, goroutine ticker harian di `main.go`)** — 02:00 waktu server: isi `qty_actual` forecast kemarin, generate forecast horizon berikutnya, hitung ulang saran ROP, evaluasi alert (expiry/ROP) → emit SSE `ppic_alert` via `services/events.go`. Ikuti kontrak timezone (`docs/kontrak-waktu-transaksi-utc.md`): agregasi harian per zona outlet.
5. **Scope work-unit** — semua endpoint PPIC memakai `getOutletScope`/`getWorkUnitScope`; ingat aturan sentinel `__none__` untuk scope specific-kosong (jangan bypass). Katalog item & resep global; angka stok/pemakaian/forecast difilter.
6. **SSE** — event baru `ppic_alert` (payload: tipe, severity, item, gudang) dikonsumsi Pusat Peringatan dashboard, pola sama dengan realtime existing.

## 10. Pedoman UI/UX

- **Modern & informatif:** kartu KPI = angka besar + sparkline + delta; semantik warna konsisten (hijau aman / kuning waspada / merah tindakan); setiap angka bisa diklik ke sumbernya (no dead-end numbers); skeleton loading; responsive (tablet dipakai saat opname di gudang).
- **Bahasa:** label Indonesia konsisten dengan sidebar existing ("Kebutuhan Bahan", "Rencana Produksi"), istilah teknis standar tetap ditampilkan (ROP, FEFO, MRP) karena itu vocabulary PPIC.
- **Komponen:** pakai komponen existing (`AppInput`, tabel & filter pola halaman gudang, ApexCharts). Tanpa library baru di fase 1–2.
- **Empty state edukatif:** halaman fase lanjut yang datanya belum siap (mis. variance saat resep <80%) menjelaskan prasyaratnya, bukan menampilkan angka menyesatkan.

## 11. Roadmap Implementasi

| Fase | Isi | Prasyarat data | Perkiraan |
|---|---|---|---|
| **1 — Pengendalian** (quick win) | Dashboard (tanpa KPI forecast), Par Level & ROP, Monitor Kedaluwarsa, Stock Opname; permission & sidebar; scheduler alert | Tidak ada — semua dari data existing | 2–3 minggu |
| **2 — Perencanaan** | Demand Forecast + evaluasi MAPE, MRP + draft PR/Transfer, KPI forecast di dashboard, laporan forecast & OTIF | Histori penjualan ≥ 4 minggu (sudah ada); disiplin isi lead time & MOQ | 3–4 minggu |
| **3 — Produksi & Analitik** | Rencana Produksi + Work Order + yield, Variance pemakaian, Laporan PPIC lengkap + export | Kelengkapan resep ≥ 80% produk terjual; `shelf_life_days` terisi | 3–4 minggu |

**Kriteria terima per fase (ringkas):**
- F1: alert expiry & ROP muncul realtime dan akurat vs buku stok; satu siklus opname penuh (blind → approve → adjustment tercatat di ledger); IRA tampil.
- F2: MRP run menghasilkan draft PR yang benar (net req memperhitungkan on-order & in-transit, dibulatkan MOQ); akurasi forecast terukur otomatis setiap hari.
- F3: WO memotong bahan sesuai FIFO, hasil produksi masuk sebagai batch ber-expiry; laporan variance cocok dengan buku stok; export Excel PPIC menggantikan file manual.

Build & rilis mengikuti alur existing: backend via image `golang:1.24-alpine` (host tanpa Go), UI + nginx rebuild via docker compose.

## 12. Risiko & Mitigasi

| Risiko | Dampak | Mitigasi |
|---|---|---|
| Resep tidak lengkap/tidak akurat | MRP & variance menyesatkan | Metrik coverage resep tampil di laporan; item tanpa resep dilaporkan terpisah; fase 3 menunggu coverage ≥ 80% |
| Expiry GR jarang diisi | Monitor FEFO kosong | Wajib-expiry untuk kategori perishable; KPI kelengkapan per gudang |
| Forecast dianggap "perintah" | Over/under produksi | Forecast selalu bisa dioverride manual; MRP hanya membuat **draft** yang tetap lewat approval existing |
| Beban query agregasi di dashboard | Lambat | Satu endpoint agregat, index pada `stock_movements(warehouse_id, created_at)`, `stock_batches(expiry_date)`, `demand_forecasts(forecast_date)`; hitung berat di scheduler malam bila perlu |
| Re-seed permission tiap boot | Izin dicabut hidup lagi | Guard marker `app_settings` + blok di akhir `RunMigrations` (§6.3) |
| Angka PPIC vs laporan keuangan beda | Kepercayaan turun | Semua nilai pakai `avg_cost` & ledger yang sama dengan laporan existing; drill-down ke buku stok |
