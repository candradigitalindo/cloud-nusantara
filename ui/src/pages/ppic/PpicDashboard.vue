<template>
  <div class="pd">

    <!-- ── Header ── -->
    <div class="pd-hd">
      <div>
        <h1 class="pd-title">Dashboard PPIC</h1>
        <p class="pd-sub">Pengendalian stok, risiko kedaluwarsa, dan sinyal pesan ulang — standar F&amp;B.</p>
      </div>
      <button class="pd-refresh" @click="load" :disabled="loading">
        <svg :class="{ spin: loading }" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg>
        Refresh
      </button>
    </div>

    <AppAlert type="error" :message="err" />
    <div v-if="loading && !d" class="pd-loading"><AppSpinner size="lg" class="text-emerald-600"/></div>

    <template v-if="d">

      <!-- ── KPI Cards ── -->
      <div class="kpi-grid">
        <router-link class="kpi kpi--purple" to="/stock-ledger">
          <div class="kpi-label">Nilai Stok</div>
          <div class="kpi-val">{{ fmtRp(d.total_stock_value) }}</div>
          <div class="kpi-sub">Turnover 30 hari: {{ (d.turnover_ratio_30d || 0).toFixed(2) }}×</div>
        </router-link>

        <router-link class="kpi" :class="d.below_rop_count > 0 ? 'kpi--amber' : 'kpi--ok'" to="/ppic/planning-params?below=1">
          <div class="kpi-label">Di Bawah Titik Pesan</div>
          <div class="kpi-val">{{ d.below_rop_count }}</div>
          <div class="kpi-sub">item menyentuh ROP / stok minimum</div>
        </router-link>

        <router-link class="kpi" :class="d.out_of_stock_count > 0 ? 'kpi--red' : 'kpi--ok'" to="/ppic/planning-params?below=1">
          <div class="kpi-label">Stok Habis</div>
          <div class="kpi-val">{{ d.out_of_stock_count }}</div>
          <div class="kpi-sub">item ber-ambang yang kosong</div>
        </router-link>

        <div class="kpi kpi--blue">
          <div class="kpi-label">Coverage Stok</div>
          <div class="kpi-val">{{ d.coverage_days ? d.coverage_days.toFixed(1) : '—' }} <span class="kpi-unit">hari</span></div>
          <div class="kpi-sub">nilai stok ÷ pemakaian harian (28 hr)</div>
        </div>

        <router-link class="kpi" :class="d.expired_count > 0 ? 'kpi--red' : (d.expiring_7d_count > 0 ? 'kpi--amber' : 'kpi--ok')" to="/ppic/expiry">
          <div class="kpi-label">Risiko Kedaluwarsa</div>
          <div class="kpi-val">{{ fmtRp(d.expired_value + d.expiring_7d_value) }}</div>
          <div class="kpi-sub">{{ d.expired_count }} batch lewat · {{ d.expiring_7d_count }} batch ≤7 hari</div>
        </router-link>

        <router-link class="kpi" :class="wasteClass" to="/stock-wastes">
          <div class="kpi-label">Waste 30 Hari</div>
          <div class="kpi-val">{{ (d.waste_pct_30d || 0).toFixed(1) }}%</div>
          <div class="kpi-sub">{{ fmtRp(d.waste_value_30d) }} dari pemakaian {{ fmtRp(d.usage_value_30d) }}</div>
        </router-link>

        <div class="kpi" :class="d.dead_stock_count > 0 ? 'kpi--gray' : 'kpi--ok'">
          <div class="kpi-label">Dead Stock</div>
          <div class="kpi-val">{{ d.dead_stock_count }}</div>
          <div class="kpi-sub">{{ fmtRp(d.dead_stock_value) }} tanpa gerak &gt;30 hari</div>
        </div>

        <router-link class="kpi" :class="opnameClass" to="/ppic/opname">
          <div class="kpi-label">Akurasi Opname (IRA)</div>
          <div class="kpi-val">{{ d.last_opname ? d.last_opname.accuracy_pct.toFixed(1) + '%' : '—' }}</div>
          <div class="kpi-sub">{{ d.last_opname ? d.last_opname.opname_number + ' · ' + d.last_opname.warehouse_name : 'belum ada sesi opname' }}</div>
        </router-link>

        <router-link class="kpi" :class="forecastClass" to="/ppic/forecast">
          <div class="kpi-label">Akurasi Forecast (7 hr)</div>
          <div class="kpi-val">{{ d.forecast_eval_rows ? d.forecast_accuracy_7d.toFixed(1) + '%' : '—' }}</div>
          <div class="kpi-sub">{{ d.forecast_eval_rows ? d.forecast_eval_rows + ' titik evaluasi · target ≥ 80%' : 'belum ada forecast dievaluasi' }}</div>
        </router-link>
      </div>

      <!-- ── Row: Alert + Trend ── -->
      <div class="row2">
        <div class="card">
          <div class="card-hd">
            <span class="card-title">🔔 Pusat Peringatan</span>
            <span class="alert-count" v-if="d.alerts.length">{{ d.alerts.length }}</span>
          </div>
          <div v-if="!d.alerts.length" class="empty-sm ok">Tidak ada peringatan — semua terkendali ✓</div>
          <div v-else class="alert-list">
            <router-link v-for="(a, i) in d.alerts" :key="i" class="alert-row" :class="`alert--${a.severity}`" :to="alertLink(a)">
              <span class="alert-dot" :class="`dot--${a.severity}`"></span>
              <div class="alert-body">
                <div class="alert-main">
                  <span class="alert-item">{{ a.item_name }}</span>
                  <span class="alert-wh">{{ a.warehouse_name }}</span>
                </div>
                <div class="alert-msg">
                  {{ a.message }}
                  <template v-if="a.date"> · {{ fmtDate(a.date) }}</template>
                  <template v-if="a.qty"> · {{ fmtQty(a.qty) }} {{ a.unit }}</template>
                  <template v-if="a.value"> · {{ fmtRp(a.value) }}</template>
                </div>
              </div>
            </router-link>
          </div>
        </div>

        <div class="card">
          <div class="card-hd"><span class="card-title">Nilai Stok Masuk vs Keluar — 30 Hari</span></div>
          <div v-if="noTrend" class="empty-sm">Belum ada pergerakan stok</div>
          <div v-else class="trend-wrap">
            <div class="trend-legend">
              <span class="dot dot--in"></span><span>Masuk</span>
              <span class="dot dot--out" style="margin-left:.75rem"></span><span>Keluar (pemakaian)</span>
            </div>
            <div class="trend-bars">
              <div v-for="p in d.usage_trend" :key="p.date" class="trend-col">
                <div class="bar-wrap">
                  <div class="bar bar--out" :style="{ height: barH(p.out_value) + 'px' }" :title="`${fmtDay(p.date)} keluar ${fmtRp(p.out_value)}`"></div>
                  <div class="bar bar--in" :style="{ height: barH(p.in_value) + 'px' }" :title="`${fmtDay(p.date)} masuk ${fmtRp(p.in_value)}`"></div>
                </div>
                <div class="bar-label" v-if="showLabel(p.date)">{{ fmtDay(p.date) }}</div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- ── Row: Kalender expiry + Top waste ── -->
      <div class="row2">
        <div class="card">
          <div class="card-hd">
            <span class="card-title">Nilai Kedaluwarsa 30 Hari ke Depan</span>
            <router-link to="/ppic/expiry" class="card-link">Monitor FEFO →</router-link>
          </div>
          <div v-if="!d.expiry_calendar.length" class="empty-sm ok">Tidak ada batch kedaluwarsa dalam 30 hari ✓</div>
          <div v-else class="exp-cal">
            <div v-for="p in d.expiry_calendar" :key="p.date" class="exp-day" :class="expDayClass(p.date)"
              :title="`${fmtDate(p.date)}: ${p.count} batch · ${fmtRp(p.value)}`">
              <div class="exp-bar" :style="{ height: expH(p.value) + 'px' }"></div>
              <div class="exp-label">{{ fmtDay(p.date) }}</div>
              <div class="exp-val">{{ fmtRpShort(p.value) }}</div>
            </div>
          </div>
        </div>

        <div class="card">
          <div class="card-hd">
            <span class="card-title">Top Waste 30 Hari</span>
            <router-link to="/stock-wastes" class="card-link">Lihat Semua →</router-link>
          </div>
          <div v-if="!d.top_waste.length" class="empty-sm ok">Tidak ada waste 30 hari terakhir ✓</div>
          <table v-else class="mini-table">
            <thead><tr><th>Item</th><th class="th-r">Qty</th><th class="th-r">Nilai</th><th></th></tr></thead>
            <tbody>
              <tr v-for="w in d.top_waste" :key="w.item_id">
                <td class="td-name">{{ w.item_name }}</td>
                <td class="td-num">{{ fmtQty(w.total_qty) }} {{ w.base_unit }}</td>
                <td class="td-num txt-red">{{ fmtRp(w.value) }}</td>
                <td class="td-bar"><div class="wbar" :style="{ width: wasteW(w.value) + '%' }"></div></td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- ── Ringkasan per gudang ── -->
      <div class="card">
        <div class="card-hd"><span class="card-title">Ringkasan per Gudang</span></div>
        <div class="overflow-x-auto">
          <table class="mini-table">
            <thead>
              <tr><th>Gudang</th><th>Outlet</th><th class="th-r">Nilai Stok</th><th class="th-r">Di Bawah ROP</th><th class="th-r">Nilai Risiko Expiry ≤7 hr</th></tr>
            </thead>
            <tbody>
              <tr v-for="w in d.warehouse_rows" :key="w.warehouse_id">
                <td class="td-name">
                  <span class="wh-badge" :class="w.warehouse_type === 'central' ? 'wh-badge--central' : 'wh-badge--outlet'">
                    {{ w.warehouse_type === 'central' ? 'Induk' : 'Outlet' }}
                  </span>
                  {{ w.warehouse_name }}
                </td>
                <td class="td-wh">{{ w.outlet_name }}</td>
                <td class="td-num">{{ fmtRp(w.stock_value) }}</td>
                <td class="td-num" :class="w.below_rop_count > 0 ? 'txt-amber' : ''">{{ w.below_rop_count }}</td>
                <td class="td-num" :class="w.expiring_value > 0 ? 'txt-red' : ''">{{ fmtRp(w.expiring_value) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ppicApi } from '@/api/ppic'
import { useRealtime } from '@/utils/realtime'
import AppAlert   from '@/components/ui/AppAlert.vue'
import AppSpinner from '@/components/ui/AppSpinner.vue'

const d       = ref(null)
const loading = ref(false)
const err     = ref('')

onMounted(load)
useRealtime(['ppic_alert'], load)

async function load() {
  loading.value = true; err.value = ''
  try {
    d.value = await ppicApi.getDashboard()
  } catch (e) {
    err.value = e?.message ?? 'Gagal memuat dashboard PPIC'
  } finally {
    loading.value = false
  }
}

const wasteClass = computed(() => {
  const p = d.value?.waste_pct_30d ?? 0
  if (p > 5) return 'kpi--red'
  if (p > 2) return 'kpi--amber'
  return 'kpi--ok'
})
const opnameClass = computed(() => {
  const o = d.value?.last_opname
  if (!o) return 'kpi--gray'
  return o.accuracy_pct >= 97 ? 'kpi--ok' : 'kpi--amber'
})
const forecastClass = computed(() => {
  if (!d.value?.forecast_eval_rows) return 'kpi--gray'
  return d.value.forecast_accuracy_7d >= 80 ? 'kpi--ok' : 'kpi--amber'
})

function alertLink(a) {
  if (a.type === 'expired' || a.type === 'expiring') return '/ppic/expiry'
  return '/ppic/planning-params?below=1'
}

const noTrend = computed(() => !d.value?.usage_trend?.some(p => p.in_value > 0 || p.out_value > 0))
const maxTrend = computed(() => {
  if (!d.value?.usage_trend?.length) return 1
  return Math.max(1, ...d.value.usage_trend.map(p => Math.max(p.in_value, p.out_value)))
})
function barH(v) { return Math.max(2, Math.round((v / maxTrend.value) * 80)) }
function showLabel(date) { return new Date(date).getDate() % 5 === 0 }

const maxExp = computed(() => {
  if (!d.value?.expiry_calendar?.length) return 1
  return Math.max(1, ...d.value.expiry_calendar.map(p => p.value))
})
function expH(v) { return Math.max(4, Math.round((v / maxExp.value) * 56)) }
function expDayClass(date) {
  const days = Math.round((new Date(date) - new Date().setHours(0, 0, 0, 0)) / 86400000)
  if (days <= 3) return 'exp--red'
  if (days <= 7) return 'exp--amber'
  return 'exp--ok'
}

const maxWaste = computed(() => Math.max(1, ...(d.value?.top_waste ?? []).map(w => w.value)))
function wasteW(v) { return Math.max(3, Math.round((v / maxWaste.value) * 100)) }

function fmtRp(v) {
  if (!v) return 'Rp 0'
  if (v >= 1_000_000_000) return 'Rp ' + (v / 1_000_000_000).toFixed(1) + ' M'
  if (v >= 1_000_000)     return 'Rp ' + (v / 1_000_000).toFixed(1) + ' jt'
  return 'Rp ' + Math.round(v).toLocaleString('id-ID')
}
function fmtRpShort(v) {
  if (!v) return ''
  if (v >= 1_000_000) return (v / 1_000_000).toFixed(1) + 'jt'
  if (v >= 1_000)     return Math.round(v / 1_000) + 'rb'
  return Math.round(v)
}
function fmtQty(v)  { return Number(v ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 2 }) }
function fmtDay(s)  { return new Date(s).toLocaleDateString('id-ID', { day: '2-digit', month: 'short' }) }
function fmtDate(s) { return s ? new Date(s).toLocaleDateString('id-ID', { day: '2-digit', month: 'short', year: 'numeric' }) : '—' }
</script>

<style scoped>
.pd { display: flex; flex-direction: column; gap: 1.25rem; }

.pd-hd { display: flex; align-items: flex-start; justify-content: space-between; gap: 1rem; }
.pd-title { font-size: 1.35rem; font-weight: 800; color: #111827; margin: 0; }
.pd-sub { font-size: .8rem; color: #6b7280; margin: .2rem 0 0; }
.pd-loading { display: flex; justify-content: center; padding: 4rem; }
.pd-refresh {
  display: inline-flex; align-items: center; gap: .4rem;
  font-size: .78rem; font-weight: 600; padding: .5rem .9rem;
  border: 1.5px solid #a7f3d0; border-radius: .55rem;
  background: #ecfdf5; color: #065f46; cursor: pointer; transition: all .15s;
}
.pd-refresh:hover { background: #d1fae5; }
.pd-refresh:disabled { opacity: .5; cursor: not-allowed; }
.spin { animation: spin .7s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

/* KPI */
.kpi-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: .75rem; }
@media (max-width: 1100px) { .kpi-grid { grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 520px)  { .kpi-grid { grid-template-columns: 1fr; } }

.kpi { display: block; padding: .9rem 1rem; border-radius: .875rem; border: 1.5px solid; text-decoration: none; transition: transform .12s; }
.kpi:hover { transform: translateY(-1px); }
.kpi--ok     { background: #ecfdf5; border-color: #a7f3d0; }
.kpi--amber  { background: #fffbeb; border-color: #fde68a; }
.kpi--red    { background: #fef2f2; border-color: #fecaca; }
.kpi--blue   { background: #eff6ff; border-color: #bfdbfe; }
.kpi--purple { background: #faf5ff; border-color: #e9d5ff; }
.kpi--gray   { background: #f9fafb; border-color: #e5e7eb; }

.kpi-label { font-size: .68rem; font-weight: 700; text-transform: uppercase; letter-spacing: .05em; color: #6b7280; }
.kpi-val   { font-size: 1.35rem; font-weight: 800; color: #111827; line-height: 1.15; margin-top: .25rem; }
.kpi-unit  { font-size: .8rem; font-weight: 600; color: #6b7280; }
.kpi-sub   { font-size: .68rem; color: #9ca3af; margin-top: .2rem; }

/* Layout */
.row2 { display: grid; grid-template-columns: 1fr 1fr; gap: .75rem; }
@media (max-width: 900px) { .row2 { grid-template-columns: 1fr; } }
.card { background: #fff; border-radius: .875rem; border: 1.5px solid #e5e7eb; padding: 1.1rem; box-shadow: 0 1px 3px rgba(0,0,0,.04); }
.card-hd { display: flex; align-items: center; justify-content: space-between; margin-bottom: .9rem; gap: .5rem; }
.card-title { font-size: .85rem; font-weight: 700; color: #111827; }
.card-link { font-size: .72rem; font-weight: 600; color: #059669; text-decoration: none; white-space: nowrap; }
.card-link:hover { text-decoration: underline; }
.empty-sm { display: flex; align-items: center; justify-content: center; gap: .4rem; padding: 1.75rem; font-size: .8rem; color: #9ca3af; }
.empty-sm.ok { color: #059669; }

/* Alerts */
.alert-count { font-size: .66rem; font-weight: 700; background: #fee2e2; color: #dc2626; padding: .1rem .5rem; border-radius: 999px; }
.alert-list { display: flex; flex-direction: column; gap: .4rem; max-height: 330px; overflow-y: auto; }
.alert-row { display: flex; gap: .55rem; padding: .5rem .6rem; border-radius: .5rem; border: 1px solid; text-decoration: none; }
.alert--red   { background: #fef2f2; border-color: #fecaca; }
.alert--amber { background: #fffbeb; border-color: #fde68a; }
.alert--gray  { background: #f9fafb; border-color: #e5e7eb; }
.alert-dot { width: 8px; height: 8px; border-radius: 50%; margin-top: .35rem; flex-shrink: 0; }
.dot--red   { background: #dc2626; }
.dot--amber { background: #d97706; }
.dot--gray  { background: #9ca3af; }
.alert-main { display: flex; gap: .5rem; align-items: baseline; flex-wrap: wrap; }
.alert-item { font-size: .78rem; font-weight: 700; color: #111827; }
.alert-wh   { font-size: .66rem; color: #6b7280; }
.alert-msg  { font-size: .7rem; color: #6b7280; margin-top: .05rem; }

/* Trend */
.trend-legend { display: flex; align-items: center; gap: .3rem; font-size: .7rem; color: #6b7280; margin-bottom: .7rem; }
.dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.dot--in  { background: #10b981; }
.dot--out { background: #f43f5e; }
.trend-bars { display: flex; align-items: flex-end; gap: 2px; height: 100px; overflow-x: auto; padding-bottom: 1.25rem; position: relative; }
.trend-col { display: flex; flex-direction: column; align-items: center; flex: 1; min-width: 10px; }
.bar-wrap { display: flex; gap: 1px; align-items: flex-end; }
.bar { width: 5px; border-radius: 2px 2px 0 0; min-height: 2px; }
.bar--in  { background: #10b981; }
.bar--out { background: #f43f5e; }
.bar-label { font-size: .55rem; color: #9ca3af; position: absolute; bottom: 0; white-space: nowrap; }

/* Expiry calendar */
.exp-cal { display: flex; align-items: flex-end; gap: .35rem; overflow-x: auto; padding-bottom: .25rem; }
.exp-day { display: flex; flex-direction: column; align-items: center; gap: .2rem; min-width: 40px; }
.exp-bar { width: 22px; border-radius: 3px 3px 0 0; }
.exp--red  .exp-bar { background: #ef4444; }
.exp--amber .exp-bar { background: #f59e0b; }
.exp--ok   .exp-bar { background: #a7f3d0; }
.exp-label { font-size: .58rem; color: #6b7280; white-space: nowrap; }
.exp-val { font-size: .58rem; font-weight: 700; color: #374151; }

/* Tables */
.mini-table { width: 100%; border-collapse: collapse; font-size: .75rem; }
.mini-table thead tr { background: #f9fafb; }
.mini-table th { padding: .4rem .6rem; text-align: left; font-size: .65rem; font-weight: 700; text-transform: uppercase; letter-spacing: .04em; color: #9ca3af; border-bottom: 1px solid #f3f4f6; white-space: nowrap; }
.mini-table .th-r { text-align: right; }
.mini-table tbody tr { border-bottom: 1px solid #f9fafb; }
.mini-table tbody tr:last-child { border-bottom: none; }
.mini-table tbody tr:hover { background: #f9fafb; }
.mini-table td { padding: .45rem .6rem; color: #374151; }
.td-name { font-weight: 600; }
.td-wh { color: #6b7280; }
.td-num { text-align: right; font-weight: 600; font-family: monospace; white-space: nowrap; }
.td-bar { width: 90px; }
.wbar { height: 8px; border-radius: 999px; background: #fca5a5; }
.txt-red { color: #dc2626; }
.txt-amber { color: #d97706; }
.wh-badge { font-size: .6rem; font-weight: 700; padding: .12rem .4rem; border-radius: .3rem; margin-right: .35rem; }
.wh-badge--central { background: #ede9fe; color: #6d28d9; }
.wh-badge--outlet  { background: #d1fae5; color: #065f46; }
.overflow-x-auto { overflow-x: auto; }
</style>
