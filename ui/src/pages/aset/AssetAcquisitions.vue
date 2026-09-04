<template>
  <div class="space-y-5">
    <div class="flex items-start justify-between flex-wrap gap-3">
      <div>
        <h1 class="text-xl font-bold text-gray-900">Histori Perolehan</h1>
        <p class="text-sm text-gray-500 mt-0.5">Asal-usul setiap aset: pembelian, hibah, sewa, atau produksi sendiri.</p>
      </div>
      <AppButton v-if="canManage" @click="openCreate">+ Catat Perolehan</AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <div class="kpi-grid">
      <div class="kpi kpi--slate">
        <div class="kpi-label">Total Nilai Perolehan</div>
        <div class="kpi-val">{{ formatRupiah(totalCost) }}</div>
        <div class="kpi-sub">{{ rows.length }} catatan · {{ totalQty }} unit</div>
      </div>
      <div v-for="s in sourceTotals" :key="s.key" class="kpi kpi--slate">
        <div class="kpi-label">{{ SOURCES[s.key] || s.key }}</div>
        <div class="kpi-val">{{ formatRupiah(s.value) }}</div>
        <div class="kpi-sub">{{ s.count }} catatan · {{ s.qty }} unit</div>
      </div>
    </div>

    <AppCard>
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-3">
        <SearchSelect v-model="filterOutlet" :options="outletFilterOptions" placeholder="Semua outlet" searchPlaceholder="Cari outlet…" @change="load" />
        <select v-model="filterSource" @change="load" class="form-input">
          <option value="">Semua sumber</option>
          <option v-for="(lbl, key) in SOURCES" :key="key" :value="key">{{ lbl }}</option>
        </select>
        <input v-model="from" @change="load" type="date" class="form-input" title="Dari tanggal" />
        <input v-model="to" @change="load" type="date" class="form-input" title="Sampai tanggal" />
        <input v-model="search" @input="debouncedLoad" type="search" placeholder="Cari aset / no. dokumen / vendor…" class="form-input" />
      </div>
    </AppCard>

    <AppCard :padding="false">
      <!-- Mobile -->
      <div class="sm:hidden">
        <div v-if="loading" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
        <div v-else-if="!rows.length" class="p-6 text-center text-sm text-gray-400">Belum ada catatan perolehan.</div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="x in rows" :key="x.id" class="p-4 space-y-1.5">
            <div class="flex items-start justify-between gap-2">
              <div class="min-w-0">
                <p class="font-semibold text-gray-900 break-words">{{ x.asset_name }}</p>
                <p class="text-xs text-gray-500">{{ formatDateStr(x.acquisition_date) }} · {{ x.outlet_name }}</p>
              </div>
              <span class="src-badge shrink-0">{{ SOURCES[x.source] || x.source }}</span>
            </div>
            <p class="text-sm text-gray-800">{{ x.quantity }} × {{ formatRupiah(x.unit_price) }} = <strong>{{ formatRupiah(x.total_cost) }}</strong></p>
            <p v-if="x.vendor_name || x.document_no" class="text-xs text-gray-500">
              <span v-if="x.vendor_name">{{ x.vendor_name }}</span><span v-if="x.document_no"> · {{ x.document_no }}</span>
            </p>
            <button v-if="canManage" @click="confirmDelete(x)" class="text-xs font-medium text-red-600">Hapus catatan</button>
          </li>
        </ul>
      </div>

      <AppTable class="hidden sm:block" :columns="COLUMNS" :rows="rows" :loading="loading" emptyText="Belum ada catatan perolehan.">
        <template #cell-date="{ row }">{{ formatDateStr(row.acquisition_date) }}</template>
        <template #cell-asset="{ row }">
          <div>
            <p class="font-medium text-gray-900">{{ row.asset_name }}</p>
            <p v-if="row.asset_code" class="text-xs text-gray-400 font-mono">{{ row.asset_code }}</p>
          </div>
        </template>
        <template #cell-source="{ row }"><span class="src-badge">{{ SOURCES[row.source] || row.source }}</span></template>
        <template #cell-qty="{ row }">{{ row.quantity }} × {{ formatRupiah(row.unit_price) }}</template>
        <template #cell-total="{ row }"><strong>{{ formatRupiah(row.total_cost) }}</strong></template>
        <template #cell-vendor="{ row }">
          <span>{{ row.vendor_name || '—' }}</span>
          <span v-if="row.document_no" class="text-xs text-gray-400 block font-mono">{{ row.document_no }}</span>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex justify-end">
            <button v-if="canManage" @click="confirmDelete(row)" class="text-red-600 hover:text-red-800 text-xs font-medium px-2 py-1 rounded hover:bg-red-50">Hapus</button>
          </div>
        </template>
      </AppTable>
    </AppCard>

    <AppModal v-model="modal" title="Catat Perolehan">
      <form class="space-y-3" @submit.prevent="save">
        <div>
          <label class="lbl">Aset <span class="text-red-500">*</span></label>
          <SearchSelect v-model="form.asset_id" :options="assetOptions" placeholder="Pilih aset…" searchPlaceholder="Cari aset…" />
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="lbl">Tanggal Perolehan</label>
            <input v-model="form.acquisition_date" type="date" class="form-input" />
          </div>
          <div>
            <label class="lbl">Sumber</label>
            <select v-model="form.source" class="form-input">
              <option v-for="(lbl, key) in SOURCES" :key="key" :value="key">{{ lbl }}</option>
            </select>
          </div>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="lbl">Jumlah Unit</label>
            <input v-model.number="form.quantity" type="number" min="1" class="form-input" />
          </div>
          <div>
            <label class="lbl">Harga Satuan</label>
            <input v-model.number="form.unit_price" type="number" min="0" class="form-input" />
          </div>
        </div>
        <p class="text-xs text-gray-500 -mt-1">Total: <strong>{{ formatRupiah((form.quantity || 0) * (form.unit_price || 0)) }}</strong></p>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="lbl">Vendor / Pemasok</label>
            <input v-model="form.vendor_name" class="form-input" placeholder="Nama vendor" />
          </div>
          <div>
            <label class="lbl">No. Dokumen</label>
            <input v-model="form.document_no" class="form-input" placeholder="Faktur / PO" />
          </div>
        </div>
        <div>
          <label class="lbl">Catatan</label>
          <textarea v-model="form.notes" rows="2" class="form-input" placeholder="Opsional"></textarea>
        </div>
        <label class="flex items-start gap-2 text-sm text-gray-700 bg-gray-50 rounded-lg p-2.5">
          <input v-model="form.add_to_quantity" type="checkbox" class="mt-0.5" />
          <span>
            Tambahkan {{ form.quantity || 0 }} unit ke jumlah aset
            <span class="block text-xs text-gray-500">Aktifkan bila ini unit baru yang benar-benar masuk. Matikan bila hanya merapikan catatan perolehan lama yang unitnya sudah terhitung.</span>
          </span>
        </label>
        <div class="flex justify-end gap-2 pt-1">
          <button type="button" class="btn-ghost" @click="modal = false">Batal</button>
          <AppButton type="submit" :loading="saving">Simpan</AppButton>
        </div>
      </form>
    </AppModal>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { assetsApi } from '@/api/assets.js'
import { outletsApi } from '@/api/outlets.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth.js'
import { formatRupiah, formatDateStr, todayDateString } from '@/utils/format.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppTable from '@/components/ui/AppTable.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppModal from '@/components/ui/AppModal.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'

const toast = useToastStore()
const auth = useAuthStore()
const canManage = auth.hasPermission('assets.acquisition.manage')

const SOURCES = { pembelian: 'Pembelian', hibah: 'Hibah', sewa: 'Sewa', produksi_sendiri: 'Produksi Sendiri' }

const COLUMNS = [
  { key: 'date',    label: 'Tanggal' },
  { key: 'asset',   label: 'Aset' },
  { key: 'outlet_name', label: 'Outlet' },
  { key: 'source',  label: 'Sumber' },
  { key: 'qty',     label: 'Jumlah × Harga' },
  { key: 'total',   label: 'Total' },
  { key: 'vendor',  label: 'Vendor / Dokumen' },
  { key: 'actions', label: '' },
]

const rows = ref([])
const assets = ref([])
const outlets = ref([])
const loading = ref(false)
const errorMsg = ref('')
const filterOutlet = ref('')
const filterSource = ref('')
const from = ref('')
const to = ref('')
const search = ref('')

function asArray(d) { return Array.isArray(d) ? d : (d?.data || []) }
const outletFilterOptions = computed(() => [{ id: '', name: 'Semua outlet' }, ...outlets.value])
const assetOptions = computed(() => assets.value.map(a => ({ id: a.id, name: `${a.name}${a.code ? ' · ' + a.code : ''} — ${a.outlet_name}` })))
const totalCost = computed(() => rows.value.reduce((s, x) => s + Number(x.total_cost || 0), 0))
const totalQty = computed(() => rows.value.reduce((s, x) => s + Number(x.quantity || 0), 0))
const sourceTotals = computed(() => {
  const map = {}
  for (const x of rows.value) {
    const k = x.source || 'pembelian'
    map[k] = map[k] || { key: k, value: 0, count: 0, qty: 0 }
    map[k].value += Number(x.total_cost || 0)
    map[k].count += 1
    map[k].qty += Number(x.quantity || 0)
  }
  return Object.values(map).sort((a, b) => b.value - a.value).slice(0, 3)
})

async function load() {
  loading.value = true; errorMsg.value = ''
  try {
    rows.value = asArray(await assetsApi.acquisitions({
      outlet_id: filterOutlet.value || undefined,
      source: filterSource.value || undefined,
      from: from.value || undefined,
      to: to.value || undefined,
      search: search.value.trim() || undefined,
    }))
  } catch (e) {
    errorMsg.value = e?.message || 'Gagal memuat histori perolehan'
  } finally {
    loading.value = false
  }
}
let _t = null
function debouncedLoad() { clearTimeout(_t); _t = setTimeout(load, 350) }

async function loadRefs() {
  try { assets.value = asArray(await assetsApi.list()) } catch { assets.value = [] }
  try { const r = await outletsApi.myOutlets(); outlets.value = r?.outlets ?? r ?? [] } catch { outlets.value = [] }
}

const modal = ref(false)
const saving = ref(false)
const form = ref({})
function openCreate() {
  form.value = { asset_id: '', acquisition_date: todayDateString(), source: 'pembelian', quantity: 1, unit_price: 0, vendor_name: '', document_no: '', notes: '', add_to_quantity: true }
  modal.value = true
}
async function save() {
  if (!form.value.asset_id) { toast.error('Pilih aset'); return }
  saving.value = true
  try {
    await assetsApi.addAcquisition(form.value)
    toast.success('Perolehan dicatat')
    modal.value = false
    await Promise.all([load(), loadRefs()])
  } catch (e) { toast.error(e?.message || 'Gagal menyimpan') } finally { saving.value = false }
}
async function confirmDelete(x) {
  if (!window.confirm(`Hapus catatan perolehan "${x.asset_name}" tanggal ${formatDateStr(x.acquisition_date)}? Jumlah unit aset tidak ikut berkurang.`)) return
  try { await assetsApi.removeAcquisition(x.id); toast.success('Catatan dihapus'); await load() }
  catch (e) { toast.error(e?.message || 'Gagal menghapus') }
}

onMounted(async () => { await loadRefs(); await load() })
</script>

<style scoped>
.form-input {
  width: 100%; padding: .5rem .7rem; border-radius: .6rem; font-size: .85rem;
  border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none;
}
.form-input:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }
.lbl { display: block; font-size: .72rem; font-weight: 700; color: #4b5563; margin-bottom: .25rem; }
.btn-ghost { padding: .5rem 1rem; border-radius: .6rem; font-size: .85rem; font-weight: 600; color: #374151; background: #f3f4f6; }
.btn-ghost:hover { background: #e5e7eb; }

.kpi-grid { display: grid; gap: .75rem; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); }
.kpi { border-radius: .85rem; padding: .85rem 1rem; background: #fff; border: 1px solid rgba(0,0,0,.07); box-shadow: 0 1px 2px rgba(16,24,40,.04); }
.kpi--slate { background: linear-gradient(180deg, #fff, #f8fafc); }
.kpi-label { font-size: .7rem; font-weight: 700; color: #6b7280; text-transform: uppercase; letter-spacing: .02em; }
.kpi-val { font-size: 1.3rem; font-weight: 800; color: #111827; line-height: 1.2; margin-top: .15rem; }
.kpi-sub { font-size: .7rem; color: #6b7280; margin-top: .1rem; }

.src-badge { display: inline-block; padding: .12rem .5rem; border-radius: 999px; font-size: .68rem; font-weight: 700; background: rgba(99,102,241,.12); color: #4338ca; white-space: nowrap; }
</style>
