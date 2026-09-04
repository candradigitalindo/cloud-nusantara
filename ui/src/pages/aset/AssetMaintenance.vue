<template>
  <div class="space-y-5">
    <div>
      <h1 class="text-xl font-bold text-gray-900">Perawatan Aset</h1>
      <p class="text-sm text-gray-500 mt-0.5">Jadwal yang perlu ditindak dan riwayat perawatan seluruh outlet.</p>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Jadwal yang perlu ditindak -->
    <AppCard>
      <div class="flex items-center justify-between flex-wrap gap-2 mb-3">
        <h2 class="sec-title mb-0">Perlu Ditindak</h2>
        <SearchSelect v-model="filterOutlet" :options="outletFilterOptions" placeholder="Semua outlet" searchPlaceholder="Cari outlet…" @change="loadAll" class="min-w-[180px]" />
      </div>

      <div v-if="loadingDue" class="text-sm text-gray-400 py-4 text-center">Memuat…</div>
      <div v-else-if="!dueAssets.length" class="text-sm text-gray-400 py-4 text-center">
        Tidak ada perawatan yang terlambat atau jatuh tempo dalam 7 hari.
      </div>
      <ul v-else class="divide-y divide-gray-100">
        <li v-for="a in dueAssets" :key="a.id" class="py-2.5 flex items-center justify-between gap-3 flex-wrap">
          <div class="min-w-0">
            <p class="font-medium text-gray-900 text-sm">{{ a.name }}</p>
            <p class="text-xs text-gray-500">
              {{ a.outlet_name }}<span v-if="a.location"> · {{ a.location }}</span>
              · terakhir dirawat {{ a.last_maintenance ? formatDateStr(a.last_maintenance) : 'belum pernah' }}
            </p>
          </div>
          <div class="flex items-center gap-2 shrink-0">
            <div class="text-right">
              <span class="due-badge" :class="a.due_in_days < 0 ? 'due-overdue' : 'due-soon'">
                {{ a.due_in_days < 0 ? `Terlambat ${Math.abs(a.due_in_days)} hari` : (a.due_in_days === 0 ? 'Hari ini' : `${a.due_in_days} hari lagi`) }}
              </span>
              <span class="text-xs text-gray-400 block mt-0.5">{{ formatDateStr(a.next_due_date) }}</span>
            </div>
            <AppButton v-if="canUpdate" size="sm" @click="openRecord(a)">Catat</AppButton>
          </div>
        </li>
      </ul>
    </AppCard>

    <!-- Riwayat -->
    <AppCard>
      <h2 class="sec-title">Riwayat Perawatan</h2>
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
        <select v-model="filterType" @change="loadHistory" class="form-input">
          <option value="">Semua jenis</option>
          <option v-for="(lbl, key) in MTYPES" :key="key" :value="key">{{ lbl }}</option>
        </select>
        <input v-model="from" @change="loadHistory" type="date" class="form-input" title="Dari tanggal" />
        <input v-model="to" @change="loadHistory" type="date" class="form-input" title="Sampai tanggal" />
        <div class="kpi-inline">
          <span class="kpi-inline-label">Total biaya</span>
          <span class="kpi-inline-val">{{ formatRupiah(totalCost) }}</span>
        </div>
      </div>
    </AppCard>

    <AppCard :padding="false">
      <div class="sm:hidden">
        <div v-if="loadingHistory" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
        <div v-else-if="!history.length" class="p-6 text-center text-sm text-gray-400">Belum ada catatan perawatan.</div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="m in history" :key="m.id" class="p-4 space-y-1">
            <div class="flex items-start justify-between gap-2">
              <p class="font-semibold text-gray-900 break-words">{{ m.asset_name }}</p>
              <span class="mtype-badge shrink-0">{{ MTYPES[m.type] || m.type }}</span>
            </div>
            <p class="text-xs text-gray-500">{{ formatDateStr(m.maintenance_date) }} · {{ m.outlet_name }}</p>
            <p class="text-sm text-gray-700 break-words">{{ m.description }}</p>
            <p class="text-xs text-gray-500">
              <span v-if="m.cost > 0">{{ formatRupiah(m.cost) }}</span>
              <span v-if="m.performed_by"> · {{ m.performed_by }}</span>
              <span v-if="m.next_due_date"> · berikutnya {{ formatDateStr(m.next_due_date) }}</span>
            </p>
          </li>
        </ul>
      </div>

      <AppTable class="hidden sm:block" :columns="COLUMNS" :rows="history" :loading="loadingHistory" emptyText="Belum ada catatan perawatan.">
        <template #cell-date="{ row }">{{ formatDateStr(row.maintenance_date) }}</template>
        <template #cell-asset="{ row }">
          <div>
            <p class="font-medium text-gray-900">{{ row.asset_name }}</p>
            <p v-if="row.asset_code" class="text-xs text-gray-400 font-mono">{{ row.asset_code }}</p>
          </div>
        </template>
        <template #cell-type="{ row }"><span class="mtype-badge">{{ MTYPES[row.type] || row.type }}</span></template>
        <template #cell-cost="{ row }">{{ row.cost > 0 ? formatRupiah(row.cost) : '—' }}</template>
        <template #cell-next="{ row }">{{ row.next_due_date ? formatDateStr(row.next_due_date) : '—' }}</template>
      </AppTable>
    </AppCard>

    <!-- Catat perawatan -->
    <AppModal v-model="recordModal" :title="`Catat Perawatan — ${activeAsset?.name || ''}`">
      <form v-if="activeAsset" class="space-y-3" @submit.prevent="saveRecord">
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
          <textarea v-model="mForm.description" rows="2" class="form-input" required placeholder="Pekerjaan yang dilakukan"></textarea>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="lbl">Biaya</label>
            <input v-model.number="mForm.cost" type="number" min="0" class="form-input" />
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
        <p class="text-xs text-gray-500">Mengosongkan jadwal berikutnya membuat aset ini dianggap belum terjadwal.</p>
        <div class="flex justify-end gap-2 pt-1">
          <button type="button" class="btn-ghost" @click="recordModal = false">Batal</button>
          <AppButton type="submit" :loading="savingM">Simpan</AppButton>
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
import { useRealtime } from '@/utils/realtime.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppTable from '@/components/ui/AppTable.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppModal from '@/components/ui/AppModal.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'

const toast = useToastStore()
const auth = useAuthStore()
const canUpdate = auth.hasPermission('assets.update')

const MTYPES = { rutin: 'Rutin', perbaikan: 'Perbaikan', penggantian: 'Penggantian Part', inspeksi: 'Inspeksi' }
const CONDITIONS = { baik: 'Baik', rusak_ringan: 'Rusak Ringan', rusak_berat: 'Rusak Berat', perbaikan: 'Dalam Perbaikan' }

const COLUMNS = [
  { key: 'date',        label: 'Tanggal' },
  { key: 'asset',       label: 'Aset' },
  { key: 'outlet_name', label: 'Outlet' },
  { key: 'type',        label: 'Jenis' },
  { key: 'description', label: 'Pekerjaan' },
  { key: 'cost',        label: 'Biaya' },
  { key: 'performed_by', label: 'Pelaksana' },
  { key: 'next',        label: 'Jadwal Berikutnya' },
]

const dueAssets = ref([])
const history = ref([])
const outlets = ref([])
const loadingDue = ref(false)
const loadingHistory = ref(false)
const errorMsg = ref('')
const filterOutlet = ref('')
const filterType = ref('')
const from = ref('')
const to = ref('')

function asArray(d) { return Array.isArray(d) ? d : (d?.data || []) }
const outletFilterOptions = computed(() => [{ id: '', name: 'Semua outlet' }, ...outlets.value])
const totalCost = computed(() => history.value.reduce((s, m) => s + Number(m.cost || 0), 0))

// Dua panggilan terpisah: yang terlambat selalu di atas yang segera jatuh tempo.
async function loadDue() {
  loadingDue.value = true; errorMsg.value = ''
  try {
    const params = { outlet_id: filterOutlet.value || undefined }
    const [overdue, soon] = await Promise.all([
      assetsApi.list({ ...params, due: 'overdue' }),
      assetsApi.list({ ...params, due: 'due_soon' }),
    ])
    dueAssets.value = [...asArray(overdue), ...asArray(soon)]
  } catch (e) {
    errorMsg.value = e?.message || 'Gagal memuat jadwal perawatan'
  } finally {
    loadingDue.value = false
  }
}

async function loadHistory() {
  loadingHistory.value = true
  try {
    history.value = asArray(await assetsApi.allMaintenances({
      outlet_id: filterOutlet.value || undefined,
      type: filterType.value || undefined,
      from: from.value || undefined,
      to: to.value || undefined,
    }))
  } catch (e) {
    errorMsg.value = e?.message || 'Gagal memuat riwayat perawatan'
  } finally {
    loadingHistory.value = false
  }
}

async function loadAll() { await Promise.all([loadDue(), loadHistory()]) }

async function loadOutlets() {
  try { const r = await outletsApi.myOutlets(); outlets.value = r?.outlets ?? r ?? [] } catch { outlets.value = [] }
}

const recordModal = ref(false)
const activeAsset = ref(null)
const savingM = ref(false)
const mForm = ref({})
function openRecord(a) {
  activeAsset.value = a
  mForm.value = { maintenance_date: todayDateString(), type: 'rutin', description: '', cost: 0, performed_by: '', condition_after: '', next_due_date: '' }
  recordModal.value = true
}
async function saveRecord() {
  if (!mForm.value.description?.trim()) { toast.error('Deskripsi wajib diisi'); return }
  savingM.value = true
  try {
    await assetsApi.addMaintenance(activeAsset.value.id, mForm.value)
    toast.success('Perawatan dicatat')
    recordModal.value = false
    await loadAll()
  } catch (e) { toast.error(e?.message || 'Gagal menyimpan') } finally { savingM.value = false }
}

useRealtime(['asset_alert'], loadAll)
onMounted(async () => { await loadOutlets(); await loadAll() })
</script>

<style scoped>
.sec-title { font-size: .8rem; font-weight: 800; color: #374151; text-transform: uppercase; letter-spacing: .03em; margin-bottom: .6rem; }
.form-input {
  width: 100%; padding: .5rem .7rem; border-radius: .6rem; font-size: .85rem;
  border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none;
}
.form-input:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }
.lbl { display: block; font-size: .72rem; font-weight: 700; color: #4b5563; margin-bottom: .25rem; }
.btn-ghost { padding: .5rem 1rem; border-radius: .6rem; font-size: .85rem; font-weight: 600; color: #374151; background: #f3f4f6; }
.btn-ghost:hover { background: #e5e7eb; }

.kpi-inline { display: flex; flex-direction: column; justify-content: center; padding: .35rem .7rem; border-radius: .6rem; background: #f8fafc; border: 1px solid rgba(0,0,0,.06); }
.kpi-inline-label { font-size: .65rem; font-weight: 700; color: #6b7280; text-transform: uppercase; }
.kpi-inline-val { font-size: .95rem; font-weight: 800; color: #111827; }

.due-badge { display: inline-block; padding: .12rem .5rem; border-radius: 999px; font-size: .68rem; font-weight: 700; white-space: nowrap; }
.due-overdue { background: rgba(239,68,68,.13); color: #b91c1c; }
.due-soon { background: rgba(245,158,11,.15); color: #b45309; }
.mtype-badge { display: inline-block; padding: .05rem .45rem; border-radius: 999px; font-size: .65rem; font-weight: 700; background: rgba(99,102,241,.12); color: #4338ca; white-space: nowrap; }
</style>
