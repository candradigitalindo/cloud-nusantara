<template>
  <div class="space-y-6">
    <section class="cust-hero relative overflow-hidden rounded-3xl p-5 md:p-7">
      <div class="relative z-10">
        <p class="text-xs font-semibold uppercase tracking-[0.2em] text-teal-100/90">Penjualan</p>
        <h1 class="mt-1 text-2xl font-black text-white md:text-3xl">Pelanggan</h1>
        <p class="mt-2 max-w-2xl text-sm text-teal-100/95">
          Master pelanggan terbentuk otomatis dari order kasir — dicocokkan lewat nomor HP.
          Klik pelanggan untuk melihat histori kunjungan per outlet beserta menu yang dipesan.
        </p>
      </div>
    </section>

    <AppAlert type="error" :message="errorMsg" />

    <AppCard>
      <div class="grid grid-cols-1 gap-3 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
        <div class="relative">
          <svg class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="11" cy="11" r="8" /><path d="m21 21-4.35-4.35" />
          </svg>
          <input
            v-model="search"
            type="search"
            placeholder="Cari nama atau nomor HP pelanggan..."
            class="w-full rounded-xl border border-slate-200 bg-white/90 py-2.5 pl-9 pr-3 text-sm text-slate-700 outline-none transition focus:border-teal-500 focus:ring-2 focus:ring-teal-100"
          />
        </div>
        <p class="text-xs text-slate-500">
          Total <span class="font-semibold text-slate-700">{{ total }}</span> pelanggan
        </p>
      </div>
    </AppCard>

    <AppCard :padding="false">
      <AppTable :columns="COLUMNS" :rows="customers" :loading="loading" emptyText="Belum ada data pelanggan.">
        <template #cell-name="{ row }">
          <button class="flex items-center gap-3 text-left" @click="openDetail(row)">
            <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-teal-500 to-cyan-500 text-xs font-bold text-white shadow-sm">
              {{ initial(row.name) }}
            </div>
            <div>
              <div class="font-semibold text-emerald-700 hover:text-emerald-900 hover:underline">{{ row.name || '(Tanpa nama)' }}</div>
              <div v-if="row.phone" class="text-xs text-gray-400">{{ row.phone }}</div>
              <div v-else class="text-xs italic text-gray-300">HP belum tercatat</div>
            </div>
          </button>
        </template>
        <template #cell-visits="{ row }">
          <span class="inline-flex rounded-full bg-teal-50 px-2.5 py-0.5 text-xs font-semibold text-teal-700">{{ row.visit_count }}×</span>
        </template>
        <template #cell-outlets="{ row }">
          <span class="text-sm text-gray-600">{{ row.outlet_names || '—' }}</span>
        </template>
        <template #cell-total="{ row }">
          <span class="font-semibold text-gray-700">{{ formatRupiah(row.total_spent) }}</span>
        </template>
        <template #cell-last="{ row }">
          <span class="text-sm text-gray-600">{{ formatDateTime(row.last_visit_at) }}</span>
        </template>
      </AppTable>
      <div class="border-t border-slate-100 p-3">
        <AppPagination v-model="page" :total="total" :perPage="limit" @change="load" />
      </div>
    </AppCard>

    <!-- Detail pelanggan: histori kunjungan -->
    <AppModal v-model="showDetail" :title="detail?.customer?.name || 'Detail Pelanggan'" size="lg">
      <div v-if="detailLoading" class="flex justify-center py-10"><AppSpinner /></div>
      <div v-else-if="detail" class="space-y-5">
        <div class="grid gap-3 sm:grid-cols-3">
          <div class="rounded-xl bg-slate-50 p-3">
            <p class="text-[11px] font-semibold uppercase tracking-wide text-slate-400">Nomor HP</p>
            <p class="mt-0.5 text-sm font-semibold text-slate-700">{{ detail.customer.phone || '—' }}</p>
          </div>
          <div class="rounded-xl bg-slate-50 p-3">
            <p class="text-[11px] font-semibold uppercase tracking-wide text-slate-400">Total Kunjungan</p>
            <p class="mt-0.5 text-sm font-semibold text-slate-700">{{ detail.customer.visit_count }}× — {{ formatRupiah(detail.customer.total_spent) }}</p>
          </div>
          <div class="rounded-xl bg-slate-50 p-3">
            <p class="text-[11px] font-semibold uppercase tracking-wide text-slate-400">Kunjungan Terakhir</p>
            <p class="mt-0.5 text-sm font-semibold text-slate-700">{{ formatDateTime(detail.customer.last_visit_at) }}</p>
          </div>
        </div>

        <div>
          <h3 class="mb-2 text-sm font-bold text-slate-700">Rekap per Outlet</h3>
          <div class="flex flex-wrap gap-2">
            <div v-for="o in detail.outlets" :key="o.outlet_id"
              class="rounded-xl border border-teal-100 bg-teal-50/60 px-3 py-2 text-xs">
              <span class="font-semibold text-teal-800">{{ o.outlet_name }}</span>
              <span class="text-teal-600"> — {{ o.visit_count }}× kunjungan, {{ formatRupiah(o.total_spent) }}</span>
            </div>
          </div>
        </div>

        <div>
          <h3 class="mb-2 text-sm font-bold text-slate-700">Histori Kunjungan <span class="font-normal text-slate-400">({{ detail.visits.length }} terakhir)</span></h3>
          <div class="max-h-[45vh] space-y-3 overflow-y-auto pr-1">
            <div v-for="v in detail.visits" :key="v.order_id" class="rounded-2xl border border-slate-100 p-3">
              <div class="flex flex-wrap items-center justify-between gap-2">
                <div class="text-sm">
                  <span class="font-semibold text-slate-700">{{ v.outlet_name }}</span>
                  <span v-if="v.table_number" class="text-slate-400"> · Meja {{ v.table_number }}</span>
                  <span v-if="v.pax" class="text-slate-400"> · {{ v.pax }} pax</span>
                </div>
                <div class="flex items-center gap-2 text-xs">
                  <span class="text-slate-400">{{ formatDateTime(v.created_at) }}</span>
                  <span class="inline-flex rounded-full px-2 py-0.5 font-semibold"
                    :class="v.status === 'completed' ? 'bg-emerald-50 text-emerald-700' : 'bg-amber-50 text-amber-700'">
                    {{ v.status }}
                  </span>
                </div>
              </div>
              <ul class="mt-2 divide-y divide-slate-50 text-sm">
                <li v-for="(it, i) in parseItems(v.items)" :key="i" class="flex justify-between py-1">
                  <span class="text-slate-600">{{ it.qty }}× {{ it.product_name }}</span>
                  <span class="text-slate-500">{{ formatRupiah(it.subtotal ?? (it.qty * it.price)) }}</span>
                </li>
              </ul>
              <div class="mt-1 flex justify-between border-t border-slate-100 pt-1.5 text-sm font-semibold">
                <span class="text-slate-500">Total</span>
                <span class="text-slate-800">{{ formatRupiah(v.total_amount) }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </AppModal>
  </div>
</template>

<script setup>
import { onMounted, ref, watch } from 'vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppCard from '@/components/ui/AppCard.vue'
import AppModal from '@/components/ui/AppModal.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import AppSpinner from '@/components/ui/AppSpinner.vue'
import AppTable from '@/components/ui/AppTable.vue'
import { customersApi } from '@/api/customers.js'
import { formatDateTime, formatRupiah } from '@/utils/format.js'

const COLUMNS = [
  { key: 'name',    label: 'Pelanggan' },
  { key: 'visits',  label: 'Kunjungan', align: 'center' },
  { key: 'outlets', label: 'Outlet' },
  { key: 'total',   label: 'Total Belanja', align: 'right' },
  { key: 'last',    label: 'Terakhir Datang' },
]

const customers = ref([])
const loading = ref(false)
const errorMsg = ref('')
const search = ref('')
const page = ref(1)
const limit = 20
const total = ref(0)

const showDetail = ref(false)
const detail = ref(null)
const detailLoading = ref(false)

async function load() {
  loading.value = true
  errorMsg.value = ''
  try {
    const res = await customersApi.list({ search: search.value, page: page.value, limit })
    customers.value = res.data.data.customers || []
    total.value = res.data.data.pagination?.total || 0
  } catch (e) {
    errorMsg.value = e.response?.data?.error || 'Gagal memuat data pelanggan'
  } finally {
    loading.value = false
  }
}

let searchTimer
watch(search, () => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => { page.value = 1; load() }, 350)
})
watch(page, load)

async function openDetail(row) {
  showDetail.value = true
  detailLoading.value = true
  detail.value = null
  try {
    const res = await customersApi.get(row.id)
    detail.value = res.data.data
  } catch (e) {
    errorMsg.value = e.response?.data?.error || 'Gagal memuat detail pelanggan'
    showDetail.value = false
  } finally {
    detailLoading.value = false
  }
}

function parseItems(items) {
  try {
    const arr = typeof items === 'string' ? JSON.parse(items) : items
    return Array.isArray(arr) ? arr : []
  } catch { return [] }
}

function initial(name) {
  return (name || '?').trim().charAt(0).toUpperCase() || '?'
}

onMounted(load)
</script>

<style scoped>
.cust-hero {
  background: linear-gradient(135deg, #0f766e 0%, #0e7490 55%, #155e75 100%);
}
</style>
