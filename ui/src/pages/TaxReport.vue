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
        <button @click="fetchReport"
          class="px-4 py-2 bg-emerald-600 text-white text-sm font-medium rounded-lg hover:bg-emerald-700 transition-colors shadow-sm">
          Tampilkan
        </button>
        <button @click="downloadExcel" :disabled="exporting"
          class="inline-flex items-center gap-2 px-4 py-2 bg-white border border-emerald-600 text-emerald-700 text-sm font-medium rounded-lg hover:bg-emerald-50 transition-colors shadow-sm disabled:opacity-60 disabled:cursor-not-allowed">
          <svg v-if="!exporting" class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M4 16v2a2 2 0 002 2h12a2 2 0 002-2v-2M12 4v12m0 0l-4-4m4 4l4-4" />
          </svg>
          <AppSpinner v-else size="sm" />
          {{ exporting ? 'Menyiapkan…' : 'Download Excel' }}
        </button>
      </div>
    </AppCard>

    <AppAlert type="error" :message="errorMsg" />

    <div v-if="loading" class="flex justify-center py-12">
      <AppSpinner size="lg" />
    </div>

    <template v-if="!loading && report">
      <!-- Info Tarif -->
      <div class="flex items-center gap-2 px-1">
        <svg class="size-4 text-blue-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
        </svg>
        <p class="text-xs text-gray-500">
          Pajak dihitung <strong>per outlet</strong> dari nilai pajak nyata tiap transaksi (inklusif).
          Tarif efektif gabungan <strong>{{ report.summary.tax_rate }}%</strong> — rincian tarif tiap outlet ada di tabel "Per Outlet".
        </p>
      </div>

      <!-- Summary Cards -->
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <SummaryCard label="Total Transaksi" :value="report.summary.total_transactions" icon="receipt" />
        <SummaryCard label="Pendapatan Bruto" :value="formatRupiah(report.summary.gross_revenue)" icon="revenue" />
        <SummaryCard label="Pajak Restoran" :value="formatRupiah(report.summary.tax_amount)" icon="avg" />
        <SummaryCard label="Pendapatan Neto" :value="formatRupiah(report.summary.net_revenue)" icon="revenue" />
      </div>

      <!-- Daily -->
      <AppCard v-if="report.daily?.length > 0" :padding="false">
        <div class="px-4 pt-4 pb-2">
          <h3 class="text-sm font-semibold text-gray-700">Rincian Harian</h3>
        </div>
        <AppTable :columns="DAILY_COLS" :rows="report.daily" :loading="false" emptyText="Tidak ada data.">
          <template #cell-date="{ row }">{{ formatDateStr(row.date) }}</template>
          <template #cell-gross_revenue="{ row }">{{ formatRupiah(row.gross_revenue) }}</template>
          <template #cell-tax_amount="{ row }">
            <span class="text-amber-600 font-medium">{{ formatRupiah(row.tax_amount) }}</span>
          </template>
          <template #cell-net_revenue="{ row }">
            <span class="font-semibold text-emerald-700">{{ formatRupiah(row.net_revenue) }}</span>
          </template>
        </AppTable>
      </AppCard>

      <!-- By Outlet -->
      <AppCard v-if="report.by_outlet?.length > 1" :padding="false">
        <div class="px-4 pt-4 pb-2">
          <h3 class="text-sm font-semibold text-gray-700">Per Outlet</h3>
        </div>
        <AppTable :columns="OUTLET_COLS" :rows="report.by_outlet" :loading="false" emptyText="Tidak ada data.">
          <template #cell-tax_rate="{ row }">
            <span v-if="row.tax_enabled" class="text-gray-700">{{ row.tax_rate }}%</span>
            <span v-else class="text-gray-400">Nonaktif</span>
          </template>
          <template #cell-gross_revenue="{ row }">{{ formatRupiah(row.gross_revenue) }}</template>
          <template #cell-tax_amount="{ row }">
            <span class="text-amber-600 font-medium">{{ formatRupiah(row.tax_amount) }}</span>
          </template>
          <template #cell-net_revenue="{ row }">
            <span class="font-semibold text-emerald-700">{{ formatRupiah(row.net_revenue) }}</span>
          </template>
        </AppTable>
      </AppCard>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { salesApi }    from '@/api/sales.js'
import { outletsApi }  from '@/api/outlets.js'
import { formatRupiah, formatDateStr, todayDateString } from '@/utils/format.js'
import AppCard      from '@/components/ui/AppCard.vue'
import AppTable     from '@/components/ui/AppTable.vue'
import AppAlert     from '@/components/ui/AppAlert.vue'
import AppSpinner   from '@/components/ui/AppSpinner.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'
import SummaryCard  from '@/components/SummaryCard.vue'
import DateRangePicker from '@/components/ui/DateRangePicker.vue'

const today = todayDateString()
const dateFrom       = ref(today)
const dateTo         = ref(today)
const range          = ref({ from: dateFrom.value, to: dateTo.value, label: 'Hari Ini' })
watch(range, (r) => { dateFrom.value = r.from; dateTo.value = r.to; fetchReport() })
const selectedOutlet = ref('')
const outletOptions  = ref([])
const loading        = ref(false)
const errorMsg       = ref('')
const report         = ref(null)

const taxRateLabel = computed(() => {
  const rate = report.value?.summary?.tax_rate
  return rate != null ? `Pajak (${rate}%)` : 'Pajak'
})

const DAILY_COLS = computed(() => [
  { key: 'date',              label: 'Tanggal' },
  { key: 'total_transactions',label: 'Transaksi', align: 'right' },
  { key: 'gross_revenue',     label: 'Bruto', align: 'right' },
  { key: 'tax_amount',        label: taxRateLabel.value, align: 'right' },
  { key: 'net_revenue',       label: 'Neto', align: 'right' },
])

const OUTLET_COLS = computed(() => [
  { key: 'outlet_name',   label: 'Outlet' },
  { key: 'tax_rate',      label: 'Tarif', align: 'right' },
  { key: 'gross_revenue', label: 'Bruto', align: 'right' },
  { key: 'tax_amount',    label: 'Pajak', align: 'right' },
  { key: 'net_revenue',   label: 'Neto', align: 'right' },
])

onMounted(async () => {
  try {
    const data = await outletsApi.myOutlets()
    const list = data.outlets ?? data ?? []
    outletOptions.value = list.map(o => ({ value: o.id, label: o.name }))
  } catch { /* ignore */ }
  fetchReport()
})

async function fetchReport() {
  loading.value = true
  errorMsg.value = ''
  try {
    const params = { date_from: dateFrom.value, date_to: dateTo.value }
    if (selectedOutlet.value) params.outlet_id = selectedOutlet.value
    report.value = await salesApi.getTaxReport(params)
  } catch (err) {
    errorMsg.value = err?.message ?? 'Gagal memuat laporan pajak.'
  } finally {
    loading.value = false
  }
}

// Download Excel mengikuti filter aktif (tanggal + outlet); scope role
// tetap dipaksakan di server, jadi user hanya menerima data outlet miliknya.
const exporting = ref(false)
async function downloadExcel() {
  if (exporting.value) return
  exporting.value = true
  errorMsg.value = ''
  try {
    const params = { date_from: dateFrom.value, date_to: dateTo.value }
    if (selectedOutlet.value) params.outlet_id = selectedOutlet.value
    const blob = await salesApi.exportTaxReport(params)

    const outletLabel = selectedOutlet.value
      ? (outletOptions.value.find(o => o.value === selectedOutlet.value)?.label || 'Outlet')
      : 'Semua-Outlet'
    const slug = outletLabel.replace(/[^A-Za-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || 'Outlet'
    const filename = `Laporan-Pajak_${slug}_${dateFrom.value}_sd_${dateTo.value}.xlsx`

    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  } catch {
    errorMsg.value = 'Gagal mengunduh Excel. Coba lagi, atau persempit rentang tanggal.'
  } finally {
    exporting.value = false
  }
}
</script>
