<template>
  <div class="space-y-5">
    <!-- Header -->
    <div class="flex items-start justify-between flex-wrap gap-3">
      <div>
        <h1 class="text-xl font-bold text-gray-900">Dashboard Aset</h1>
        <p class="text-sm text-gray-500 mt-0.5">Nilai, penyusutan, kesehatan perawatan, dan pelepasan aset seluruh outlet.</p>
      </div>
      <div class="flex items-center gap-2">
        <SearchSelect v-model="filterOutlet" :options="outletFilterOptions" placeholder="Semua outlet" searchPlaceholder="Cari outlet…" @change="load" class="min-w-[190px]" />
        <button class="refresh-btn" @click="load" :disabled="loading">
          <svg :class="{ spin: loading }" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg>
          Refresh
        </button>
      </div>
    </div>

    <AppAlert type="error" :message="errorMsg" />
    <div v-if="loading && !d" class="py-12 text-center"><AppSpinner size="lg" class="text-emerald-600" /></div>

    <template v-if="d">
      <!-- ── Nilai ── -->
      <div class="kpi-grid">
        <div class="kpi kpi--slate">
          <div class="kpi-label">Nilai Perolehan</div>
          <div class="kpi-val">{{ formatRupiah(d.acquisition_cost) }}</div>
          <div class="kpi-sub">{{ d.total_assets }} aset · {{ d.total_quantity }} unit</div>
        </div>
        <div class="kpi kpi--blue">
          <div class="kpi-label">Nilai Buku</div>
          <div class="kpi-val">{{ formatRupiah(d.book_value) }}</div>
          <div class="kpi-sub">{{ bookPct }}% dari nilai perolehan</div>
        </div>
        <div class="kpi kpi--slate">
          <div class="kpi-label">Akumulasi Penyusutan</div>
          <div class="kpi-val">{{ formatRupiah(d.accumulated_deprec) }}</div>
          <div class="kpi-sub">beban berjalan {{ formatRupiah(d.monthly_deprec) }}/bulan</div>
        </div>
        <router-link class="kpi kpi--btn" :class="d.ending_soon_count > 0 ? 'kpi--amber' : 'kpi--ok'" to="/aset/penyusutan">
          <div class="kpi-label">Akhir Umur Ekonomis</div>
          <div class="kpi-val">{{ d.ending_soon_count }}</div>
          <div class="kpi-sub">aset dengan sisa umur ≤3 bulan</div>
        </router-link>
      </div>

      <!-- ── Perawatan & pelepasan ── -->
      <div class="kpi-grid">
        <router-link class="kpi kpi--btn" :class="d.overdue > 0 ? 'kpi--red' : 'kpi--ok'" to="/aset/daftar?due=overdue">
          <div class="kpi-label">Perawatan Terlambat</div>
          <div class="kpi-val">{{ d.overdue }}</div>
          <div class="kpi-sub">lewat dari jadwal berikutnya</div>
        </router-link>
        <router-link class="kpi kpi--btn" :class="d.due_soon > 0 ? 'kpi--amber' : 'kpi--ok'" to="/aset/daftar?due=due_soon">
          <div class="kpi-label">Jatuh Tempo ≤{{ d.due_soon_days }} Hari</div>
          <div class="kpi-val">{{ d.due_soon }}</div>
          <div class="kpi-sub">{{ d.unscheduled }} aset belum dijadwalkan</div>
        </router-link>
        <div class="kpi" :class="d.needs_attention > 0 ? 'kpi--blue' : 'kpi--ok'">
          <div class="kpi-label">Biaya Perawatan</div>
          <div class="kpi-val">{{ formatRupiah(d.maint_cost_mtd) }}</div>
          <div class="kpi-sub">bulan ini · {{ formatRupiah(d.maint_cost_12m) }} dalam 12 bulan</div>
        </div>
        <router-link class="kpi kpi--btn" :class="d.gain_loss_ytd < 0 ? 'kpi--red' : 'kpi--slate'" to="/aset/penghapusan">
          <div class="kpi-label">Pelepasan Tahun Ini</div>
          <div class="kpi-val">{{ d.disposed_ytd }}</div>
          <div class="kpi-sub">hasil {{ formatRupiah(d.proceeds_ytd) }} · {{ d.gain_loss_ytd < 0 ? 'rugi' : 'laba' }} {{ formatRupiah(Math.abs(d.gain_loss_ytd)) }}</div>
        </router-link>
      </div>

      <!-- ── Perawatan mendesak ── -->
      <AppCard v-if="d.upcoming_maint.length">
        <h2 class="sec-title">Perawatan Paling Mendesak</h2>
        <ul class="divide-y divide-gray-100">
          <li v-for="a in d.upcoming_maint" :key="a.id" class="py-2.5 flex items-center justify-between gap-3">
            <div class="min-w-0">
              <p class="font-medium text-gray-900 text-sm truncate">{{ a.name }}</p>
              <p class="text-xs text-gray-500">{{ a.outlet_name }}<span v-if="a.location"> · {{ a.location }}</span></p>
            </div>
            <div class="text-right shrink-0">
              <span class="due-badge" :class="a.due_in_days < 0 ? 'due-overdue' : 'due-soon'">
                {{ a.due_in_days < 0 ? `Terlambat ${Math.abs(a.due_in_days)} hari` : (a.due_in_days === 0 ? 'Hari ini' : `${a.due_in_days} hari lagi`) }}
              </span>
              <span class="text-xs text-gray-400 block mt-0.5">{{ formatDateStr(a.next_due_date) }}</span>
            </div>
          </li>
        </ul>
      </AppCard>

      <!-- ── Sebaran ── -->
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <AppCard>
          <h2 class="sec-title">Nilai Buku per Outlet</h2>
          <BarList :rows="d.by_outlet" :formatter="formatRupiah" emptyText="Belum ada aset." />
        </AppCard>
        <AppCard>
          <h2 class="sec-title">Nilai Buku per Kategori</h2>
          <BarList :rows="d.by_category" :formatter="formatRupiah" emptyText="Belum ada kategori." />
        </AppCard>
        <AppCard>
          <h2 class="sec-title">Kondisi Aset</h2>
          <BarList :rows="conditionRows" :formatter="(v) => `${v} aset`" valueKey="count" emptyText="Belum ada aset." />
        </AppCard>
      </div>

      <!-- ── Tren 12 bulan ── -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <AppCard>
          <h2 class="sec-title">Perolehan 12 Bulan Terakhir</h2>
          <MonthlyBars :rows="d.acquisition_trend" color="#6366f1" />
        </AppCard>
        <AppCard>
          <h2 class="sec-title">Biaya Perawatan 12 Bulan Terakhir</h2>
          <MonthlyBars :rows="d.maintenance_trend" color="#f59e0b" />
        </AppCard>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, h } from 'vue'
import { assetsApi } from '@/api/assets.js'
import { outletsApi } from '@/api/outlets.js'
import { formatRupiah, formatDateStr } from '@/utils/format.js'
import { useRealtime } from '@/utils/realtime.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppSpinner from '@/components/ui/AppSpinner.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'

const CONDITIONS = { baik: 'Baik', rusak_ringan: 'Rusak Ringan', rusak_berat: 'Rusak Berat', perbaikan: 'Dalam Perbaikan' }

// Dua komponen ringan didefinisikan lokal — dipakai hanya di halaman ini, jadi
// tidak perlu menambah berkas komponen global.
const BarList = {
  props: { rows: Array, formatter: Function, valueKey: { type: String, default: 'value' }, emptyText: String },
  setup(props) {
    return () => {
      const rows = props.rows || []
      if (!rows.length) return h('p', { class: 'text-sm text-gray-400 py-3' }, props.emptyText)
      const max = Math.max(...rows.map(r => Number(r[props.valueKey]) || 0), 1)
      return h('ul', { class: 'space-y-2.5' }, rows.map(r => h('li', { key: r.key }, [
        h('div', { class: 'flex items-baseline justify-between gap-2 mb-1' }, [
          h('span', { class: 'text-sm text-gray-700 truncate' }, r.label),
          h('span', { class: 'text-xs font-semibold text-gray-900 shrink-0' }, props.formatter(Number(r[props.valueKey]) || 0)),
        ]),
        h('div', { class: 'h-1.5 rounded-full bg-gray-100 overflow-hidden' }, [
          h('div', { class: 'h-full rounded-full bg-emerald-500', style: { width: `${((Number(r[props.valueKey]) || 0) / max) * 100}%` } }),
        ]),
      ])))
    }
  },
}

const MonthlyBars = {
  props: { rows: Array, color: String },
  setup(props) {
    return () => {
      const rows = props.rows || []
      if (!rows.length) return h('p', { class: 'text-sm text-gray-400 py-3' }, 'Belum ada data pada rentang ini.')
      const max = Math.max(...rows.map(r => Number(r.value) || 0), 1)
      return h('div', { class: 'flex items-end gap-1.5 h-32 pt-2' }, rows.map(r => h('div', {
        key: r.month,
        class: 'flex-1 flex flex-col items-center gap-1 min-w-0',
        title: `${r.month}: ${formatRupiah(r.value)} (${r.count}×)`,
      }, [
        h('div', { class: 'w-full rounded-t', style: { height: `${Math.max(((Number(r.value) || 0) / max) * 100, 2)}%`, background: props.color } }),
        h('span', { class: 'text-[9px] text-gray-400 truncate w-full text-center' }, r.month.slice(5)),
      ])))
    }
  },
}

const d = ref(null)
const outlets = ref([])
const loading = ref(false)
const errorMsg = ref('')
const filterOutlet = ref('')

const outletFilterOptions = computed(() => [{ id: '', name: 'Semua outlet' }, ...outlets.value])
const bookPct = computed(() => {
  if (!d.value?.acquisition_cost) return 0
  return Math.round((d.value.book_value / d.value.acquisition_cost) * 100)
})
const conditionRows = computed(() =>
  (d.value?.condition_breakdown || []).map(b => ({ ...b, label: CONDITIONS[b.key] || b.label })))

async function load() {
  loading.value = true; errorMsg.value = ''
  try {
    d.value = await assetsApi.dashboard({ outlet_id: filterOutlet.value || undefined })
  } catch (e) {
    errorMsg.value = e?.message || 'Gagal memuat dashboard aset'
  } finally {
    loading.value = false
  }
}

async function loadOutlets() {
  try {
    const r = await outletsApi.myOutlets()
    outlets.value = r?.outlets ?? r ?? []
  } catch { outlets.value = [] }
}

useRealtime(['asset_alert'], load)
onMounted(async () => { await loadOutlets(); await load() })
</script>

<style scoped>
.sec-title { font-size: .8rem; font-weight: 800; color: #374151; text-transform: uppercase; letter-spacing: .03em; margin-bottom: .6rem; }
.refresh-btn {
  display: inline-flex; align-items: center; gap: .4rem; padding: .45rem .8rem; border-radius: .6rem;
  font-size: .78rem; font-weight: 600; color: #374151; background: #fff; border: 1px solid rgba(0,0,0,.1);
}
.refresh-btn:hover { background: #f9fafb; }
.refresh-btn:disabled { opacity: .55; }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

.kpi-grid { display: grid; gap: .75rem; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); }
.kpi {
  border-radius: .85rem; padding: .85rem 1rem; background: #fff; text-align: left; display: block;
  border: 1px solid rgba(0,0,0,.07); box-shadow: 0 1px 2px rgba(16,24,40,.04);
}
.kpi--btn { transition: transform .12s ease, box-shadow .12s ease; }
.kpi--btn:hover { transform: translateY(-1px); box-shadow: 0 4px 12px rgba(16,24,40,.09); }
.kpi-label { font-size: .7rem; font-weight: 700; color: #6b7280; text-transform: uppercase; letter-spacing: .02em; }
.kpi-val { font-size: 1.4rem; font-weight: 800; color: #111827; line-height: 1.2; margin-top: .15rem; }
.kpi-sub { font-size: .7rem; color: #6b7280; margin-top: .1rem; }
.kpi--slate { background: linear-gradient(180deg, #fff, #f8fafc); }
.kpi--ok    { border-color: rgba(16,185,129,.25); background: linear-gradient(180deg, #fff, rgba(16,185,129,.05)); }
.kpi--red   { border-color: rgba(239,68,68,.3);  background: linear-gradient(180deg, #fff, rgba(239,68,68,.07)); }
.kpi--red .kpi-val { color: #b91c1c; }
.kpi--amber { border-color: rgba(245,158,11,.35); background: linear-gradient(180deg, #fff, rgba(245,158,11,.08)); }
.kpi--amber .kpi-val { color: #b45309; }
.kpi--blue  { border-color: rgba(59,130,246,.3); background: linear-gradient(180deg, #fff, rgba(59,130,246,.06)); }
.kpi--blue .kpi-val { color: #1d4ed8; }

.due-badge { display: inline-block; padding: .12rem .5rem; border-radius: 999px; font-size: .68rem; font-weight: 700; white-space: nowrap; }
.due-overdue { background: rgba(239,68,68,.13); color: #b91c1c; }
.due-soon { background: rgba(245,158,11,.15); color: #b45309; }
</style>
