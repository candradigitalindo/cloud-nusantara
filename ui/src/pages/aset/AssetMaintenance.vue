<template>
  <div class="space-y-5">
    <div>
      <h1 class="text-xl font-bold text-gray-900">Perawatan Aset</h1>
      <p class="text-sm text-gray-500 mt-0.5">Jadwal yang perlu ditindak dan riwayat perawatan seluruh outlet.</p>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Ringkasan — flat, konsisten dengan Daftar Aset & Histori Perolehan -->
    <div class="kpi-grid">
      <div class="kpi" :class="dueAssets.length > 0 ? 'kpi--warn' : 'kpi--good'">
        <div class="kpi-label">Perlu Ditindak</div>
        <div class="kpi-val">{{ dueAssets.length }}</div>
        <div class="kpi-sub">{{ overdueCount }} terlambat · {{ dueAssets.length - overdueCount }} segera</div>
      </div>
      <div class="kpi">
        <div class="kpi-label">Catatan Ditemukan</div>
        <div class="kpi-val">{{ history.length }}</div>
        <div class="kpi-sub">pada rentang &amp; filter saat ini</div>
      </div>
      <div class="kpi">
        <div class="kpi-label">Total Biaya</div>
        <div class="kpi-val">{{ formatRupiah(totalCost) }}</div>
        <div class="kpi-sub">dari catatan yang tersaring</div>
      </div>
    </div>

    <!-- Jadwal yang perlu ditindak -->
    <AppCard>
      <div class="flex items-center justify-between flex-wrap gap-2 mb-3">
        <div class="flex items-center gap-2">
          <span class="sec-ic sec-ic--warn" v-html="ICONS.alert"></span>
          <h2 class="sec-title mb-0">Perlu Ditindak</h2>
        </div>
        <SearchSelect v-model="filterOutlet" :options="outletFilterOptions" placeholder="Semua outlet" searchPlaceholder="Cari outlet…" @change="loadAll" class="min-w-[180px]" />
      </div>

      <div v-if="loadingDue" class="text-sm text-gray-400 py-4 text-center">Memuat…</div>
      <div v-else-if="!dueAssets.length" class="empty-block empty-block--inline">
        <span class="empty-ic empty-ic--good" v-html="ICONS.shieldCheck"></span>
        <p class="empty-title">{{ filterOutlet ? 'Outlet ini aman' : 'Semua aset terkendali' }}</p>
        <p class="empty-desc">Tidak ada perawatan yang terlambat atau jatuh tempo dalam 7 hari{{ filterOutlet ? ' untuk outlet ini' : '' }}.</p>
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
            <AppButton v-if="canUpdate" size="sm" @click="openRecord(a)">
              <span class="btn-ic" v-html="ICONS.wrench"></span>Catat
            </AppButton>
          </div>
        </li>
      </ul>
    </AppCard>

    <!-- Riwayat -->
    <AppCard>
      <div class="flex items-center gap-2 mb-3">
        <span class="sec-ic sec-ic--indigo" v-html="ICONS.history"></span>
        <h2 class="sec-title mb-0">Riwayat Perawatan</h2>
      </div>
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
        <select v-model="filterType" @change="loadHistory" class="form-input">
          <option value="">Semua jenis</option>
          <option v-for="(lbl, key) in MTYPES" :key="key" :value="key">{{ lbl }}</option>
        </select>
        <input v-model="from" @change="loadHistory" type="date" class="form-input" title="Dari tanggal" />
        <input v-model="to" @change="loadHistory" type="date" class="form-input" title="Sampai tanggal" />
      </div>
      <div v-if="hasHistoryFilters" class="filter-summary">
        <span>{{ history.length }} catatan ditemukan</span>
        <button type="button" class="lnk" @click="resetHistoryFilters">Bersihkan filter</button>
      </div>
    </AppCard>

    <AppCard :padding="false">
      <div class="sm:hidden">
        <div v-if="loadingHistory" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
        <div v-else-if="!history.length" class="empty-block">
          <span class="empty-ic" v-html="ICONS.doc"></span>
          <p class="empty-title">{{ hasHistoryFilters ? 'Tidak ada catatan yang cocok' : 'Belum ada catatan perawatan' }}</p>
          <p class="empty-desc">{{ hasHistoryFilters ? 'Coba ubah jenis, rentang tanggal, atau outlet.' : 'Catatan akan muncul di sini setelah perawatan pertama dicatat.' }}</p>
        </div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="m in history" :key="m.id" class="p-4 space-y-1">
            <div class="flex items-start justify-between gap-2">
              <p class="font-semibold text-gray-900 break-words">{{ m.asset_name }}</p>
              <span class="mtype-badge shrink-0" :class="`mtype-badge--${m.type}`">
                <span class="mtype-ic" v-html="maintIcon(m.type)"></span>{{ MTYPES[m.type] || m.type }}
              </span>
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

      <!-- v-if di sini, bukan hanya emptyText — AppTable selalu merender baris
           "Tidak ada data" bawaannya sendiri saat rows kosong; tanpa v-if,
           blok kosong kustom di bawah akan tampil dobel dengan baris bawaan itu. -->
      <AppTable v-if="loadingHistory || history.length" class="hidden sm:block" :columns="COLUMNS" :rows="history" :loading="loadingHistory">
        <template #cell-date="{ row }">{{ formatDateStr(row.maintenance_date) }}</template>
        <template #cell-asset="{ row }">
          <div>
            <p class="font-medium text-gray-900">{{ row.asset_name }}</p>
            <p v-if="row.asset_code" class="text-xs text-gray-400 font-mono">{{ row.asset_code }}</p>
          </div>
        </template>
        <template #cell-type="{ row }">
          <span class="mtype-badge" :class="`mtype-badge--${row.type}`">
            <span class="mtype-ic" v-html="maintIcon(row.type)"></span>{{ MTYPES[row.type] || row.type }}
          </span>
        </template>
        <template #cell-cost="{ row }">{{ row.cost > 0 ? formatRupiah(row.cost) : '—' }}</template>
        <template #cell-next="{ row }">{{ row.next_due_date ? formatDateStr(row.next_due_date) : '—' }}</template>
      </AppTable>

      <div v-if="!loadingHistory && !history.length" class="hidden sm:block empty-block">
        <span class="empty-ic" v-html="ICONS.doc"></span>
        <p class="empty-title">{{ hasHistoryFilters ? 'Tidak ada catatan yang cocok dengan filter' : 'Belum ada catatan perawatan' }}</p>
        <p class="empty-desc">{{ hasHistoryFilters ? 'Coba ubah jenis, rentang tanggal, atau outlet.' : 'Catat perawatan pertama lewat tombol "Catat" pada daftar Perlu Ditindak di atas, atau dari Daftar Aset.' }}</p>
        <button v-if="hasHistoryFilters" type="button" class="lnk mt-1" @click="resetHistoryFilters">Bersihkan filter</button>
      </div>
    </AppCard>

    <!-- ══════════════════ Modal: Catat Perawatan ══════════════════ -->
    <!-- Dua kolom berdampingan supaya modal besar ini tetap muat satu layar
         tanpa perlu menggulir. Di layar sempit (<md) kembali bertumpuk. -->
    <AppModal v-model="recordModal" :title="`Catat Perawatan — ${activeAsset?.name || ''}`" size="2xl">
      <template v-if="activeAsset">
        <div class="asset-banner">
          <span>{{ activeAsset.outlet_name }}<span v-if="activeAsset.location"> · {{ activeAsset.location }}</span></span>
          <span class="cond-badge" :class="condCls(activeAsset.condition)">{{ CONDITIONS[activeAsset.condition] || activeAsset.condition }}</span>
        </div>

        <form class="grid md:grid-cols-2 gap-x-8 gap-y-5" @submit.prevent="saveRecord">
          <div class="space-y-5">
            <!-- Detail Perawatan -->
            <section class="f-section">
              <div class="f-section-hd">
                <span class="f-section-ic f-section-ic--indigo" v-html="ICONS.wrench"></span>
                <div>
                  <h3 class="f-section-title">Detail Perawatan</h3>
                  <p class="f-section-desc">Kapan dan jenis pekerjaan yang dilakukan.</p>
                </div>
              </div>
              <div class="space-y-3">
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
                  <label class="lbl">Deskripsi <span class="req">*</span></label>
                  <textarea v-model="mForm.description" rows="3" class="form-input" required placeholder="Pekerjaan yang dilakukan"></textarea>
                </div>
              </div>
            </section>
          </div>

          <div class="space-y-5">
            <!-- Hasil & Jadwal -->
            <section class="f-section">
              <div class="f-section-hd">
                <span class="f-section-ic f-section-ic--emerald" v-html="ICONS.banknote"></span>
                <div>
                  <h3 class="f-section-title">Hasil &amp; Jadwal</h3>
                  <p class="f-section-desc">Biaya, pelaksana, dan jadwal berikutnya.</p>
                </div>
              </div>
              <div class="space-y-3">
                <div class="grid grid-cols-2 gap-3">
                  <div>
                    <label class="lbl">Biaya</label>
                    <RupiahInput v-model="mForm.cost" placeholder="0" />
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
                <p class="hint-text">
                  <span class="btn-ic btn-ic--inline" v-html="ICONS.info"></span>
                  Mengosongkan jadwal berikutnya membuat aset ini dianggap belum terjadwal.
                </p>
              </div>
            </section>
          </div>
        </form>
      </template>

      <template #footer>
        <button type="button" class="btn-ghost" @click="recordModal = false">Batal</button>
        <AppButton :loading="savingM" @click="saveRecord">Simpan Perawatan</AppButton>
      </template>
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
import RupiahInput from '@/components/ui/RupiahInput.vue'

const toast = useToastStore()
const auth = useAuthStore()
const canUpdate = auth.hasPermission('assets.update')

const MTYPES = { rutin: 'Rutin', perbaikan: 'Perbaikan', penggantian: 'Penggantian Part', inspeksi: 'Inspeksi' }
const CONDITIONS = { baik: 'Baik', rusak_ringan: 'Rusak Ringan', rusak_berat: 'Rusak Berat', perbaikan: 'Dalam Perbaikan' }

// ── Ikon — bahasa visual yang sama dengan Daftar Aset & Histori Perolehan.
const ICONS = {
  wrench: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M14.7 6.3a1 1 0 000 1.4l1.6 1.6a1 1 0 001.4 0l3.35-3.35a6 6 0 01-7.94 7.94l-6.7 6.7a2.12 2.12 0 01-3-3l6.7-6.7a6 6 0 017.94-7.94L14.7 6.3z"/></svg>',
  banknote: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><rect x="2.5" y="6" width="19" height="12" rx="2"/><circle cx="12" cy="12" r="2.3"/><path stroke-linecap="round" d="M5.5 9.5h.01M18.5 14.5h.01"/></svg>',
  doc: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><path stroke-linecap="round" stroke-linejoin="round" d="M9 2h6a1 1 0 011 1v1h2a2 2 0 012 2v13a2 2 0 01-2 2H6a2 2 0 01-2-2V6a2 2 0 012-2h2V3a1 1 0 011-1z"/><line x1="8" y1="11" x2="16" y2="11"/><line x1="8" y1="15" x2="13" y2="15"/></svg>',
  history: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M3 3v5h5"/><path stroke-linecap="round" stroke-linejoin="round" d="M3.05 13a9 9 0 106.02-9.36"/><path stroke-linecap="round" stroke-linejoin="round" d="M12 7v5l3.5 2"/></svg>',
  alert: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v4m0 4h.01M10.29 3.86l-8.18 14.18A1.5 1.5 0 003.5 20.5h17a1.5 1.5 0 001.39-2.46L13.71 3.86a1.5 1.5 0 00-2.42 0z"/></svg>',
  shieldCheck: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6"><path stroke-linecap="round" stroke-linejoin="round" d="M12 2.5l8 3.5v5.5c0 5-3.4 8.7-8 10-4.6-1.3-8-5-8-10V6l8-3.5z"/><path stroke-linecap="round" stroke-linejoin="round" d="M8.5 12l2.5 2.5L16 9"/></svg>',
  info: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="9"/><path stroke-linecap="round" d="M12 11v5"/><path stroke-linecap="round" d="M12 8h.01"/></svg>',
  refresh: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg>',
  swap: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M8 7h12m0 0l-4-4m4 4l-4 4M16 17H4m0 0l4 4m-4-4l4-4"/></svg>',
  checkCircle: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>',
}
const MAINT_ICONS = { rutin: ICONS.refresh, perbaikan: ICONS.wrench, penggantian: ICONS.swap, inspeksi: ICONS.checkCircle }
function maintIcon(type) { return MAINT_ICONS[type] || ICONS.refresh }

function condCls(c) {
  return {
    'cond-baik': c === 'baik',
    'cond-ringan': c === 'rusak_ringan',
    'cond-berat': c === 'rusak_berat',
    'cond-perbaikan': c === 'perbaikan',
  }
}

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
const overdueCount = computed(() => dueAssets.value.filter(a => a.due_in_days < 0).length)
const hasHistoryFilters = computed(() => !!(filterOutlet.value || filterType.value || from.value || to.value))

function resetHistoryFilters() {
  filterType.value = ''; from.value = ''; to.value = ''
  loadHistory()
}

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
.sec-title { font-size: .8rem; font-weight: 800; color: #374151; text-transform: uppercase; letter-spacing: .03em; }
.sec-ic { display: inline-flex; width: 1.6rem; height: 1.6rem; border-radius: .5rem; align-items: center; justify-content: center; flex-shrink: 0; }
.sec-ic :deep(svg) { width: .9rem; height: .9rem; }
.sec-ic--warn { background: rgba(217,119,6,.12); color: #b45309; }
.sec-ic--indigo { background: rgba(79,70,229,.1); color: #4338ca; }

.form-input {
  width: 100%; padding: .5rem .7rem; border-radius: .6rem; font-size: .85rem;
  border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none;
}
.form-input:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }
.lbl { display: block; font-size: .72rem; font-weight: 700; color: #4b5563; margin-bottom: .25rem; }
.req { color: #ef4444; }
.btn-ghost { padding: .5rem 1rem; border-radius: .6rem; font-size: .85rem; font-weight: 600; color: #374151; background: #f3f4f6; }
.btn-ghost:hover { background: #e5e7eb; }
.lnk { color: #047857; font-weight: 600; text-decoration: none; background: none; border: none; cursor: pointer; font-size: inherit; padding: 0; }
.lnk:hover { text-decoration: underline; }

.btn-ic { display: inline-flex; margin-right: .35rem; vertical-align: -2px; }
.btn-ic :deep(svg) { width: .85rem; height: .85rem; }
.btn-ic--inline { margin-right: .25rem; vertical-align: -3px; }
.btn-ic--inline :deep(svg) { width: .8rem; height: .8rem; }

/* ── Kartu ringkasan — flat ── */
.kpi-grid { display: grid; gap: .75rem; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); }
.kpi { border-radius: .85rem; padding: .85rem 1rem; background: #fff; border: 1px solid rgba(17,24,39,.08); box-shadow: 0 1px 2px rgba(16,24,40,.04); }
.kpi-label { font-size: .68rem; font-weight: 700; color: #6b7280; text-transform: uppercase; letter-spacing: .03em; }
.kpi-val { font-size: 1.3rem; font-weight: 800; color: #111827; line-height: 1.2; margin-top: .2rem; }
.kpi-sub { font-size: .71rem; color: #6b7280; margin-top: .15rem; }
.kpi--good .kpi-val { color: #047857; }
.kpi--warn { border-color: rgba(217,119,6,.3); background: rgba(217,119,6,.04); }
.kpi--warn .kpi-val { color: #b45309; }

.filter-summary { display: flex; align-items: center; justify-content: space-between; gap: .5rem; margin-top: .7rem; padding-top: .7rem; border-top: 1px solid rgba(17,24,39,.06); font-size: .75rem; color: #6b7280; }

.due-badge { display: inline-block; padding: .12rem .5rem; border-radius: 999px; font-size: .68rem; font-weight: 700; white-space: nowrap; }
.due-overdue { background: rgba(239,68,68,.13); color: #b91c1c; }
.due-soon { background: rgba(245,158,11,.15); color: #b45309; }

.mtype-badge { display: inline-flex; align-items: center; gap: .3rem; padding: .1rem .5rem; border-radius: 999px; font-size: .65rem; font-weight: 700; white-space: nowrap; }
.mtype-ic { display: inline-flex; }
.mtype-ic :deep(svg) { width: .7rem; height: .7rem; }
.mtype-badge--rutin { background: rgba(99,102,241,.12); color: #4338ca; }
.mtype-badge--perbaikan { background: rgba(245,158,11,.14); color: #b45309; }
.mtype-badge--penggantian { background: rgba(14,165,233,.13); color: #0369a1; }
.mtype-badge--inspeksi { background: rgba(16,185,129,.13); color: #047857; }

.cond-badge { display: inline-block; padding: .12rem .5rem; border-radius: 999px; font-size: .68rem; font-weight: 700; white-space: nowrap; }
.cond-baik { background: rgba(16,185,129,.13); color: #047857; }
.cond-ringan { background: rgba(245,158,11,.15); color: #b45309; }
.cond-berat { background: rgba(239,68,68,.13); color: #b91c1c; }
.cond-perbaikan { background: rgba(59,130,246,.13); color: #1d4ed8; }

/* ── Keadaan kosong ── */
.empty-block { padding: 2.5rem 1.5rem; text-align: center; }
.empty-block--inline { padding: 1.5rem 1rem; }
.empty-ic { display: inline-flex; width: 2.75rem; height: 2.75rem; border-radius: .9rem; align-items: center; justify-content: center; color: #9ca3af; background: #f3f4f6; margin-bottom: .7rem; }
.empty-ic--good { color: #047857; background: rgba(16,185,129,.1); }
.empty-ic :deep(svg) { width: 1.4rem; height: 1.4rem; }
.empty-title { font-size: .88rem; font-weight: 700; color: #374151; }
.empty-desc { font-size: .78rem; color: #9ca3af; margin-top: .25rem; max-width: 26rem; margin-inline: auto; line-height: 1.5; }

/* ── Modal: banner konteks aset ── */
.asset-banner {
  display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: .5rem;
  font-size: .78rem; color: #6b7280; background: #f9fafb; border-radius: .6rem; padding: .55rem .8rem; margin-bottom: 1.1rem;
}

/* ── Bagian form modal ── */
.f-section { padding-top: 1.1rem; border-top: 1px solid rgba(17,24,39,.06); }
.f-section:first-child { padding-top: 0; border-top: none; }
.f-section-hd { display: flex; align-items: flex-start; gap: .65rem; margin-bottom: .85rem; }
.f-section-ic { flex-shrink: 0; width: 2.1rem; height: 2.1rem; border-radius: .65rem; display: flex; align-items: center; justify-content: center; }
.f-section-ic :deep(svg) { width: 1.1rem; height: 1.1rem; }
.f-section-ic--indigo { background: rgba(79,70,229,.1); color: #4338ca; }
.f-section-ic--emerald { background: rgba(16,185,129,.1); color: #047857; }
.f-section-title { font-size: .88rem; font-weight: 700; color: #111827; line-height: 1.3; }
.f-section-desc { font-size: .74rem; color: #9ca3af; margin-top: .05rem; }

/* Bukan display:flex — .btn-ic--inline sudah inline-flex sendiri; membuat
   .hint-text jadi flex container malah menghilangkan spasi antara teks polos
   dan elemen di sekitarnya karena anonymous flex item boundary. */
.hint-text { font-size: .72rem; color: #9ca3af; line-height: 1.5; }
</style>
