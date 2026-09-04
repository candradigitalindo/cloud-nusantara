<template>
  <div class="space-y-5">
    <div class="flex items-start justify-between flex-wrap gap-3">
      <div>
        <h1 class="text-xl font-bold text-gray-900">Penghapusan Aset</h1>
        <p class="text-sm text-gray-500 mt-0.5">Pelepasan aset — dijual, dimusnahkan, dihibahkan, atau hilang — beserta laba/ruginya.</p>
      </div>
      <AppButton v-if="canManage" @click="openCreate">+ Hapus Aset</AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <div class="kpi-grid">
      <div class="kpi kpi--slate">
        <div class="kpi-label">Aset Dilepas</div>
        <div class="kpi-val">{{ rows.length }}</div>
        <div class="kpi-sub">{{ totalQty }} unit pada rentang ini</div>
      </div>
      <div class="kpi kpi--slate">
        <div class="kpi-label">Nilai Buku Dilepas</div>
        <div class="kpi-val">{{ formatRupiah(totals.book) }}</div>
        <div class="kpi-sub">nilai tercatat saat dihapus</div>
      </div>
      <div class="kpi kpi--slate">
        <div class="kpi-label">Hasil Penjualan</div>
        <div class="kpi-val">{{ formatRupiah(totals.proceeds) }}</div>
        <div class="kpi-sub">uang masuk dari pelepasan</div>
      </div>
      <div class="kpi" :class="totals.gainLoss < 0 ? 'kpi--red' : 'kpi--ok'">
        <div class="kpi-label">{{ totals.gainLoss < 0 ? 'Rugi Pelepasan' : 'Laba Pelepasan' }}</div>
        <div class="kpi-val">{{ formatRupiah(Math.abs(totals.gainLoss)) }}</div>
        <div class="kpi-sub">hasil − nilai buku</div>
      </div>
    </div>

    <AppCard>
      <div class="grid grid-cols-1 sm:grid-cols-4 gap-3">
        <SearchSelect v-model="filterOutlet" :options="outletFilterOptions" placeholder="Semua outlet" searchPlaceholder="Cari outlet…" @change="load" />
        <select v-model="filterMethod" @change="load" class="form-input">
          <option value="">Semua cara</option>
          <option v-for="(lbl, key) in METHODS" :key="key" :value="key">{{ lbl }}</option>
        </select>
        <input v-model="from" @change="load" type="date" class="form-input" title="Dari tanggal" />
        <input v-model="to" @change="load" type="date" class="form-input" title="Sampai tanggal" />
      </div>
    </AppCard>

    <AppCard :padding="false">
      <div class="sm:hidden">
        <div v-if="loading" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
        <div v-else-if="!rows.length" class="p-6 text-center text-sm text-gray-400">Belum ada penghapusan aset.</div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="x in rows" :key="x.id" class="p-4 space-y-1">
            <div class="flex items-start justify-between gap-2">
              <p class="font-semibold text-gray-900 break-words">{{ x.asset_name }}</p>
              <span class="method-badge shrink-0">{{ METHODS[x.method] || x.method }}</span>
            </div>
            <p class="text-xs text-gray-500">{{ formatDateStr(x.disposal_date) }} · {{ x.outlet_name }} · {{ x.quantity }} unit</p>
            <p class="text-sm text-gray-800">
              Hasil {{ formatRupiah(x.proceeds) }} − buku {{ formatRupiah(x.book_value_at_disposal) }} =
              <strong :class="x.gain_loss < 0 ? 'text-red-600' : 'text-emerald-700'">{{ formatRupiah(x.gain_loss) }}</strong>
            </p>
            <p v-if="x.reason" class="text-xs text-gray-500">{{ x.reason }}</p>
            <button v-if="canManage" @click="confirmRestore(x)" class="text-xs font-medium text-emerald-700">Batalkan penghapusan</button>
          </li>
        </ul>
      </div>

      <AppTable class="hidden sm:block" :columns="COLUMNS" :rows="rows" :loading="loading" emptyText="Belum ada penghapusan aset.">
        <template #cell-date="{ row }">{{ formatDateStr(row.disposal_date) }}</template>
        <template #cell-asset="{ row }">
          <div>
            <p class="font-medium text-gray-900">{{ row.asset_name }}</p>
            <p v-if="row.asset_code" class="text-xs text-gray-400 font-mono">{{ row.asset_code }}</p>
          </div>
        </template>
        <template #cell-method="{ row }"><span class="method-badge">{{ METHODS[row.method] || row.method }}</span></template>
        <template #cell-book="{ row }">{{ formatRupiah(row.book_value_at_disposal) }}</template>
        <template #cell-proceeds="{ row }">{{ formatRupiah(row.proceeds) }}</template>
        <template #cell-gainloss="{ row }">
          <strong :class="row.gain_loss < 0 ? 'text-red-600' : 'text-emerald-700'">{{ formatRupiah(row.gain_loss) }}</strong>
        </template>
        <template #cell-reason="{ row }">
          <span>{{ row.reason || '—' }}</span>
          <span v-if="row.approved_by" class="text-xs text-gray-400 block">disetujui {{ row.approved_by }}</span>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex justify-end">
            <button v-if="canManage" @click="confirmRestore(row)" class="text-emerald-700 hover:text-emerald-900 text-xs font-medium px-2 py-1 rounded hover:bg-emerald-50">Batalkan</button>
          </div>
        </template>
      </AppTable>
    </AppCard>

    <AppModal v-model="modal" title="Hapus Aset">
      <form class="space-y-3" @submit.prevent="save">
        <div>
          <label class="lbl">Aset <span class="text-red-500">*</span></label>
          <SearchSelect v-model="form.asset_id" :options="assetOptions" placeholder="Pilih aset…" searchPlaceholder="Cari aset…" />
          <p v-if="selectedAsset" class="text-xs text-gray-500 mt-1">
            {{ selectedAsset.outlet_name }} · {{ selectedAsset.quantity }} {{ selectedAsset.unit }} ·
            nilai buku saat ini <strong>{{ formatRupiah(selectedAsset.book_value) }}</strong>
          </p>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="lbl">Tanggal</label>
            <input v-model="form.disposal_date" type="date" class="form-input" />
          </div>
          <div>
            <label class="lbl">Cara Pelepasan</label>
            <select v-model="form.method" class="form-input">
              <option v-for="(lbl, key) in METHODS" :key="key" :value="key">{{ lbl }}</option>
            </select>
          </div>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="lbl">Hasil Penjualan</label>
            <input v-model.number="form.proceeds" type="number" min="0" class="form-input" :disabled="form.method !== 'dijual'" />
          </div>
          <div>
            <label class="lbl">Disetujui Oleh</label>
            <input v-model="form.approved_by" class="form-input" placeholder="Nama penyetuju" />
          </div>
        </div>
        <p v-if="selectedAsset" class="text-xs px-2.5 py-2 rounded-lg" :class="previewGainLoss < 0 ? 'bg-red-50 text-red-700' : 'bg-emerald-50 text-emerald-700'">
          {{ previewGainLoss < 0 ? 'Rugi' : 'Laba' }} pelepasan: <strong>{{ formatRupiah(Math.abs(previewGainLoss)) }}</strong>
          ({{ formatRupiah(form.proceeds || 0) }} − {{ formatRupiah(selectedAsset.book_value) }})
        </p>
        <div>
          <label class="lbl">Alasan <span class="text-red-500">*</span></label>
          <textarea v-model="form.reason" rows="2" class="form-input" required placeholder="mis. Rusak berat tidak ekonomis diperbaiki"></textarea>
        </div>
        <p class="text-xs text-gray-500">
          Aset akan keluar dari daftar aktif namun riwayatnya tetap tersimpan, dan penghapusan ini bisa dibatalkan.
        </p>
        <div class="flex justify-end gap-2 pt-1">
          <button type="button" class="btn-ghost" @click="modal = false">Batal</button>
          <AppButton type="submit" :loading="saving">Hapus Aset</AppButton>
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
const canManage = auth.hasPermission('assets.disposal.manage')

const METHODS = { dijual: 'Dijual', dimusnahkan: 'Dimusnahkan', hibah: 'Dihibahkan', hilang: 'Hilang' }

const COLUMNS = [
  { key: 'date',     label: 'Tanggal' },
  { key: 'asset',    label: 'Aset' },
  { key: 'outlet_name', label: 'Outlet' },
  { key: 'method',   label: 'Cara' },
  { key: 'book',     label: 'Nilai Buku' },
  { key: 'proceeds', label: 'Hasil' },
  { key: 'gainloss', label: 'Laba/Rugi' },
  { key: 'reason',   label: 'Alasan' },
  { key: 'actions',  label: '' },
]

const rows = ref([])
const assets = ref([])
const outlets = ref([])
const loading = ref(false)
const errorMsg = ref('')
const filterOutlet = ref('')
const filterMethod = ref('')
const from = ref('')
const to = ref('')

function asArray(d) { return Array.isArray(d) ? d : (d?.data || []) }
const outletFilterOptions = computed(() => [{ id: '', name: 'Semua outlet' }, ...outlets.value])
const assetOptions = computed(() => assets.value.map(a => ({ id: a.id, name: `${a.name}${a.code ? ' · ' + a.code : ''} — ${a.outlet_name}` })))
const selectedAsset = computed(() => assets.value.find(a => a.id === form.value.asset_id) || null)
const previewGainLoss = computed(() => Number(form.value.proceeds || 0) - Number(selectedAsset.value?.book_value || 0))
const totalQty = computed(() => rows.value.reduce((s, x) => s + Number(x.quantity || 0), 0))
const totals = computed(() => rows.value.reduce((t, x) => ({
  book: t.book + Number(x.book_value_at_disposal || 0),
  proceeds: t.proceeds + Number(x.proceeds || 0),
  gainLoss: t.gainLoss + Number(x.gain_loss || 0),
}), { book: 0, proceeds: 0, gainLoss: 0 }))

async function load() {
  loading.value = true; errorMsg.value = ''
  try {
    rows.value = asArray(await assetsApi.disposals({
      outlet_id: filterOutlet.value || undefined,
      method: filterMethod.value || undefined,
      from: from.value || undefined,
      to: to.value || undefined,
    }))
  } catch (e) {
    errorMsg.value = e?.message || 'Gagal memuat riwayat penghapusan'
  } finally {
    loading.value = false
  }
}
async function loadRefs() {
  try { assets.value = asArray(await assetsApi.list()) } catch { assets.value = [] }
  try { const r = await outletsApi.myOutlets(); outlets.value = r?.outlets ?? r ?? [] } catch { outlets.value = [] }
}

const modal = ref(false)
const saving = ref(false)
const form = ref({})
function openCreate() {
  form.value = { asset_id: '', disposal_date: todayDateString(), method: 'dijual', proceeds: 0, reason: '', approved_by: '' }
  modal.value = true
}
async function save() {
  if (!form.value.asset_id) { toast.error('Pilih aset'); return }
  if (!form.value.reason?.trim()) { toast.error('Alasan wajib diisi'); return }
  saving.value = true
  try {
    await assetsApi.dispose(form.value)
    toast.success('Aset dihapus dari daftar aktif')
    modal.value = false
    await Promise.all([load(), loadRefs()])
  } catch (e) { toast.error(e?.message || 'Gagal menghapus aset') } finally { saving.value = false }
}
async function confirmRestore(x) {
  if (!window.confirm(`Batalkan penghapusan "${x.asset_name}"? Aset kembali aktif dan catatan pelepasan ini dihapus.`)) return
  try {
    await assetsApi.restoreDisposal(x.id)
    toast.success('Penghapusan dibatalkan')
    await Promise.all([load(), loadRefs()])
  } catch (e) { toast.error(e?.message || 'Gagal membatalkan') }
}

onMounted(async () => { await loadRefs(); await load() })
</script>

<style scoped>
.form-input {
  width: 100%; padding: .5rem .7rem; border-radius: .6rem; font-size: .85rem;
  border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none;
}
.form-input:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }
.form-input:disabled { background: #f3f4f6; color: #9ca3af; }
.lbl { display: block; font-size: .72rem; font-weight: 700; color: #4b5563; margin-bottom: .25rem; }
.btn-ghost { padding: .5rem 1rem; border-radius: .6rem; font-size: .85rem; font-weight: 600; color: #374151; background: #f3f4f6; }
.btn-ghost:hover { background: #e5e7eb; }

.kpi-grid { display: grid; gap: .75rem; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); }
.kpi { border-radius: .85rem; padding: .85rem 1rem; background: #fff; border: 1px solid rgba(0,0,0,.07); box-shadow: 0 1px 2px rgba(16,24,40,.04); }
.kpi--slate { background: linear-gradient(180deg, #fff, #f8fafc); }
.kpi--ok { border-color: rgba(16,185,129,.25); background: linear-gradient(180deg, #fff, rgba(16,185,129,.05)); }
.kpi--red { border-color: rgba(239,68,68,.3); background: linear-gradient(180deg, #fff, rgba(239,68,68,.07)); }
.kpi--red .kpi-val { color: #b91c1c; }
.kpi-label { font-size: .7rem; font-weight: 700; color: #6b7280; text-transform: uppercase; letter-spacing: .02em; }
.kpi-val { font-size: 1.3rem; font-weight: 800; color: #111827; line-height: 1.2; margin-top: .15rem; }
.kpi-sub { font-size: .7rem; color: #6b7280; margin-top: .1rem; }

.method-badge { display: inline-block; padding: .12rem .5rem; border-radius: 999px; font-size: .68rem; font-weight: 700; background: rgba(107,114,128,.14); color: #4b5563; white-space: nowrap; }
</style>
