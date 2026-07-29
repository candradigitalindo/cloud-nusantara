<template>
  <div class="space-y-5">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-xl font-bold text-gray-900">Par Level &amp; Titik Pesan Ulang (ROP)</h1>
        <p class="text-xs text-gray-500 mt-0.5">
          Safety stock = 1,65 × σ pemakaian × √lead time · ROP = pemakaian harian × lead time + safety stock · Par = pemakaian × (lead time + 3 hari) + safety stock
        </p>
      </div>
      <div class="flex gap-2">
        <AppButton v-if="canUpdate" variant="secondary" :loading="autoFilling" @click="autoFill">
          Isi Otomatis dari Histori
        </AppButton>
        <AppButton v-if="canUpdate" variant="primary" :disabled="!dirtyCount" :loading="saving" @click="saveAll">
          Simpan Perubahan ({{ dirtyCount }})
        </AppButton>
      </div>
    </div>

    <AppCard :padding="false">
      <div class="flex flex-wrap items-center gap-3 px-4 py-3 border-b border-gray-100">
        <div class="min-w-[220px]">
          <SearchSelect v-model="filters.warehouse_id" :options="warehouseOptions" placeholder="Pilih Gudang" @change="reload" />
        </div>
        <div class="min-w-[180px]">
          <SearchSelect v-model="filters.category" :options="categoryOptions" placeholder="Semua Kategori" @change="reload" />
        </div>
        <label class="flex items-center gap-1.5 text-xs font-medium text-gray-600 cursor-pointer">
          <input type="checkbox" v-model="filters.below_only" @change="reload" class="rounded text-emerald-600" />
          Hanya di bawah ROP
        </label>
        <input v-model="filters.search" @input="debouncedReload" placeholder="Cari item..."
          class="flex-1 min-w-[180px] text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-emerald-400" />
      </div>

      <div v-if="!filters.warehouse_id" class="p-10 text-center text-sm text-gray-400">Pilih gudang untuk menampilkan parameter perencanaan.</div>
      <div v-else class="overflow-x-auto">
        <table class="pp-table">
          <thead>
            <tr>
              <th>Item</th>
              <th class="th-r">Stok</th>
              <th class="th-r" title="Rata-rata pemakaian harian 28 hari (± deviasi)">Pemakaian/hari</th>
              <th class="th-r">Lead Time (hr)</th>
              <th class="th-r">Safety Stock</th>
              <th class="th-r">ROP</th>
              <th class="th-r">Par Level</th>
              <th class="th-r">MOQ</th>
              <th>Saran Sistem</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading"><td colspan="10" class="p-8 text-center"><AppSpinner class="text-emerald-600 mx-auto" /></td></tr>
            <tr v-else-if="!rows.length"><td colspan="10" class="p-8 text-center text-sm text-gray-400">Tidak ada item.</td></tr>
            <tr v-for="r in rows" :key="r.item_id" :class="{ 'row-dirty': isDirty(r) }">
              <td>
                <div class="font-medium text-gray-900 text-sm">{{ r.item_name }}</div>
                <div class="text-[11px] text-gray-400 font-mono">{{ r.item_code }} · {{ r.category || '—' }}</div>
              </td>
              <td class="td-num" :class="r.is_below_rop ? 'text-red-600' : ''">{{ fmtQty(r.qty_base) }} {{ r.base_unit }}</td>
              <td class="td-num text-gray-600">{{ fmtQty(r.avg_daily_usage) }} <span class="text-gray-400">±{{ fmtQty(r.std_daily_usage) }}</span></td>
              <td class="td-num"><input type="number" min="0" class="num-in" v-model.number="r.lead_time_days" :disabled="!canUpdate" /></td>
              <td class="td-num"><input type="number" min="0" step="0.01" class="num-in num-in--wide" v-model.number="r.safety_stock" :disabled="!canUpdate" /></td>
              <td class="td-num"><input type="number" min="0" step="0.01" class="num-in num-in--wide" v-model.number="r.reorder_point" :disabled="!canUpdate" /></td>
              <td class="td-num"><input type="number" min="0" step="0.01" class="num-in num-in--wide" v-model.number="r.par_level" :disabled="!canUpdate" /></td>
              <td class="td-num"><input type="number" min="0" step="0.01" class="num-in" v-model.number="r.moq" :disabled="!canUpdate" /></td>
              <td>
                <button v-if="canUpdate && r.avg_daily_usage > 0" class="sug-chip" @click="applySuggestion(r)"
                  :title="`SS ${fmtQty(r.sug_safety_stock)} · ROP ${fmtQty(r.sug_reorder_point)} · Par ${fmtQty(r.sug_par_level)}`">
                  SS {{ fmtQty(r.sug_safety_stock) }} · ROP {{ fmtQty(r.sug_reorder_point) }} ↵
                </button>
                <span v-else class="text-[11px] text-gray-300">tanpa histori</span>
              </td>
              <td>
                <span v-if="r.is_below_rop" class="pill pill-red">Perlu Pesan</span>
                <span v-else-if="r.reorder_point > 0 || r.min_stock > 0" class="pill pill-green">Aman</span>
                <span v-else class="pill pill-gray">Tanpa Ambang</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="filters.warehouse_id" class="px-4 py-3 border-t border-gray-100 flex items-center justify-between">
        <span class="text-xs text-gray-400">{{ total }} item</span>
        <AppPagination :page="page" :total-pages="totalPages" @change="changePage" />
      </div>
    </AppCard>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ppicApi } from '@/api/ppic'
import { warehousesApi, stockItemCategoriesApi } from '@/api/warehouse'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toast'
import debounce from 'lodash/debounce'
import AppButton     from '@/components/ui/AppButton.vue'
import AppCard       from '@/components/ui/AppCard.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import AppSpinner    from '@/components/ui/AppSpinner.vue'
import SearchSelect  from '@/components/ui/SearchSelect.vue'

const route = useRoute()
const auth = useAuthStore()
const toast = useToastStore()
const canUpdate = computed(() => auth.hasPermission('ppic.planning.update'))

const loading = ref(false)
const saving = ref(false)
const autoFilling = ref(false)
const rows = ref([])
const original = ref({}) // item_id → snapshot params untuk deteksi dirty
const total = ref(0)
const page = ref(1)
const totalPages = ref(1)

const filters = reactive({
  warehouse_id: '',
  category: '',
  search: '',
  below_only: route.query.below === '1',
})

const warehouseOptions = ref([])
const categoryOptions = ref([{ id: '', name: 'Semua Kategori' }])

onMounted(async () => {
  try {
    const [whRes, catRes] = await Promise.all([warehousesApi.list({ limit: 100 }), stockItemCategoriesApi.list()])
    const whList = Array.isArray(whRes) ? whRes : (whRes?.data || [])
    warehouseOptions.value = whList.map(w => ({ id: w.id, name: `${w.name} (${w.code})` }))
    const cats = Array.isArray(catRes) ? catRes : (catRes?.data || [])
    categoryOptions.value = [{ id: '', name: 'Semua Kategori' }, ...cats.map(c => ({ id: c.name, name: c.name }))]
    if (whList.length && !filters.warehouse_id) {
      filters.warehouse_id = whList[0].id
      load()
    }
  } catch { toast.error('Gagal memuat daftar gudang') }
})

async function load() {
  if (!filters.warehouse_id) return
  loading.value = true
  try {
    const res = await ppicApi.listPlanningParams({
      warehouse_id: filters.warehouse_id,
      category: filters.category,
      search: filters.search,
      below_only: filters.below_only ? 'true' : '',
      page: page.value,
      limit: 30,
    })
    rows.value = res.data || []
    total.value = res.total || 0
    totalPages.value = Math.max(1, Math.ceil(total.value / 30))
    original.value = Object.fromEntries(rows.value.map(r => [r.item_id, snap(r)]))
  } catch (e) {
    toast.error(e?.message ?? 'Gagal memuat parameter perencanaan')
  } finally {
    loading.value = false
  }
}

function snap(r) {
  return JSON.stringify([r.lead_time_days, r.safety_stock, r.reorder_point, r.par_level, r.moq])
}
function isDirty(r) { return original.value[r.item_id] !== snap(r) }
const dirtyCount = computed(() => rows.value.filter(isDirty).length)

function applySuggestion(r) {
  r.safety_stock = r.sug_safety_stock
  r.reorder_point = r.sug_reorder_point
  r.par_level = r.sug_par_level
  if (!r.lead_time_days) r.lead_time_days = 1
}

async function saveAll() {
  const dirty = rows.value.filter(isDirty).map(r => ({
    item_id: r.item_id,
    warehouse_id: filters.warehouse_id,
    lead_time_days: Number(r.lead_time_days) || 0,
    safety_stock: Number(r.safety_stock) || 0,
    reorder_point: Number(r.reorder_point) || 0,
    par_level: Number(r.par_level) || 0,
    moq: Number(r.moq) || 0,
  }))
  if (!dirty.length) return
  saving.value = true
  try {
    await ppicApi.savePlanningParams(dirty)
    toast.success(`${dirty.length} parameter tersimpan`)
    load()
  } catch (e) {
    toast.error(e?.message ?? 'Gagal menyimpan parameter')
  } finally {
    saving.value = false
  }
}

async function autoFill() {
  if (!filters.warehouse_id) return
  if (!confirm('Hitung & simpan saran parameter untuk SEMUA item dengan histori pemakaian 28 hari di gudang ini? Parameter yang sudah ada akan diperbarui.')) return
  autoFilling.value = true
  try {
    const res = await ppicApi.autoFillPlanningParams(filters.warehouse_id)
    toast.success(`${res?.saved ?? 0} item terisi otomatis`)
    load()
  } catch (e) {
    toast.error(e?.message ?? 'Gagal mengisi otomatis')
  } finally {
    autoFilling.value = false
  }
}

function reload() { page.value = 1; load() }
const debouncedReload = debounce(reload, 500)
function changePage(p) { page.value = p; load() }
function fmtQty(v) { return Number(v ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 2 }) }
</script>

<style scoped>
.pp-table { width: 100%; border-collapse: collapse; font-size: .8rem; }
.pp-table thead tr { background: #f9fafb; }
.pp-table th { padding: .5rem .7rem; text-align: left; font-size: .64rem; font-weight: 700; text-transform: uppercase; letter-spacing: .04em; color: #9ca3af; border-bottom: 1px solid #f3f4f6; white-space: nowrap; }
.pp-table .th-r { text-align: right; }
.pp-table tbody tr { border-bottom: 1px solid #f9fafb; }
.pp-table tbody tr:hover { background: #fafafa; }
.pp-table td { padding: .45rem .7rem; }
.td-num { text-align: right; font-family: monospace; white-space: nowrap; }
.row-dirty { background: #fefce8 !important; }

.num-in { width: 64px; text-align: right; font-size: .78rem; font-family: monospace; border: 1px solid #e5e7eb; border-radius: .4rem; padding: .25rem .4rem; }
.num-in--wide { width: 84px; }
.num-in:focus { outline: none; border-color: #34d399; box-shadow: 0 0 0 2px rgba(52,211,153,.2); }
.num-in:disabled { background: #f9fafb; color: #9ca3af; }

.sug-chip { font-size: .66rem; font-weight: 600; padding: .2rem .5rem; border-radius: 999px; background: #eff6ff; color: #1d4ed8; border: 1px solid #bfdbfe; cursor: pointer; white-space: nowrap; }
.sug-chip:hover { background: #dbeafe; }

.pill { font-size: .62rem; font-weight: 700; padding: .15rem .5rem; border-radius: 999px; white-space: nowrap; }
.pill-red { background: #fee2e2; color: #dc2626; }
.pill-green { background: #dcfce7; color: #15803d; }
.pill-gray { background: #f3f4f6; color: #9ca3af; }
</style>
