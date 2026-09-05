<template>
  <div class="space-y-5">
    <div>
      <h1 class="text-xl font-bold text-gray-900">Penyusutan &amp; Nilai Buku</h1>
      <p class="text-sm text-gray-500 mt-0.5">
        Metode garis lurus: (nilai perolehan − nilai residu) ÷ umur ekonomis, diakumulasi per bulan penuh.
        Setiap perolehan disusutkan sejak tanggalnya sendiri — unit yang dibeli belakangan tidak dihitung mundur.
      </p>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Ringkasan — flat, konsisten dengan halaman aset lain -->
    <div class="kpi-grid">
      <div class="kpi">
        <div class="kpi-label">Nilai Perolehan</div>
        <div class="kpi-val">{{ formatRupiah(totals.cost) }}</div>
        <div class="kpi-sub">{{ filtered.length }} aset</div>
      </div>
      <div class="kpi">
        <div class="kpi-label">Nilai Buku</div>
        <div class="kpi-val">{{ formatRupiah(totals.book) }}</div>
        <div v-if="totals.cost" class="meter"><div class="meter-fill" :style="{ width: bookPct + '%' }"></div></div>
        <div class="kpi-sub">{{ bookPct }}% dari nilai perolehan</div>
      </div>
      <div class="kpi">
        <div class="kpi-label">Akumulasi Penyusutan</div>
        <div class="kpi-val">{{ formatRupiah(totals.accum) }}</div>
        <div class="kpi-sub">beban {{ formatRupiah(totals.monthly) }}/bulan</div>
      </div>
      <div class="kpi" :class="notDepreciated > 0 ? 'kpi--warn' : 'kpi--good'">
        <div class="kpi-label">Belum Disusutkan</div>
        <div class="kpi-val">{{ notDepreciated }}</div>
        <div class="kpi-sub">{{ notDepreciated > 0 ? 'aset tanpa umur ekonomis' : 'semua aset sudah diatur' }}</div>
      </div>
    </div>

    <!-- Filters -->
    <AppCard>
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <SearchSelect v-model="filterOutlet" :options="outletFilterOptions" placeholder="Semua outlet" searchPlaceholder="Cari outlet…" @change="load" />
        <select v-model="filterState" class="form-input">
          <option value="">Semua aset</option>
          <option value="depreciating">Sedang disusutkan</option>
          <option value="ending">Sisa umur ≤3 bulan</option>
          <option value="finished">Habis disusutkan</option>
          <option value="none">Tanpa umur ekonomis</option>
        </select>
        <input v-model="search" type="search" placeholder="Cari nama / kode aset…" class="form-input" />
      </div>
      <div v-if="hasActiveFilters" class="filter-summary">
        <span>{{ filtered.length }} aset ditemukan</span>
        <button type="button" class="lnk" @click="resetFilters">Bersihkan filter</button>
      </div>
    </AppCard>

    <!-- List -->
    <AppCard :padding="false">
      <div class="sm:hidden">
        <div v-if="loading" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
        <div v-else-if="!filtered.length" class="empty-block">
          <span class="empty-ic" v-html="ICONS.trendDown"></span>
          <p class="empty-title">{{ hasActiveFilters ? 'Tidak ada aset yang cocok' : 'Belum ada aset tercatat' }}</p>
          <p class="empty-desc">{{ hasActiveFilters ? 'Coba ubah outlet, status, atau kata kunci.' : 'Tambahkan aset dari Daftar Aset untuk melihat penyusutannya di sini.' }}</p>
        </div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="a in filtered" :key="a.id" class="p-4 space-y-1">
            <div class="flex items-start justify-between gap-2">
              <p class="font-semibold text-gray-900 break-words">{{ a.name }}</p>
              <span class="state-badge shrink-0" :class="stateCls(a)">
                <span class="state-ic" v-html="stateIcon(a)"></span>{{ stateLabel(a) }}
              </span>
            </div>
            <p class="text-xs text-gray-500">{{ a.outlet_name }} · perolehan {{ formatRupiah(a.acquisition_cost) }}</p>
            <p class="text-sm text-gray-800">Nilai buku <strong>{{ formatRupiah(a.book_value) }}</strong></p>
            <div class="meter"><div class="meter-fill" :style="{ width: deprecPct(a) + '%' }"></div></div>
            <p class="text-xs text-gray-500">{{ deprecPct(a) }}% tersusut · {{ a.useful_life_months ? `${a.age_months}/${a.useful_life_months} bulan` : 'tanpa umur ekonomis' }}</p>
          </li>
        </ul>
      </div>

      <!-- v-if di sini, bukan hanya emptyText — AppTable selalu merender baris
           "Tidak ada data" bawaannya sendiri saat rows kosong; tanpa v-if,
           blok kosong kustom di bawah akan tampil dobel dengan baris bawaan itu. -->
      <AppTable v-if="loading || filtered.length" class="hidden sm:block" :columns="COLUMNS" :rows="filtered" :loading="loading">
        <template #cell-name="{ row }">
          <div>
            <p class="font-medium text-gray-900">{{ row.name }}</p>
            <p v-if="row.code" class="text-xs text-gray-400 font-mono">{{ row.code }}</p>
          </div>
        </template>
        <template #cell-cost="{ row }">{{ formatRupiah(row.acquisition_cost) }}</template>
        <template #cell-residual="{ row }">{{ row.residual_value > 0 ? formatRupiah(row.residual_value) : '—' }}</template>
        <template #cell-life="{ row }">
          <span v-if="row.useful_life_months">{{ row.age_months }} / {{ row.useful_life_months }} bln</span>
          <span v-else class="text-gray-400">—</span>
          <span v-if="row.useful_life_months" class="text-xs text-gray-400 block">sisa {{ row.remaining_months }} bln</span>
        </template>
        <template #cell-monthly="{ row }">{{ row.monthly_deprec > 0 ? formatRupiah(row.monthly_deprec) : '—' }}</template>
        <template #cell-accum="{ row }">
          <span>{{ formatRupiah(row.accumulated_deprec) }}</span>
          <div class="meter mt-1"><div class="meter-fill" :style="{ width: deprecPct(row) + '%' }"></div></div>
        </template>
        <template #cell-book="{ row }"><strong>{{ formatRupiah(row.book_value) }}</strong></template>
        <template #cell-state="{ row }">
          <span class="state-badge" :class="stateCls(row)">
            <span class="state-ic" v-html="stateIcon(row)"></span>{{ stateLabel(row) }}
          </span>
        </template>
      </AppTable>

      <div v-if="!loading && !filtered.length" class="hidden sm:block empty-block">
        <span class="empty-ic" v-html="ICONS.trendDown"></span>
        <p class="empty-title">{{ hasActiveFilters ? 'Tidak ada aset yang cocok dengan filter' : 'Belum ada aset tercatat' }}</p>
        <p class="empty-desc">{{ hasActiveFilters ? 'Coba ubah outlet, status, atau kata kunci pencarian.' : 'Tambahkan aset dari Daftar Aset untuk melihat penyusutan dan nilai bukunya di sini.' }}</p>
        <button v-if="hasActiveFilters" type="button" class="lnk mt-1" @click="resetFilters">Bersihkan filter</button>
      </div>
    </AppCard>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { assetsApi } from '@/api/assets.js'
import { outletsApi } from '@/api/outlets.js'
import { formatRupiah } from '@/utils/format.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppTable from '@/components/ui/AppTable.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'

// ── Ikon — bahasa visual yang sama dengan halaman aset lain.
const ICONS = {
  trendDown: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M3 6l7 7 4-4 7 7"/><path stroke-linecap="round" stroke-linejoin="round" d="M15 16h6v-6"/></svg>',
  alertTriangle: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v4m0 4h.01M10.29 3.86l-8.18 14.18A1.5 1.5 0 003.5 20.5h17a1.5 1.5 0 001.39-2.46L13.71 3.86a1.5 1.5 0 00-2.42 0z"/></svg>',
  flag: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M5 21V4m0 0h11l-2 4 2 4H5"/></svg>',
  info: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="9"/><path stroke-linecap="round" d="M12 11v5"/><path stroke-linecap="round" d="M12 8h.01"/></svg>',
}

const COLUMNS = [
  { key: 'name',        label: 'Aset' },
  { key: 'outlet_name', label: 'Outlet' },
  { key: 'cost',        label: 'Perolehan' },
  { key: 'residual',    label: 'Residu' },
  { key: 'life',        label: 'Umur' },
  { key: 'monthly',     label: 'Susut/Bulan' },
  { key: 'accum',       label: 'Akumulasi' },
  { key: 'book',        label: 'Nilai Buku' },
  { key: 'state',       label: 'Status' },
]

const rows = ref([])
const outlets = ref([])
const loading = ref(false)
const errorMsg = ref('')
const filterOutlet = ref('')
const filterState = ref('')
const search = ref('')

function asArray(d) { return Array.isArray(d) ? d : (d?.data || []) }
const outletFilterOptions = computed(() => [{ id: '', name: 'Semua outlet' }, ...outlets.value])
const hasActiveFilters = computed(() => !!(filterOutlet.value || filterState.value || search.value.trim()))

function resetFilters() {
  filterOutlet.value = ''; filterState.value = ''; search.value = ''
  load()
}

// Persentase tersusut dihitung terhadap nilai yang boleh disusutkan
// (perolehan − residu), bukan terhadap perolehan penuh — kalau tidak, aset yang
// sudah habis umur ekonomis tak pernah terlihat 100%.
function deprecPct(a) {
  const depreciable = Number(a.acquisition_cost || 0) - Number(a.residual_value || 0)
  if (depreciable <= 0) return 0
  return Math.min(Math.round((Number(a.accumulated_deprec || 0) / depreciable) * 100), 100)
}
function assetState(a) {
  if (!a.useful_life_months) return 'none'
  if (a.remaining_months === 0) return 'finished'
  if (a.remaining_months <= 3) return 'ending'
  return 'depreciating'
}
function stateLabel(a) {
  return { none: 'Tanpa umur', finished: 'Habis disusutkan', ending: 'Akhir umur', depreciating: 'Berjalan' }[assetState(a)]
}
function stateCls(a) {
  const s = assetState(a)
  return { 'state-none': s === 'none', 'state-finished': s === 'finished', 'state-ending': s === 'ending', 'state-run': s === 'depreciating' }
}
function stateIcon(a) {
  return { none: ICONS.info, finished: ICONS.flag, ending: ICONS.alertTriangle, depreciating: ICONS.trendDown }[assetState(a)]
}

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  return rows.value.filter(a => {
    if (filterState.value && assetState(a) !== filterState.value) return false
    if (q && !(`${a.name} ${a.code}`.toLowerCase().includes(q))) return false
    return true
  })
})
const totals = computed(() => filtered.value.reduce((t, a) => ({
  cost: t.cost + Number(a.acquisition_cost || 0),
  accum: t.accum + Number(a.accumulated_deprec || 0),
  book: t.book + Number(a.book_value || 0),
  monthly: t.monthly + Number(a.monthly_deprec || 0),
}), { cost: 0, accum: 0, book: 0, monthly: 0 }))
const bookPct = computed(() => totals.value.cost ? Math.round((totals.value.book / totals.value.cost) * 100) : 0)
const notDepreciated = computed(() => filtered.value.filter(a => !a.useful_life_months).length)

async function load() {
  loading.value = true; errorMsg.value = ''
  try {
    rows.value = asArray(await assetsApi.list({ outlet_id: filterOutlet.value || undefined }))
  } catch (e) {
    errorMsg.value = e?.message || 'Gagal memuat data penyusutan'
  } finally {
    loading.value = false
  }
}
async function loadOutlets() {
  try { const r = await outletsApi.myOutlets(); outlets.value = r?.outlets ?? r ?? [] } catch { outlets.value = [] }
}

onMounted(async () => { await loadOutlets(); await load() })
</script>

<style scoped>
.form-input {
  width: 100%; padding: .5rem .7rem; border-radius: .6rem; font-size: .85rem;
  border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none;
}
.form-input:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }
.lnk { color: #047857; font-weight: 600; text-decoration: none; background: none; border: none; cursor: pointer; font-size: inherit; padding: 0; }
.lnk:hover { text-decoration: underline; }

/* ── Kartu ringkasan — flat ── */
.kpi-grid { display: grid; gap: .75rem; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); }
.kpi { border-radius: .85rem; padding: .85rem 1rem; background: #fff; border: 1px solid rgba(17,24,39,.08); box-shadow: 0 1px 2px rgba(16,24,40,.04); }
.kpi-label { font-size: .68rem; font-weight: 700; color: #6b7280; text-transform: uppercase; letter-spacing: .03em; }
.kpi-val { font-size: 1.3rem; font-weight: 800; color: #111827; line-height: 1.2; margin-top: .2rem; }
.kpi-sub { font-size: .71rem; color: #6b7280; margin-top: .15rem; }
.kpi--good .kpi-val { color: #047857; }
.kpi--warn { border-color: rgba(217,119,6,.3); background: rgba(217,119,6,.04); }
.kpi--warn .kpi-val { color: #b45309; }

.filter-summary { display: flex; align-items: center; justify-content: space-between; gap: .5rem; margin-top: .7rem; padding-top: .7rem; border-top: 1px solid rgba(17,24,39,.06); font-size: .75rem; color: #6b7280; }

.meter { height: .3rem; border-radius: 999px; background: #f1f5f9; overflow: hidden; min-width: 70px; margin: .4rem 0 .15rem; }
.meter-fill { height: 100%; border-radius: 999px; background: #6366f1; }

.state-badge { display: inline-flex; align-items: center; gap: .3rem; padding: .1rem .5rem; border-radius: 999px; font-size: .68rem; font-weight: 700; white-space: nowrap; }
.state-ic { display: inline-flex; }
.state-ic :deep(svg) { width: .72rem; height: .72rem; }
.state-run { background: rgba(16,185,129,.13); color: #047857; }
.state-ending { background: rgba(245,158,11,.15); color: #b45309; }
.state-finished { background: rgba(107,114,128,.14); color: #4b5563; }
.state-none { background: rgba(59,130,246,.12); color: #1d4ed8; }

/* ── Keadaan kosong ── */
.empty-block { padding: 2.5rem 1.5rem; text-align: center; }
.empty-ic { display: inline-flex; width: 2.75rem; height: 2.75rem; border-radius: .9rem; align-items: center; justify-content: center; color: #9ca3af; background: #f3f4f6; margin-bottom: .7rem; }
.empty-ic :deep(svg) { width: 1.4rem; height: 1.4rem; }
.empty-title { font-size: .88rem; font-weight: 700; color: #374151; }
.empty-desc { font-size: .78rem; color: #9ca3af; margin-top: .25rem; max-width: 26rem; margin-inline: auto; line-height: 1.5; }
</style>
