<template>
  <div class="space-y-5">
    <div class="flex items-start justify-between flex-wrap gap-3">
      <div>
        <h1 class="text-xl font-bold text-gray-900">Mutasi Aset</h1>
        <p class="text-sm text-gray-500 mt-0.5">Perpindahan aset antar outlet beserta alasan dan pelaksananya.</p>
      </div>
      <AppButton v-if="canManage" @click="openCreate">+ Mutasi Aset</AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />
    <AppAlert type="info" message="Mutasi memindahkan seluruh unit pada satu baris aset. Untuk memindahkan sebagian unit, pisahkan dulu menjadi baris aset tersendiri." />

    <AppCard>
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <SearchSelect v-model="filterOutlet" :options="outletFilterOptions" placeholder="Semua outlet (asal/tujuan)" searchPlaceholder="Cari outlet…" @change="load" />
        <input v-model="from" @change="load" type="date" class="form-input" title="Dari tanggal" />
        <input v-model="to" @change="load" type="date" class="form-input" title="Sampai tanggal" />
      </div>
    </AppCard>

    <AppCard :padding="false">
      <div class="sm:hidden">
        <div v-if="loading" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
        <div v-else-if="!rows.length" class="p-6 text-center text-sm text-gray-400">Belum ada mutasi.</div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="x in rows" :key="x.id" class="p-4 space-y-1">
            <p class="font-semibold text-gray-900 break-words">{{ x.asset_name }}</p>
            <p class="text-sm text-gray-700">
              <span class="outlet-chip">{{ x.from_outlet_name }}</span>
              <span class="mx-1.5 text-gray-400">→</span>
              <span class="outlet-chip outlet-chip--to">{{ x.to_outlet_name }}</span>
            </p>
            <p class="text-xs text-gray-500">{{ formatDateStr(x.transfer_date) }} · {{ x.quantity }} unit<span v-if="x.performed_by"> · {{ x.performed_by }}</span></p>
            <p v-if="x.reason" class="text-xs text-gray-500">{{ x.reason }}</p>
          </li>
        </ul>
      </div>

      <AppTable class="hidden sm:block" :columns="COLUMNS" :rows="rows" :loading="loading" emptyText="Belum ada mutasi aset.">
        <template #cell-date="{ row }">{{ formatDateStr(row.transfer_date) }}</template>
        <template #cell-asset="{ row }">
          <div>
            <p class="font-medium text-gray-900">{{ row.asset_name }}</p>
            <p v-if="row.asset_code" class="text-xs text-gray-400 font-mono">{{ row.asset_code }}</p>
          </div>
        </template>
        <template #cell-route="{ row }">
          <span class="outlet-chip">{{ row.from_outlet_name }}</span>
          <span class="mx-1.5 text-gray-400">→</span>
          <span class="outlet-chip outlet-chip--to">{{ row.to_outlet_name }}</span>
        </template>
        <template #cell-qty="{ row }">{{ row.quantity }} unit</template>
        <template #cell-reason="{ row }">{{ row.reason || '—' }}</template>
        <template #cell-by="{ row }">{{ row.performed_by || '—' }}</template>
      </AppTable>
    </AppCard>

    <AppModal v-model="modal" title="Mutasi Aset">
      <form class="space-y-3" @submit.prevent="save">
        <div>
          <label class="lbl">Aset <span class="text-red-500">*</span></label>
          <SearchSelect v-model="form.asset_id" :options="assetOptions" placeholder="Pilih aset…" searchPlaceholder="Cari aset…" />
          <p v-if="selectedAsset" class="text-xs text-gray-500 mt-1">
            Saat ini di <strong>{{ selectedAsset.outlet_name }}</strong> · {{ selectedAsset.quantity }} {{ selectedAsset.unit }}
          </p>
        </div>
        <div>
          <label class="lbl">Outlet Tujuan <span class="text-red-500">*</span></label>
          <SearchSelect v-model="form.to_outlet_id" :options="destinationOptions" placeholder="Pilih outlet tujuan…" searchPlaceholder="Cari outlet…" />
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="lbl">Tanggal Mutasi</label>
            <input v-model="form.transfer_date" type="date" class="form-input" />
          </div>
          <div>
            <label class="lbl">Pelaksana</label>
            <input v-model="form.performed_by" class="form-input" placeholder="Nama petugas" />
          </div>
        </div>
        <div>
          <label class="lbl">Alasan</label>
          <textarea v-model="form.reason" rows="2" class="form-input" placeholder="mis. Kebutuhan outlet baru"></textarea>
        </div>
        <div class="flex justify-end gap-2 pt-1">
          <button type="button" class="btn-ghost" @click="modal = false">Batal</button>
          <AppButton type="submit" :loading="saving">Pindahkan</AppButton>
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
import { formatDateStr, todayDateString } from '@/utils/format.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppTable from '@/components/ui/AppTable.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppModal from '@/components/ui/AppModal.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'

const toast = useToastStore()
const auth = useAuthStore()
const canManage = auth.hasPermission('assets.transfer.manage')

const COLUMNS = [
  { key: 'date',   label: 'Tanggal' },
  { key: 'asset',  label: 'Aset' },
  { key: 'route',  label: 'Perpindahan' },
  { key: 'qty',    label: 'Jumlah' },
  { key: 'reason', label: 'Alasan' },
  { key: 'by',     label: 'Pelaksana' },
]

const rows = ref([])
const assets = ref([])
const outlets = ref([])
const loading = ref(false)
const errorMsg = ref('')
const filterOutlet = ref('')
const from = ref('')
const to = ref('')

function asArray(d) { return Array.isArray(d) ? d : (d?.data || []) }
const outletFilterOptions = computed(() => [{ id: '', name: 'Semua outlet' }, ...outlets.value])
const assetOptions = computed(() => assets.value.map(a => ({ id: a.id, name: `${a.name}${a.code ? ' · ' + a.code : ''} — ${a.outlet_name}` })))
const selectedAsset = computed(() => assets.value.find(a => a.id === form.value.asset_id) || null)
// Outlet asal disembunyikan supaya mutasi ke diri sendiri tidak bisa dipilih.
const destinationOptions = computed(() => outlets.value.filter(o => o.id !== selectedAsset.value?.outlet_id))

async function load() {
  loading.value = true; errorMsg.value = ''
  try {
    rows.value = asArray(await assetsApi.transfers({
      outlet_id: filterOutlet.value || undefined,
      from: from.value || undefined,
      to: to.value || undefined,
    }))
  } catch (e) {
    errorMsg.value = e?.message || 'Gagal memuat riwayat mutasi'
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
  form.value = { asset_id: '', to_outlet_id: '', transfer_date: todayDateString(), reason: '', performed_by: '' }
  modal.value = true
}
async function save() {
  if (!form.value.asset_id) { toast.error('Pilih aset'); return }
  if (!form.value.to_outlet_id) { toast.error('Pilih outlet tujuan'); return }
  saving.value = true
  try {
    await assetsApi.transfer(form.value)
    toast.success('Aset dipindahkan')
    modal.value = false
    await Promise.all([load(), loadRefs()])
  } catch (e) { toast.error(e?.message || 'Gagal memindahkan aset') } finally { saving.value = false }
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

.outlet-chip { display: inline-block; padding: .1rem .45rem; border-radius: .4rem; font-size: .72rem; font-weight: 600; background: rgba(107,114,128,.12); color: #374151; }
.outlet-chip--to { background: rgba(16,185,129,.14); color: #047857; }
</style>
