<template>
  <div class="space-y-6">

    <!-- Filter Bar -->
    <AppCard>
      <div class="flex flex-wrap items-end gap-4">
        <div class="flex flex-col gap-1">
          <label class="text-sm font-medium text-gray-700">Rentang Tanggal</label>
          <DateRangePicker v-model="range" />
        </div>
        <div class="flex flex-col gap-1 min-w-45">
          <label class="text-sm font-medium text-gray-700">Outlet</label>
          <SearchSelect
            v-model="selectedOutlet"
            :options="outletOptions"
            placeholder="Semua Outlet"
            searchPlaceholder="Cari outlet..."
            valueKey="value"
            labelKey="label"
          />
        </div>
        <button @click="page = 1; fetchReport()"
          class="px-4 py-2 bg-emerald-600 text-white text-sm font-medium rounded-lg hover:bg-emerald-700 transition-colors shadow-sm">
          Tampilkan
        </button>
      </div>
    </AppCard>

    <AppAlert type="error" :message="errorMsg" />

    <div v-if="loading" class="flex justify-center py-12">
      <AppSpinner size="lg" />
    </div>

    <template v-if="!loading && report">

      <!-- Summary Cards -->
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <AppCard>
          <div class="flex items-center gap-3">
            <div class="w-10 h-10 rounded-xl bg-red-100 flex items-center justify-center">
              <svg class="w-5 h-5 text-red-600" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636"/></svg>
            </div>
            <div>
              <p class="text-xs text-gray-500 font-medium uppercase tracking-wide">Void Transaksi</p>
              <p class="text-2xl font-bold text-gray-900">{{ report.summary.total_voided }}</p>
            </div>
          </div>
        </AppCard>
        <AppCard>
          <div class="flex items-center gap-3">
            <div class="w-10 h-10 rounded-xl bg-orange-100 flex items-center justify-center">
              <svg class="w-5 h-5 text-orange-600" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
            </div>
            <div>
              <p class="text-xs text-gray-500 font-medium uppercase tracking-wide">Nilai Void Transaksi</p>
              <p class="text-2xl font-bold text-gray-900">{{ formatRupiah(report.summary.total_amount) }}</p>
            </div>
          </div>
        </AppCard>
        <AppCard>
          <div class="flex items-center gap-3">
            <div class="w-10 h-10 rounded-xl bg-rose-100 flex items-center justify-center">
              <svg class="w-5 h-5 text-rose-600" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>
            </div>
            <div>
              <p class="text-xs text-gray-500 font-medium uppercase tracking-wide">Void Item</p>
              <p class="text-2xl font-bold text-gray-900">{{ report.summary.item_voided }}</p>
            </div>
          </div>
        </AppCard>
        <AppCard>
          <div class="flex items-center gap-3">
            <div class="w-10 h-10 rounded-xl bg-amber-100 flex items-center justify-center">
              <svg class="w-5 h-5 text-amber-600" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
            </div>
            <div>
              <p class="text-xs text-gray-500 font-medium uppercase tracking-wide">Nilai Void Item</p>
              <p class="text-2xl font-bold text-gray-900">{{ formatRupiah(report.summary.item_amount) }}</p>
            </div>
          </div>
        </AppCard>
      </div>

      <!-- Tab: Void Transaksi vs Void Item -->
      <div class="flex gap-2">
        <button
          class="px-4 py-2 text-sm font-medium rounded-lg transition-colors"
          :class="activeTab === 'order' ? 'bg-emerald-600 text-white shadow-sm' : 'bg-white border border-gray-200 text-gray-600 hover:bg-gray-50'"
          @click="switchTab('order')">
          Void Transaksi ({{ report.total }})
        </button>
        <button
          class="px-4 py-2 text-sm font-medium rounded-lg transition-colors"
          :class="activeTab === 'item' ? 'bg-emerald-600 text-white shadow-sm' : 'bg-white border border-gray-200 text-gray-600 hover:bg-gray-50'"
          @click="switchTab('item')">
          Void Item ({{ report.items_total }})
        </button>
      </div>

      <!-- Table: Void Transaksi -->
      <AppCard v-if="activeTab === 'order'">
        <div v-if="report.data.length === 0" class="text-center py-12 text-gray-400">
          <svg class="w-12 h-12 mx-auto mb-3 opacity-40" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"/></svg>
          <p class="font-medium">Tidak ada order void pada periode ini</p>
        </div>

        <div v-else class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-gray-200 bg-gray-50/70">
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Waktu Void</th>
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Outlet</th>
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Order</th>
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Item</th>
                <th class="text-right py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Total</th>
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Petugas & Alasan</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100">
              <tr v-for="row in report.data" :key="row.id" class="hover:bg-red-50/40 transition-colors">
                <td class="py-3 px-3 whitespace-nowrap align-top">
                  <p class="font-semibold text-gray-900 tabular-nums">{{ fmtTime(row.voided_at) }}</p>
                  <p class="text-xs text-gray-400">{{ fmtDateShort(row.voided_at) }}</p>
                  <p v-if="lifeSpan(row.created_at, row.voided_at)" class="text-[11px] text-amber-600 mt-0.5" title="Selang waktu order dibuat sampai di-void">⏱ {{ lifeSpan(row.created_at, row.voided_at) }}</p>
                </td>
                <td class="py-3 px-3 align-top">
                  <span class="vr-pill"><span class="vr-dot" />{{ row.outlet_name || '—' }}</span>
                </td>
                <td class="py-3 px-3 align-top">
                  <p class="font-medium text-gray-800">{{ row.customer_name || 'Tanpa nama' }}</p>
                  <span v-if="row.table_number" class="vr-meja">Meja {{ row.table_number }}</span>
                </td>
                <td class="py-3 px-3 align-top max-w-[16rem]">
                  <div class="flex flex-wrap gap-1">
                    <span v-for="item in parseItems(row.items).slice(0, 3)" :key="item.product_name"
                      class="vr-chip">{{ item.qty }}× {{ item.product_name }}</span>
                    <span v-if="parseItems(row.items).length > 3" class="vr-chip vr-chip-more"
                      :title="parseItems(row.items).map(i => `${i.qty}× ${i.product_name}`).join('\n')">
                      +{{ parseItems(row.items).length - 3 }} lagi
                    </span>
                  </div>
                  <p class="text-[11px] text-gray-400 mt-1">{{ itemsSummary(row.items) }}</p>
                </td>
                <td class="py-3 px-3 text-right align-top">
                  <span class="font-bold text-red-600 tabular-nums">{{ formatRupiah(row.total_amount) }}</span>
                </td>
                <td class="py-3 px-3 align-top">
                  <div class="flex items-start gap-2">
                    <span class="vr-avatar">{{ initials(row.voided_by) }}</span>
                    <div class="min-w-0">
                      <p class="font-medium text-gray-800 leading-tight">{{ row.voided_by || '—' }}</p>
                      <p class="text-xs text-gray-500 italic break-words">"{{ row.void_reason || 'Dibatalkan kasir' }}"</p>
                    </div>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Pagination -->
        <div v-if="report.total > report.limit" class="flex items-center justify-between pt-4 border-t border-gray-100 mt-4">
          <p class="text-sm text-gray-500">
            Menampilkan {{ (page-1)*report.limit + 1 }}–{{ Math.min(page*report.limit, report.total) }} dari {{ report.total }} data
          </p>
          <div class="flex gap-2">
            <button :disabled="page <= 1"
              class="px-3 py-1.5 text-sm border rounded-lg disabled:opacity-40 hover:bg-gray-50"
              @click="page--; fetchReport()">← Prev</button>
            <button :disabled="page * report.limit >= report.total"
              class="px-3 py-1.5 text-sm border rounded-lg disabled:opacity-40 hover:bg-gray-50"
              @click="page++; fetchReport()">Next →</button>
          </div>
        </div>
      </AppCard>

      <!-- Table: Void Item (hapus item dari order belum bayar) -->
      <AppCard v-else>
        <div v-if="report.items.length === 0" class="text-center py-12 text-gray-400">
          <svg class="w-12 h-12 mx-auto mb-3 opacity-40" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>
          <p class="font-medium">Tidak ada void item pada periode ini</p>
        </div>

        <div v-else class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-gray-200 bg-gray-50/70">
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Waktu Void</th>
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Outlet</th>
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Item</th>
                <th class="text-right py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Subtotal</th>
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Order</th>
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Petugas & Alasan</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100">
              <tr v-for="row in report.items" :key="row.id" class="hover:bg-red-50/40 transition-colors">
                <td class="py-3 px-3 whitespace-nowrap align-top">
                  <p class="font-semibold text-gray-900 tabular-nums">{{ fmtTime(row.voided_at) }}</p>
                  <p class="text-xs text-gray-400">{{ fmtDateShort(row.voided_at) }}</p>
                </td>
                <td class="py-3 px-3 align-top">
                  <span class="vr-pill"><span class="vr-dot" />{{ row.outlet_name || '—' }}</span>
                </td>
                <td class="py-3 px-3 align-top">
                  <p class="font-medium text-gray-800">{{ row.product_name }}</p>
                  <p class="text-xs text-gray-400 tabular-nums">{{ Number(row.qty) }} × {{ formatRupiah(row.price) }}</p>
                </td>
                <td class="py-3 px-3 text-right align-top">
                  <span class="font-bold text-red-600 tabular-nums">{{ formatRupiah(row.subtotal) }}</span>
                </td>
                <td class="py-3 px-3 align-top">
                  <span v-if="row.table_number" class="vr-meja">Meja {{ row.table_number }}</span>
                  <p v-if="row.waiter_name" class="text-xs text-gray-500 mt-0.5">Pemesan: <span class="font-medium text-gray-700">{{ row.waiter_name }}</span></p>
                  <span v-if="!row.table_number && !row.waiter_name" class="text-gray-400">—</span>
                </td>
                <td class="py-3 px-3 align-top">
                  <div class="flex items-start gap-2">
                    <span class="vr-avatar">{{ initials(row.voided_by) }}</span>
                    <div class="min-w-0">
                      <p class="font-medium text-gray-800 leading-tight">{{ row.voided_by || '—' }}</p>
                      <p class="text-xs text-gray-500 italic break-words">"{{ row.void_reason || 'Dibatalkan kasir' }}"</p>
                    </div>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Pagination -->
        <div v-if="report.items_total > report.limit" class="flex items-center justify-between pt-4 border-t border-gray-100 mt-4">
          <p class="text-sm text-gray-500">
            Menampilkan {{ (page-1)*report.limit + 1 }}–{{ Math.min(page*report.limit, report.items_total) }} dari {{ report.items_total }} data
          </p>
          <div class="flex gap-2">
            <button :disabled="page <= 1"
              class="px-3 py-1.5 text-sm border rounded-lg disabled:opacity-40 hover:bg-gray-50"
              @click="page--; fetchReport()">← Prev</button>
            <button :disabled="page * report.limit >= report.items_total"
              class="px-3 py-1.5 text-sm border rounded-lg disabled:opacity-40 hover:bg-gray-50"
              @click="page++; fetchReport()">Next →</button>
          </div>
        </div>
      </AppCard>
    </template>

  </div>
</template>

<script setup>
import { ref, onMounted, watch } from 'vue'
import { apiClient } from '@/api/client.js'
import { outletsApi } from '@/api/outlets.js'
import AppCard    from '@/components/ui/AppCard.vue'
import AppAlert   from '@/components/ui/AppAlert.vue'
import AppSpinner from '@/components/ui/AppSpinner.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'
import DateRangePicker from '@/components/ui/DateRangePicker.vue'

const dateFrom      = ref(new Date(new Date().setDate(1)).toISOString().slice(0,10))
const dateTo        = ref(new Date().toISOString().slice(0,10))
const range          = ref({ from: dateFrom.value, to: dateTo.value, label: 'Bulan Ini' })
watch(range, (r) => { dateFrom.value = r.from; dateTo.value = r.to; page.value = 1; fetchReport() })
const selectedOutlet = ref('')
const outletOptions  = ref([{ value: '', label: 'Semua Outlet' }])
const report        = ref(null)
const loading       = ref(false)
const errorMsg      = ref('')
const page          = ref(1)
const activeTab     = ref('order') // 'order' = void transaksi, 'item' = void item

function switchTab(tab) {
  if (activeTab.value === tab) return
  activeTab.value = tab
  page.value = 1
  fetchReport()
}

onMounted(async () => {
  try {
    // Use the scoped, permission-free endpoint so the dropdown only ever lists
    // outlets this role may access (consistent with the other report pages).
    const res = await outletsApi.myOutlets()
    const list = res.data ?? res ?? []
    outletOptions.value = [
      { value: '', label: 'Semua Outlet' },
      ...list.map(o => ({ value: o.id, label: o.name })),
    ]
  } catch {}
  fetchReport()
})

async function fetchReport() {
  loading.value = true; errorMsg.value = ''
  try {
    const params = new URLSearchParams({
      date_from: dateFrom.value,
      date_to: dateTo.value,
      page: page.value,
      limit: 50,
    })
    if (selectedOutlet.value) params.set('outlet_id', selectedOutlet.value)
    report.value = await apiClient.get(`/admin/void-report?${params}`)
  } catch (err) {
    errorMsg.value = err?.message ?? 'Gagal memuat laporan void'
  } finally {
    loading.value = false
  }
}

function parseItems(raw) {
  try { return JSON.parse(raw) ?? [] } catch { return [] }
}

function formatRupiah(val) {
  return 'Rp ' + Number(val ?? 0).toLocaleString('id-ID')
}

function fmtDateTime(str) {
  if (!str) return '—'
  return new Date(str).toLocaleString('id-ID', {
    day: '2-digit', month: 'short', year: 'numeric',
    hour: '2-digit', minute: '2-digit',
  })
}

function fmtTime(str) {
  if (!str) return '—'
  const d = new Date(str)
  return isNaN(d) ? '—' : d.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' })
}

function fmtDateShort(str) {
  if (!str) return ''
  const d = new Date(str)
  return isNaN(d) ? '' : d.toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' })
}

// Selang order dibuat → di-void; void yang terlalu cepat/lama bisa jadi sinyal
// pola yang perlu dicek (mis. void rutin sesaat setelah input).
function lifeSpan(createdStr, voidedStr) {
  if (!createdStr || !voidedStr) return ''
  const a = new Date(createdStr), b = new Date(voidedStr)
  if (isNaN(a) || isNaN(b)) return ''
  let mins = Math.round((b - a) / 60000)
  if (mins < 0 || mins > 60 * 24 * 30) return ''
  if (mins < 1) return 'di-void < 1 mnt'
  if (mins < 60) return `di-void setelah ${mins} mnt`
  const h = Math.floor(mins / 60)
  return `di-void setelah ${h} jam ${mins % 60} mnt`
}

function itemsSummary(raw) {
  const items = parseItems(raw)
  if (!items.length) return ''
  const qty = items.reduce((s, i) => s + (Number(i.qty) || 0), 0)
  return `${items.length} item · ${qty} qty`
}

function initials(name) {
  if (!name) return '?'
  return name.trim().split(/\s+/).slice(0, 2).map(w => w[0]).join('').toUpperCase()
}
</script>

<style scoped>
.vr-pill { display: inline-flex; align-items: center; gap: .4rem; padding: .2rem .6rem; border-radius: 999px; background: rgba(16,185,129,.08); border: 1px solid rgba(16,185,129,.2); font-size: .75rem; font-weight: 600; color: #065f46; white-space: nowrap; }
.vr-dot { width: 6px; height: 6px; border-radius: 50%; background: #10b981; flex-shrink: 0; }
.vr-meja { display: inline-block; padding: .1rem .5rem; border-radius: .4rem; background: rgba(59,130,246,.09); color: #1d4ed8; font-size: .7rem; font-weight: 700; }
.vr-chip { display: inline-flex; align-items: center; padding: .12rem .5rem; border-radius: .45rem; background: #f3f4f6; color: #374151; font-size: .72rem; white-space: nowrap; max-width: 11rem; overflow: hidden; text-overflow: ellipsis; }
.vr-chip-more { background: rgba(239,68,68,.08); color: #b91c1c; font-weight: 700; cursor: help; }
.vr-avatar { display: inline-flex; align-items: center; justify-content: center; width: 1.85rem; height: 1.85rem; border-radius: 50%; background: rgba(239,68,68,.1); color: #b91c1c; font-size: .68rem; font-weight: 800; flex-shrink: 0; }
</style>
