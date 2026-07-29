<template>
  <div class="space-y-5">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-xl font-bold text-gray-900">Demand Forecast</h1>
        <p class="text-xs text-gray-500 mt-0.5">
          Perkiraan penjualan per produk per hari — rata-rata tertimbang 4 minggu terakhir per hari-yang-sama (bobot 40/30/20/10), dibulatkan ke porsi utuh.
          Angka <span class="font-semibold text-blue-700">biru</span> = penyesuaian manual PPIC.
        </p>
      </div>
      <div class="flex gap-2">
        <AppButton v-if="canManage" variant="secondary" :loading="generating" @click="generate">
          Generate Ulang (7 hari)
        </AppButton>
        <AppButton v-if="canManage" variant="primary" :disabled="!dirtyCount" :loading="saving" @click="saveAll">
          Simpan Penyesuaian ({{ dirtyCount }})
        </AppButton>
      </div>
    </div>

    <!-- Ringkasan akurasi -->
    <div class="acc-grid">
      <div class="acc" :class="accClass">
        <div class="acc-label">Akurasi Forecast 14 Hari</div>
        <div class="acc-val">{{ d?.eval_rows_14d ? d.accuracy_14d.toFixed(1) + '%' : '—' }}</div>
        <div class="acc-sub">{{ d?.eval_rows_14d ? `${d.eval_rows_14d} titik evaluasi (100 − MAPE) · target ≥ 80%` : 'belum ada data aktual untuk dievaluasi' }}</div>
      </div>
      <div class="acc acc--info">
        <div class="acc-label">Produk Ter-forecast</div>
        <div class="acc-val">{{ d?.total ?? 0 }}</div>
        <div class="acc-sub">produk × outlet pada rentang tampil</div>
      </div>
      <div class="acc acc--info">
        <div class="acc-label">Cara Kerja</div>
        <div class="acc-sub" style="margin-top:.35rem">
          Scheduler tiap malam mengisi aktual kemarin lalu men-generate 7 hari ke depan.
          Klik sel untuk menimpa angka sistem (event, promo, long weekend).
        </div>
      </div>
    </div>

    <AppCard :padding="false">
      <div class="flex flex-wrap items-center gap-3 px-4 py-3 border-b border-gray-100">
        <div class="min-w-[200px]">
          <SearchSelect v-model="filters.outlet_id" :options="outletOptions" placeholder="Semua Outlet" @change="reload" />
        </div>
        <div class="min-w-[180px]">
          <SearchSelect v-model="filters.category" :options="categoryOptions" placeholder="Semua Kategori" @change="reload" />
        </div>
        <div class="flex items-center gap-1.5">
          <button class="nav-btn" @click="shiftRange(-7)" title="Mundur 7 hari">‹</button>
          <span class="text-xs font-semibold text-gray-600 whitespace-nowrap">{{ fmtDate(range.from) }} — {{ fmtDate(range.to) }}</span>
          <button class="nav-btn" @click="shiftRange(7)" title="Maju 7 hari">›</button>
          <button class="nav-btn nav-btn--txt" @click="resetRange">Minggu ini</button>
        </div>
        <input v-model="filters.search" @input="debouncedReload" placeholder="Cari produk..."
          class="flex-1 min-w-[180px] text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-emerald-400" />
      </div>

      <!-- Legenda cara baca -->
      <div class="legend">
        <span class="lg-title">Cara baca:</span>
        <span class="lg-item"><span class="lg-chip lg-chip--f">F 12</span> forecast (perkiraan terjual, porsi)</span>
        <span class="lg-item"><span class="lg-chip lg-chip--a">✓ 15</span> aktual terjual (hari yang sudah lewat)</span>
        <span class="lg-item"><span class="lg-chip lg-chip--m">18</span> angka biru = diubah manual</span>
        <span class="lg-item"><span class="lg-chip lg-chip--d">20</span> kuning = belum disimpan</span>
        <span class="lg-item text-gray-400">· Klik <b>nama produk</b> untuk melihat histori penjualan 4 minggu (bahan rumus forecast)</span>
        <span v-if="canManage" class="lg-item text-gray-400">· Klik sel hari ini/depan untuk mengubah; samakan lagi dengan angka sistem untuk membatalkan override</span>
      </div>

      <div class="overflow-x-auto">
        <table class="fc-table">
          <thead>
            <tr>
              <th class="th-prod">Produk / Outlet</th>
              <th v-for="dt in d?.dates ?? []" :key="dt" class="th-day" :class="{ 'th-today': isToday(dt), 'th-past': isPast(dt) }">
                <div>{{ dayName(dt) }}</div>
                <div class="th-date">{{ dayNum(dt) }}</div>
                <div v-if="isToday(dt)" class="th-badge">HARI INI</div>
                <div v-else-if="isPast(dt)" class="th-badge th-badge--past">LEWAT</div>
              </th>
              <th class="th-acc">Akurasi 14 hr</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading"><td :colspan="(d?.dates?.length ?? 7) + 2" class="p-8 text-center"><AppSpinner class="text-emerald-600 mx-auto" /></td></tr>
            <tr v-else-if="!rows.length"><td :colspan="(d?.dates?.length ?? 7) + 2" class="p-10 text-center text-sm text-gray-400">
              Belum ada forecast. <template v-if="canManage">Klik <b>Generate Ulang</b> untuk menghitung dari histori penjualan.</template>
            </td></tr>
            <template v-for="r in rows" :key="r.outlet_id + '|' + r.product_name">
            <tr>
              <td class="td-prod td-prod--click" @click="toggleHistory(r)" title="Klik untuk lihat histori penjualan 4 minggu (bahan rumus forecast)">
                <div class="flex items-center gap-1.5">
                  <span class="exp-arrow">{{ isHistOpen(r) ? '▾' : '▸' }}</span>
                  <span class="font-medium text-gray-900 text-sm">{{ r.product_name }}</span>
                </div>
                <div class="prod-tags">
                  <span class="tag tag--cat">{{ r.category || 'Tanpa Kategori' }}</span>
                  <span class="tag tag--outlet">{{ r.outlet_name }}</span>
                </div>
              </td>
              <td v-for="c in r.cells" :key="c.date" class="td-cell" :class="{ 'td-today': isToday(c.date), 'td-past': isPast(c.date) }">
                <!-- Hari lewat: forecast vs aktual, diberi label F / ✓ -->
                <template v-if="isPast(c.date)">
                  <div class="cell-line" :class="c.qty_manual != null ? 'txt-manual' : ''" title="Forecast (perkiraan)">
                    <span class="cell-tag">F</span>{{ fmtQty(effQty(c)) }}
                  </div>
                  <div class="cell-actual" :class="actualClass(c)" title="Aktual terjual">
                    <template v-if="c.qty_actual == null"><span class="cell-tag">✓</span>?</template>
                    <template v-else><span class="cell-tag">✓</span>{{ fmtQty(c.qty_actual) }}</template>
                  </div>
                </template>
                <!-- Hari ini & depan: editable -->
                <template v-else>
                  <input v-if="canManage" type="number" min="0" step="1" class="cell-in"
                    :class="{ 'cell-in--manual': c.qty_manual != null, 'cell-in--dirty': isCellDirty(r, c) }"
                    :value="c.qty_manual != null ? c.qty_manual : c.qty_system"
                    :title="`Sistem: ${fmtQty(c.qty_system)}${c.event_note ? ' · ' + c.event_note : ''}`"
                    @input="onCellInput(r, c, $event.target.value)" />
                  <div v-else class="cell-line" :class="c.qty_manual != null ? 'txt-manual' : ''">{{ fmtQty(effQty(c)) }}</div>
                </template>
              </td>
              <td class="td-acc">
                <span v-if="r.eval_rows" class="acc-pill" :class="r.accuracy_pct >= 80 ? 'acc-pill--ok' : 'acc-pill--warn'">
                  {{ r.accuracy_pct.toFixed(0) }}%
                </span>
                <span v-else class="text-gray-300 text-xs">—</span>
              </td>
            </tr>

            <!-- Histori penjualan: 4 minggu ke belakang di hari yang sama (bahan WMA) -->
            <template v-if="isHistOpen(r)">
              <tr v-if="!histData(r)" class="hist-row">
                <td :colspan="(d?.dates?.length ?? 7) + 2" class="p-3 text-center text-xs text-gray-400">Memuat histori penjualan…</td>
              </tr>
              <tr v-else v-for="wk in HIST_WEEKS" :key="wk.offset" class="hist-row">
                <td class="hist-label">Terjual {{ wk.label }} <span class="hist-weight">bobot {{ wk.weight }}</span></td>
                <td v-for="c in r.cells" :key="c.date" class="td-cell hist-cell" :class="{ 'td-today': isToday(c.date) }">
                  {{ histQty(r, c.date, wk.offset) }}
                </td>
                <td></td>
              </tr>
            </template>
            </template>
          </tbody>
          <tfoot v-if="rows.length">
            <tr>
              <td class="tf-label">TOTAL halaman ini ({{ rows.length }} produk)</td>
              <td v-for="(t, i) in colTotals" :key="i" class="td-cell tf-cell" :class="{ 'td-today': isToday(t.date), 'td-past': isPast(t.date) }">
                <div class="cell-line tf-f" title="Total forecast"><span class="cell-tag">F</span>{{ fmtQty(t.forecast) }}</div>
                <div v-if="isPast(t.date)" class="cell-actual tf-a" title="Total aktual terjual"><span class="cell-tag">✓</span>{{ fmtQty(t.actual) }}</div>
              </td>
              <td></td>
            </tr>
          </tfoot>
        </table>
      </div>

      <div class="px-4 py-3 border-t border-gray-100 flex items-center justify-between">
        <span class="text-xs text-gray-400">
          {{ d?.total ?? 0 }} produk ter-forecast · angka dalam porsi terjual · <b>F</b> = forecast, <b>✓</b> = aktual, <b>?</b> = aktual belum diisi (menunggu proses malam)
        </span>
        <AppPagination :page="page" :total-pages="totalPages" @change="changePage" />
      </div>
    </AppCard>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ppicApi } from '@/api/ppic'
import { apiClient } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toast'
import debounce from 'lodash/debounce'
import AppButton     from '@/components/ui/AppButton.vue'
import AppCard       from '@/components/ui/AppCard.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import AppSpinner    from '@/components/ui/AppSpinner.vue'
import SearchSelect  from '@/components/ui/SearchSelect.vue'

const auth = useAuthStore()
const toast = useToastStore()
const canManage = computed(() => auth.hasPermission('ppic.forecast.manage'))

const d = ref(null)
const rows = ref([])
const loading = ref(false)
const saving = ref(false)
const generating = ref(false)
const page = ref(1)
const totalPages = ref(1)
const filters = reactive({ outlet_id: '', search: '', category: '' })
const outletOptions = ref([{ id: '', name: 'Semua Outlet' }])
const categoryOptions = computed(() => [
  { id: '', name: 'Semua Kategori' },
  ...(d.value?.categories ?? []).map(c => ({ id: c, name: c })),
])

// Histori penjualan per produk (lazy, di-cache): "outlet|produk" → {tanggal: qty}
const HIST_WEEKS = [
  { offset: 7,  label: 'minggu lalu (H-7)',  weight: '40%' },
  { offset: 14, label: '2 minggu lalu (H-14)', weight: '30%' },
  { offset: 21, label: '3 minggu lalu (H-21)', weight: '20%' },
  { offset: 28, label: '4 minggu lalu (H-28)', weight: '10%' },
]
const histOpen = reactive({})
const histCache = reactive({})
function histKey(r) { return `${r.outlet_id}|${r.product_name}` }
function isHistOpen(r) { return !!histOpen[histKey(r)] }
function histData(r) { return histCache[histKey(r)] }
async function toggleHistory(r) {
  const k = histKey(r)
  histOpen[k] = !histOpen[k]
  if (histOpen[k] && !histCache[k]) {
    try {
      const points = await ppicApi.getForecastHistory(r.outlet_id, r.product_name)
      histCache[k] = Object.fromEntries((points || []).map(p => [p.date, p.qty]))
    } catch {
      histCache[k] = {}
      toast.error('Gagal memuat histori penjualan')
    }
  }
}
function minusDays(dateStr, days) {
  const dt = new Date(dateStr + 'T00:00:00')
  dt.setDate(dt.getDate() - days)
  return iso(dt)
}
function histQty(r, date, offset) {
  const h = histData(r)
  if (!h) return ''
  const v = h[minusDays(date, offset)]
  return v == null ? '—' : fmtQty(v)
}

// Rentang default: hari ini + 6 hari.
const range = reactive({ from: '', to: '' })
resetRangeDates()
function resetRangeDates() {
  const t = new Date()
  range.from = iso(t)
  const e = new Date(t); e.setDate(e.getDate() + 6)
  range.to = iso(e)
}
// Tanggal LOKAL (bukan toISOString/UTC — sebelum 07:00 WIB tanggalnya mundur sehari).
function iso(dt) {
  return `${dt.getFullYear()}-${String(dt.getMonth() + 1).padStart(2, '0')}-${String(dt.getDate()).padStart(2, '0')}`
}
function shiftRange(days) {
  const f = new Date(range.from); f.setDate(f.getDate() + days)
  const t = new Date(range.to); t.setDate(t.getDate() + days)
  range.from = iso(f); range.to = iso(t)
  reload()
}
function resetRange() { resetRangeDates(); reload() }

// dirty = penyesuaian manual yang belum disimpan: key "outlet|produk|tanggal" → qty
const dirty = reactive({})
const dirtyCount = computed(() => Object.keys(dirty).length)
function cellKey(r, c) { return `${r.outlet_id}|${r.product_name}|${c.date}` }
function isCellDirty(r, c) { return cellKey(r, c) in dirty }

onMounted(async () => {
  load()
  try {
    const res = await apiClient.get('/admin/my-outlets')
    const list = Array.isArray(res) ? res : (res?.data || [])
    outletOptions.value = [{ id: '', name: 'Semua Outlet' }, ...list.map(o => ({ id: o.id, name: o.name }))]
  } catch { /* selector tetap default */ }
})

async function load() {
  loading.value = true
  try {
    d.value = await ppicApi.listForecasts({
      outlet_id: filters.outlet_id, search: filters.search, category: filters.category,
      date_from: range.from, date_to: range.to,
      page: page.value, limit: 25,
    })
    rows.value = d.value?.rows || []
    totalPages.value = Math.max(1, Math.ceil((d.value?.total || 0) / 25))
  } catch (e) {
    toast.error(e?.message ?? 'Gagal memuat forecast')
  } finally {
    loading.value = false
  }
}
function reload() { page.value = 1; load() }
const debouncedReload = debounce(reload, 500)
function changePage(p) { page.value = p; load() }

function effQty(c) { return c.qty_manual != null ? c.qty_manual : c.qty_system }

function onCellInput(r, c, val) {
  const k = cellKey(r, c)
  if (val === '' || Number(val) === c.qty_system) {
    // kembali ke angka sistem → simpan sebagai null (hapus override)
    if (c.qty_manual != null) dirty[k] = null
    else delete dirty[k]
    c.qty_manual = null
  } else {
    c.qty_manual = Number(val)
    dirty[k] = Number(val)
  }
}

async function saveAll() {
  const payload = Object.entries(dirty).map(([k, qty]) => {
    const [outlet_id, product_name, date] = k.split('|')
    return { outlet_id, product_name, date, qty_manual: qty, event_note: '' }
  })
  if (!payload.length) return
  saving.value = true
  try {
    await ppicApi.saveForecasts(payload)
    toast.success(`${payload.length} sel forecast tersimpan`)
    Object.keys(dirty).forEach(k => delete dirty[k])
    load()
  } catch (e) {
    toast.error(e?.message ?? 'Gagal menyimpan forecast')
  } finally {
    saving.value = false
  }
}

async function generate() {
  generating.value = true
  try {
    const res = await ppicApi.generateForecasts(7)
    toast.success(`${res?.generated ?? 0} baris forecast digenerate`)
    resetRangeDates()
    load()
  } catch (e) {
    toast.error(e?.message ?? 'Gagal generate forecast')
  } finally {
    generating.value = false
  }
}

const accClass = computed(() => {
  if (!d.value?.eval_rows_14d) return 'acc--info'
  return d.value.accuracy_14d >= 80 ? 'acc--ok' : 'acc--warn'
})

const todayStr = iso(new Date())
function isToday(dt) { return dt === todayStr }
function isPast(dt) { return dt < todayStr }

// Total per kolom hari untuk produk yang tampil di halaman ini.
const colTotals = computed(() => {
  const dates = d.value?.dates ?? []
  return dates.map((date, i) => {
    let forecast = 0, actual = 0
    for (const r of rows.value) {
      const c = r.cells[i]
      if (!c) continue
      forecast += effQty(c) || 0
      actual += c.qty_actual || 0
    }
    return { date, forecast, actual }
  })
})
function actualClass(c) {
  if (c.qty_actual == null) return ''
  const f = effQty(c)
  if (f === 0 && c.qty_actual === 0) return 'act-ok'
  if (f === 0) return 'act-bad'
  const dev = Math.abs(c.qty_actual - f) / Math.max(c.qty_actual, 1)
  return dev <= 0.2 ? 'act-ok' : (dev <= 0.5 ? 'act-warn' : 'act-bad')
}

function fmtQty(v) { return Number(v ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 1 }) }
function dayName(s) { return new Date(s).toLocaleDateString('id-ID', { weekday: 'short' }) }
function dayNum(s) { return new Date(s).toLocaleDateString('id-ID', { day: '2-digit', month: 'short' }) }
function fmtDate(s) { return s ? new Date(s).toLocaleDateString('id-ID', { day: '2-digit', month: 'short' }) : '' }
</script>

<style scoped>
.acc-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: .6rem; }
@media (max-width: 800px) { .acc-grid { grid-template-columns: 1fr; } }
.acc { padding: .8rem 1rem; border-radius: .75rem; border: 1.5px solid #e5e7eb; background: #fff; }
.acc--ok   { background: #ecfdf5; border-color: #a7f3d0; }
.acc--warn { background: #fffbeb; border-color: #fde68a; }
.acc--info { background: #f9fafb; border-color: #e5e7eb; }
.acc-label { font-size: .64rem; font-weight: 700; text-transform: uppercase; letter-spacing: .04em; color: #6b7280; }
.acc-val { font-size: 1.3rem; font-weight: 800; color: #111827; }
.acc-sub { font-size: .68rem; color: #9ca3af; }

.nav-btn { border: 1px solid #e5e7eb; background: #fff; border-radius: .45rem; padding: .2rem .55rem; font-size: .8rem; color: #374151; cursor: pointer; }
.nav-btn:hover { background: #f3f4f6; }
.nav-btn--txt { font-size: .68rem; font-weight: 600; }

/* Legenda */
.legend { display: flex; flex-wrap: wrap; align-items: center; gap: .35rem .9rem; padding: .5rem 1rem; background: #f8fafc; border-bottom: 1px solid #f1f5f9; font-size: .68rem; color: #64748b; }
.lg-title { font-weight: 700; color: #475569; }
.lg-item { display: inline-flex; align-items: center; gap: .3rem; }
.lg-chip { font-family: monospace; font-size: .66rem; font-weight: 700; padding: .1rem .4rem; border-radius: .35rem; border: 1px solid; }
.lg-chip--f { background: #fff; border-color: #e5e7eb; color: #6b7280; }
.lg-chip--a { background: #ecfdf5; border-color: #a7f3d0; color: #059669; }
.lg-chip--m { background: #eff6ff; border-color: #bfdbfe; color: #1d4ed8; }
.lg-chip--d { background: #fefce8; border-color: #fde68a; color: #a16207; }

.fc-table { width: 100%; border-collapse: collapse; font-size: .78rem; }
.fc-table thead tr { background: #f9fafb; }
.fc-table th { padding: .45rem .5rem; border-bottom: 1px solid #f3f4f6; font-size: .62rem; font-weight: 700; text-transform: uppercase; color: #9ca3af; text-align: center; white-space: nowrap; }
.th-prod { text-align: left !important; min-width: 180px; }
.th-acc { min-width: 70px; }
.th-date { font-size: .6rem; font-weight: 600; color: #6b7280; text-transform: none; }
.th-today { background: #ecfdf5; color: #047857; }
.th-past { color: #c4c8cf; }
.th-badge { display: inline-block; font-size: .5rem; font-weight: 800; letter-spacing: .05em; padding: .05rem .3rem; border-radius: 999px; background: #059669; color: #fff; margin-top: .15rem; }
.th-badge--past { background: #e5e7eb; color: #9ca3af; }

.fc-table tbody tr { border-bottom: 1px solid #f9fafb; }
.fc-table tbody tr:hover { background: #fafafa; }
.fc-table td { padding: .3rem .4rem; }
.td-prod { min-width: 200px; }
.td-prod--click { cursor: pointer; }
.exp-arrow { color: #9ca3af; font-size: .7rem; width: 10px; flex-shrink: 0; }
.prod-tags { display: flex; flex-wrap: wrap; gap: .25rem; margin: .15rem 0 0 1rem; }
.tag { font-size: .58rem; font-weight: 700; padding: .08rem .4rem; border-radius: 999px; white-space: nowrap; }
.tag--cat { background: #f3e8ff; color: #7c3aed; }
.tag--outlet { background: #d1fae5; color: #065f46; }

/* Baris histori penjualan (expand) */
.hist-row { background: #f8fafc !important; }
.hist-label { padding: .25rem .4rem .25rem 1.4rem; font-size: .64rem; color: #64748b; white-space: nowrap; }
.hist-weight { font-size: .56rem; font-weight: 700; color: #94a3b8; background: #eef2f7; padding: .05rem .3rem; border-radius: 999px; margin-left: .25rem; }
.hist-cell { text-align: center; font-family: monospace; font-size: .68rem; color: #475569; }
.td-cell { text-align: center; }
.td-today { background: #f0fdf9; }
.td-past { background: #fafafa; }

.cell-line { font-family: monospace; font-weight: 600; color: #6b7280; }
.cell-actual { font-family: monospace; font-size: .68rem; }
.cell-tag { display: inline-block; font-size: .56rem; font-weight: 800; color: #b6bdc9; margin-right: .2rem; }
.cell-actual .cell-tag { color: inherit; opacity: .7; }

/* Baris total */
tfoot tr { border-top: 2px solid #e5e7eb; background: #f8fafc; }
.tf-label { padding: .45rem .6rem; font-size: .64rem; font-weight: 800; text-transform: uppercase; letter-spacing: .04em; color: #475569; white-space: nowrap; }
.tf-cell { font-weight: 700; }
.tf-f { color: #374151; }
.tf-a { font-weight: 700; color: #059669; }
.act-ok   { color: #059669; }
.act-warn { color: #d97706; }
.act-bad  { color: #dc2626; }
.txt-manual { color: #1d4ed8; }

.cell-in { width: 58px; text-align: center; font-family: monospace; font-size: .76rem; border: 1px solid #e5e7eb; border-radius: .4rem; padding: .2rem .2rem; }
.cell-in:focus { outline: none; border-color: #34d399; box-shadow: 0 0 0 2px rgba(52,211,153,.2); }
.cell-in--manual { color: #1d4ed8; font-weight: 700; border-color: #bfdbfe; background: #eff6ff; }
.cell-in--dirty { background: #fefce8; border-color: #fde68a; }

.td-acc { text-align: center; }
.acc-pill { font-size: .64rem; font-weight: 700; padding: .15rem .45rem; border-radius: 999px; }
.acc-pill--ok { background: #dcfce7; color: #15803d; }
.acc-pill--warn { background: #fef3c7; color: #d97706; }
</style>
