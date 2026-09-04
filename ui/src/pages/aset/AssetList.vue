<template>
  <div class="space-y-5">
    <!-- Header -->
    <div class="flex items-start justify-between flex-wrap gap-3">
      <div>
        <h1 class="text-xl font-bold text-gray-900">Manajemen Perlengkapan</h1>
        <p class="text-sm text-gray-500 mt-0.5">Inventaris barang (meja, kursi, dll) beserta histori perawatannya.</p>
      </div>
      <AppButton v-if="canCreate" @click="openCreate">+ Tambah Perlengkapan</AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Filters -->
    <AppCard>
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <SearchSelect v-model="filterOutlet" :options="outletFilterOptions" placeholder="Semua outlet" searchPlaceholder="Cari outlet…" @change="load" />
        <select v-model="filterCondition" @change="load" class="form-input">
          <option value="">Semua kondisi</option>
          <option v-for="(lbl, key) in CONDITIONS" :key="key" :value="key">{{ lbl }}</option>
        </select>
        <input v-model="search" @input="debouncedLoad" type="search" placeholder="Cari nama / kode / kategori…" class="form-input" />
      </div>
    </AppCard>

    <!-- List -->
    <AppCard :padding="false">
      <!-- Mobile cards -->
      <div class="sm:hidden">
        <div v-if="loading" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
        <div v-else-if="!assets.length" class="p-6 text-center text-sm text-gray-400">Belum ada perlengkapan.</div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="a in assets" :key="a.id" class="p-4 space-y-2">
            <div class="flex items-start justify-between gap-2">
              <div class="min-w-0">
                <p class="font-semibold text-gray-900 break-words">{{ a.name }}</p>
                <p class="text-xs text-gray-500 mt-0.5">
                  <span v-if="a.code" class="font-mono">{{ a.code }}</span>
                  <span v-if="a.category"> · {{ a.category }}</span>
                  · {{ a.quantity }} {{ a.unit }}
                </p>
              </div>
              <span class="cond-badge shrink-0" :class="condCls(a.condition)">{{ CONDITIONS[a.condition] || a.condition }}</span>
            </div>
            <p class="text-xs text-gray-500">{{ a.outlet_name }}<span v-if="a.location"> · {{ a.location }}</span></p>
            <p class="text-xs text-gray-500">Perawatan: {{ a.maintenance_count }}× · Terakhir: {{ a.last_maintenance ? formatDateStr(a.last_maintenance) : '—' }}</p>
            <div class="flex gap-2 pt-1">
              <button @click="openHistory(a)" class="flex-1 text-center text-xs font-medium px-2 py-1.5 rounded-lg bg-emerald-50 text-emerald-700 hover:bg-emerald-100">Riwayat</button>
              <button v-if="canUpdate" @click="openEdit(a)" class="flex-1 text-center text-xs font-medium px-2 py-1.5 rounded-lg bg-gray-100 text-gray-700 hover:bg-gray-200">Edit</button>
              <button v-if="canDelete" @click="confirmDelete(a)" class="flex-1 text-center text-xs font-medium px-2 py-1.5 rounded-lg bg-red-50 text-red-600 hover:bg-red-100">Hapus</button>
            </div>
          </li>
        </ul>
      </div>

      <!-- Desktop table -->
      <AppTable class="hidden sm:block" :columns="COLUMNS" :rows="assets" :loading="loading" emptyText="Belum ada perlengkapan. Tambahkan di kanan atas.">
        <template #cell-name="{ row }">
          <div>
            <p class="font-medium text-gray-900">{{ row.name }}</p>
            <p v-if="row.code" class="text-xs text-gray-400 font-mono">{{ row.code }}</p>
          </div>
        </template>
        <template #cell-quantity="{ row }">{{ row.quantity }} {{ row.unit }}</template>
        <template #cell-condition="{ row }">
          <span class="cond-badge" :class="condCls(row.condition)">{{ CONDITIONS[row.condition] || row.condition }}</span>
        </template>
        <template #cell-maintenance="{ row }">
          <span class="text-sm">{{ row.maintenance_count }}×</span>
          <span class="text-xs text-gray-400 block">{{ row.last_maintenance ? formatDateStr(row.last_maintenance) : 'belum ada' }}</span>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex items-center gap-1 justify-end">
            <button @click="openHistory(row)" class="text-emerald-600 hover:text-emerald-800 text-xs font-medium px-2 py-1 rounded hover:bg-emerald-50">Riwayat</button>
            <button v-if="canUpdate" @click="openEdit(row)" class="text-gray-600 hover:text-gray-900 text-xs font-medium px-2 py-1 rounded hover:bg-gray-100">Edit</button>
            <button v-if="canDelete" @click="confirmDelete(row)" class="text-red-600 hover:text-red-800 text-xs font-medium px-2 py-1 rounded hover:bg-red-50">Hapus</button>
          </div>
        </template>
      </AppTable>
    </AppCard>

    <!-- ── Asset Modal ── -->
    <AppModal v-model="assetModal" :title="editing ? 'Edit Perlengkapan' : 'Tambah Perlengkapan'">
      <form class="space-y-3" @submit.prevent="saveAsset">
        <div v-if="!editing">
          <label class="lbl">Outlet <span class="text-red-500">*</span></label>
          <SearchSelect v-model="form.outlet_id" :options="outlets" placeholder="Pilih outlet…" searchPlaceholder="Cari outlet…" />
        </div>
        <div>
          <label class="lbl">Nama Perlengkapan <span class="text-red-500">*</span></label>
          <input v-model="form.name" class="form-input" placeholder="Contoh: Meja Kayu Jati" required />
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="lbl">Kode / Tag</label>
            <input v-model="form.code" class="form-input" placeholder="mis. MJ-001" />
          </div>
          <div>
            <label class="lbl">Kategori</label>
            <input v-model="form.category" class="form-input" placeholder="mis. Furniture" list="asset-cats" />
            <datalist id="asset-cats">
              <option v-for="c in categorySuggestions" :key="c" :value="c" />
            </datalist>
          </div>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="lbl">Jumlah</label>
            <input v-model.number="form.quantity" type="number" min="1" class="form-input" />
          </div>
          <div>
            <label class="lbl">Satuan</label>
            <input v-model="form.unit" class="form-input" placeholder="unit" />
          </div>
        </div>
        <div>
          <label class="lbl">Kondisi</label>
          <select v-model="form.condition" class="form-input">
            <option v-for="(lbl, key) in CONDITIONS" :key="key" :value="key">{{ lbl }}</option>
          </select>
        </div>
        <div>
          <label class="lbl">Lokasi / Ruang</label>
          <input v-model="form.location" class="form-input" placeholder="mis. Lantai 1 – Area Indoor" />
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="lbl">Tgl Pembelian</label>
            <input v-model="form.purchase_date" type="date" class="form-input" />
          </div>
          <div>
            <label class="lbl">Harga Beli</label>
            <input v-model.number="form.purchase_price" type="number" min="0" class="form-input" placeholder="0" />
          </div>
        </div>
        <div>
          <label class="lbl">Catatan</label>
          <textarea v-model="form.notes" rows="2" class="form-input" placeholder="Opsional"></textarea>
        </div>
        <div class="flex justify-end gap-2 pt-1">
          <button type="button" class="btn-ghost" @click="assetModal = false">Batal</button>
          <AppButton type="submit" :loading="saving">{{ editing ? 'Simpan' : 'Tambah' }}</AppButton>
        </div>
      </form>
    </AppModal>

    <!-- ── Maintenance History Modal ── -->
    <AppModal v-model="historyModal" :title="`Riwayat Perawatan — ${activeAsset?.name || ''}`">
      <div v-if="activeAsset" class="space-y-4">
        <div class="flex items-center justify-between flex-wrap gap-2 text-xs text-gray-500">
          <span>{{ activeAsset.outlet_name }}<span v-if="activeAsset.location"> · {{ activeAsset.location }}</span></span>
          <span class="cond-badge" :class="condCls(activeAsset.condition)">{{ CONDITIONS[activeAsset.condition] || activeAsset.condition }}</span>
        </div>

        <!-- Add maintenance -->
        <details v-if="canUpdate" class="rounded-lg border border-gray-200" :open="!history.length">
          <summary class="cursor-pointer select-none px-3 py-2 text-sm font-medium text-emerald-700 bg-emerald-50 rounded-lg">+ Catat Perawatan</summary>
          <form class="p-3 space-y-3 border-t border-gray-100" @submit.prevent="saveMaintenance">
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="lbl">Tanggal</label>
                <input v-model="mForm.maintenance_date" type="date" class="form-input" />
              </div>
              <div>
                <label class="lbl">Jenis</label>
                <select v-model="mForm.type" class="form-input">
                  <option v-for="(lbl, key) in MTYPES" :key="key" :value="key">{{ lbl }}</option>
                </select>
              </div>
            </div>
            <div>
              <label class="lbl">Deskripsi <span class="text-red-500">*</span></label>
              <textarea v-model="mForm.description" rows="2" class="form-input" placeholder="Pekerjaan yang dilakukan" required></textarea>
            </div>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="lbl">Biaya</label>
                <input v-model.number="mForm.cost" type="number" min="0" class="form-input" placeholder="0" />
              </div>
              <div>
                <label class="lbl">Pelaksana / Teknisi</label>
                <input v-model="mForm.performed_by" class="form-input" placeholder="Nama / vendor" />
              </div>
            </div>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="lbl">Kondisi Setelah</label>
                <select v-model="mForm.condition_after" class="form-input">
                  <option value="">— tidak diubah —</option>
                  <option v-for="(lbl, key) in CONDITIONS" :key="key" :value="key">{{ lbl }}</option>
                </select>
              </div>
              <div>
                <label class="lbl">Jadwal Berikutnya</label>
                <input v-model="mForm.next_due_date" type="date" class="form-input" />
              </div>
            </div>
            <div class="flex justify-end">
              <AppButton type="submit" :loading="savingM">Simpan Perawatan</AppButton>
            </div>
          </form>
        </details>

        <!-- Timeline -->
        <div v-if="loadingHistory" class="text-center text-sm text-gray-400 py-4">Memuat histori…</div>
        <div v-else-if="!history.length" class="text-center text-sm text-gray-400 py-4">Belum ada catatan perawatan.</div>
        <ol v-else class="space-y-3">
          <li v-for="m in history" :key="m.id" class="relative pl-5 border-l-2 border-emerald-100">
            <span class="absolute -left-[7px] top-1 w-3 h-3 rounded-full bg-emerald-500 ring-2 ring-white"></span>
            <div class="flex items-start justify-between gap-2">
              <div class="min-w-0">
                <div class="flex items-center gap-2 flex-wrap">
                  <span class="text-sm font-semibold text-gray-900">{{ formatDateStr(m.maintenance_date) }}</span>
                  <span class="mtype-badge">{{ MTYPES[m.type] || m.type }}</span>
                </div>
                <p class="text-sm text-gray-700 mt-0.5 break-words">{{ m.description }}</p>
                <p class="text-xs text-gray-500 mt-1 space-x-2">
                  <span v-if="m.cost > 0">Biaya: {{ formatRupiah(m.cost) }}</span>
                  <span v-if="m.performed_by">Oleh: {{ m.performed_by }}</span>
                  <span v-if="m.condition_after">→ {{ CONDITIONS[m.condition_after] || m.condition_after }}</span>
                  <span v-if="m.next_due_date">Berikutnya: {{ formatDateStr(m.next_due_date) }}</span>
                </p>
              </div>
              <button v-if="canUpdate" @click="deleteMaintenance(m)" title="Hapus catatan" class="text-gray-300 hover:text-red-500 shrink-0">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>
              </button>
            </div>
          </li>
        </ol>
      </div>
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
import AppCard   from '@/components/ui/AppCard.vue'
import AppTable  from '@/components/ui/AppTable.vue'
import AppAlert  from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppModal  from '@/components/ui/AppModal.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'

const toast = useToastStore()
const auth = useAuthStore()
const canCreate = auth.hasPermission('assets.create')
const canUpdate = auth.hasPermission('assets.update')
const canDelete = auth.hasPermission('assets.delete')

const CONDITIONS = { baik: 'Baik', rusak_ringan: 'Rusak Ringan', rusak_berat: 'Rusak Berat', perbaikan: 'Dalam Perbaikan' }
const MTYPES = { rutin: 'Rutin', perbaikan: 'Perbaikan', penggantian: 'Penggantian Part', inspeksi: 'Inspeksi' }
function condCls(c) {
  return {
    'cond-baik': c === 'baik',
    'cond-ringan': c === 'rusak_ringan',
    'cond-berat': c === 'rusak_berat',
    'cond-perbaikan': c === 'perbaikan',
  }
}

const COLUMNS = [
  { key: 'name',        label: 'Perlengkapan' },
  { key: 'category',    label: 'Kategori' },
  { key: 'outlet_name', label: 'Outlet' },
  { key: 'quantity',    label: 'Jumlah' },
  { key: 'condition',   label: 'Kondisi' },
  { key: 'location',    label: 'Lokasi' },
  { key: 'maintenance', label: 'Perawatan' },
  { key: 'actions',     label: '' },
]

const assets = ref([])
const outlets = ref([])
const loading = ref(false)
const errorMsg = ref('')
const filterOutlet = ref('')
const filterCondition = ref('')
const search = ref('')

const outletFilterOptions = computed(() => [{ id: '', name: 'Semua outlet' }, ...outlets.value])
const categorySuggestions = computed(() => [...new Set(assets.value.map(a => a.category).filter(Boolean))])

function asArray(d) { return Array.isArray(d) ? d : (d?.data || []) }

async function load() {
  loading.value = true; errorMsg.value = ''
  try {
    const data = await assetsApi.list({
      outlet_id: filterOutlet.value || undefined,
      condition: filterCondition.value || undefined,
      search: search.value.trim() || undefined,
    })
    assets.value = asArray(data)
  } catch (e) {
    errorMsg.value = e?.message || 'Gagal memuat perlengkapan'
  } finally {
    loading.value = false
  }
}
let _t = null
function debouncedLoad() { clearTimeout(_t); _t = setTimeout(load, 350) }

async function loadOutlets() {
  try {
    const d = await outletsApi.myOutlets()
    outlets.value = d?.outlets ?? d ?? []
  } catch { outlets.value = [] }
}

// ── Asset CRUD ──
const assetModal = ref(false)
const editing = ref(null)
const saving = ref(false)
const form = ref({})
function blankForm() {
  return { outlet_id: filterOutlet.value || '', code: '', name: '', category: '', quantity: 1, unit: 'unit', condition: 'baik', location: '', purchase_date: '', purchase_price: 0, notes: '' }
}
function openCreate() { editing.value = null; form.value = blankForm(); assetModal.value = true }
function openEdit(a) {
  editing.value = a
  form.value = { outlet_id: a.outlet_id, code: a.code, name: a.name, category: a.category, quantity: a.quantity, unit: a.unit, condition: a.condition, location: a.location, purchase_date: a.purchase_date || '', purchase_price: a.purchase_price, notes: a.notes }
  assetModal.value = true
}
async function saveAsset() {
  if (!form.value.name?.trim()) { toast.error('Nama perlengkapan wajib diisi'); return }
  if (!editing.value && !form.value.outlet_id) { toast.error('Pilih outlet'); return }
  saving.value = true
  try {
    if (editing.value) await assetsApi.update(editing.value.id, form.value)
    else await assetsApi.create(form.value)
    toast.success(editing.value ? 'Perlengkapan diperbarui' : 'Perlengkapan ditambahkan')
    assetModal.value = false
    await load()
  } catch (e) { toast.error(e?.message || 'Gagal menyimpan') } finally { saving.value = false }
}
async function confirmDelete(a) {
  if (!window.confirm(`Hapus perlengkapan "${a.name}"? Histori perawatannya tetap tersimpan namun perlengkapan tak lagi tampil.`)) return
  try { await assetsApi.remove(a.id); toast.success('Perlengkapan dihapus'); await load() }
  catch (e) { toast.error(e?.message || 'Gagal menghapus') }
}

// ── Maintenance history ──
const historyModal = ref(false)
const activeAsset = ref(null)
const history = ref([])
const loadingHistory = ref(false)
const savingM = ref(false)
const mForm = ref({})
function blankM() { return { maintenance_date: todayDateString(), type: 'rutin', description: '', cost: 0, performed_by: '', condition_after: '', next_due_date: '' } }

async function openHistory(a) {
  activeAsset.value = a
  history.value = []
  mForm.value = blankM()
  historyModal.value = true
  loadingHistory.value = true
  try { history.value = asArray(await assetsApi.maintenances(a.id)) }
  catch (e) { toast.error(e?.message || 'Gagal memuat histori') }
  finally { loadingHistory.value = false }
}
async function saveMaintenance() {
  if (!mForm.value.description?.trim()) { toast.error('Deskripsi wajib diisi'); return }
  savingM.value = true
  try {
    await assetsApi.addMaintenance(activeAsset.value.id, mForm.value)
    toast.success('Perawatan dicatat')
    mForm.value = blankM()
    history.value = asArray(await assetsApi.maintenances(activeAsset.value.id))
    await load() // refresh count/last/condition in the list
    const fresh = assets.value.find(x => x.id === activeAsset.value.id)
    if (fresh) activeAsset.value = fresh
  } catch (e) { toast.error(e?.message || 'Gagal menyimpan perawatan') } finally { savingM.value = false }
}
async function deleteMaintenance(m) {
  if (!window.confirm('Hapus catatan perawatan ini?')) return
  try {
    await assetsApi.removeMaintenance(activeAsset.value.id, m.id)
    history.value = history.value.filter(x => x.id !== m.id)
    await load()
  } catch (e) { toast.error(e?.message || 'Gagal menghapus') }
}

onMounted(async () => { await loadOutlets(); await load() })
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

.cond-badge { display: inline-block; padding: .12rem .5rem; border-radius: 999px; font-size: .68rem; font-weight: 700; white-space: nowrap; }
.cond-baik { background: rgba(16,185,129,.13); color: #047857; }
.cond-ringan { background: rgba(245,158,11,.15); color: #b45309; }
.cond-berat { background: rgba(239,68,68,.13); color: #b91c1c; }
.cond-perbaikan { background: rgba(59,130,246,.13); color: #1d4ed8; }

.mtype-badge { display: inline-block; padding: .05rem .45rem; border-radius: 999px; font-size: .65rem; font-weight: 700; background: rgba(99,102,241,.12); color: #4338ca; }
</style>
