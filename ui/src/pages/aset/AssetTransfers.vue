<template>
  <div class="space-y-5">
    <div class="flex items-start justify-between flex-wrap gap-3">
      <div>
        <h1 class="text-xl font-bold text-gray-900">Mutasi Aset</h1>
        <p class="text-sm text-gray-500 mt-0.5">Perpindahan aset antar outlet beserta alasan dan pelaksananya.</p>
      </div>
      <AppButton v-if="canManage" @click="openCreate">
        <span class="btn-ic" v-html="ICONS.plus"></span>Mutasi Aset
      </AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />
    <AppAlert type="info" message="Mutasi memindahkan seluruh unit pada satu baris aset. Untuk memindahkan sebagian unit, pisahkan dulu menjadi baris aset tersendiri." />

    <!-- Ringkasan — flat, konsisten dengan halaman aset lain -->
    <div class="kpi-grid">
      <div class="kpi">
        <div class="kpi-label">Total Mutasi</div>
        <div class="kpi-val">{{ rows.length }}</div>
        <div class="kpi-sub">pada rentang &amp; filter saat ini</div>
      </div>
      <div class="kpi">
        <div class="kpi-label">Unit Dipindahkan</div>
        <div class="kpi-val">{{ totalQty }}</div>
        <div class="kpi-sub">unit total dari seluruh mutasi</div>
      </div>
      <div class="kpi">
        <div class="kpi-label">Outlet Terlibat</div>
        <div class="kpi-val">{{ outletsInvolved }}</div>
        <div class="kpi-sub">outlet asal atau tujuan</div>
      </div>
    </div>

    <!-- Filters -->
    <AppCard>
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <SearchSelect v-model="filterOutlet" :options="outletFilterOptions" placeholder="Semua outlet (asal/tujuan)" searchPlaceholder="Cari outlet…" @change="load" />
        <input v-model="from" @change="load" type="date" class="form-input" title="Dari tanggal" />
        <input v-model="to" @change="load" type="date" class="form-input" title="Sampai tanggal" />
      </div>
      <div v-if="hasActiveFilters" class="filter-summary">
        <span>{{ rows.length }} mutasi ditemukan</span>
        <button type="button" class="lnk" @click="resetFilters">Bersihkan filter</button>
      </div>
    </AppCard>

    <!-- List -->
    <AppCard :padding="false">
      <div class="sm:hidden">
        <div v-if="loading" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
        <div v-else-if="!rows.length" class="empty-block">
          <span class="empty-ic" v-html="ICONS.swap"></span>
          <p class="empty-title">{{ hasActiveFilters ? 'Tidak ada mutasi yang cocok' : 'Belum ada mutasi aset' }}</p>
          <p class="empty-desc">{{ hasActiveFilters ? 'Coba ubah outlet atau rentang tanggal.' : 'Riwayat perpindahan aset antar outlet akan muncul di sini.' }}</p>
        </div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="x in rows" :key="x.id" class="p-4 space-y-1">
            <p class="font-semibold text-gray-900 break-words">{{ x.asset_name }}</p>
            <p class="text-sm text-gray-700 flex items-center gap-1.5 flex-wrap">
              <span class="outlet-chip">{{ x.from_outlet_name }}</span>
              <span class="route-arrow" v-html="ICONS.arrowRight"></span>
              <span class="outlet-chip outlet-chip--to">{{ x.to_outlet_name }}</span>
            </p>
            <p class="text-xs text-gray-500">{{ formatDateStr(x.transfer_date) }} · {{ x.quantity }} unit<span v-if="x.performed_by"> · {{ x.performed_by }}</span></p>
            <p v-if="x.reason" class="text-xs text-gray-500">{{ x.reason }}</p>
          </li>
        </ul>
      </div>

      <!-- v-if di sini, bukan hanya emptyText — AppTable selalu merender baris
           "Tidak ada data" bawaannya sendiri saat rows kosong; tanpa v-if,
           blok kosong kustom di bawah akan tampil dobel dengan baris bawaan itu. -->
      <AppTable v-if="loading || rows.length" class="hidden sm:block" :columns="COLUMNS" :rows="rows" :loading="loading">
        <template #cell-date="{ row }">{{ formatDateStr(row.transfer_date) }}</template>
        <template #cell-asset="{ row }">
          <div>
            <p class="font-medium text-gray-900">{{ row.asset_name }}</p>
            <p v-if="row.asset_code" class="text-xs text-gray-400 font-mono">{{ row.asset_code }}</p>
          </div>
        </template>
        <template #cell-route="{ row }">
          <span class="outlet-chip">{{ row.from_outlet_name }}</span>
          <span class="route-arrow" v-html="ICONS.arrowRight"></span>
          <span class="outlet-chip outlet-chip--to">{{ row.to_outlet_name }}</span>
        </template>
        <template #cell-qty="{ row }">{{ row.quantity }} unit</template>
        <template #cell-reason="{ row }">{{ row.reason || '—' }}</template>
        <template #cell-by="{ row }">{{ row.performed_by || '—' }}</template>
      </AppTable>

      <div v-if="!loading && !rows.length" class="hidden sm:block empty-block">
        <span class="empty-ic" v-html="ICONS.swap"></span>
        <p class="empty-title">{{ hasActiveFilters ? 'Tidak ada mutasi yang cocok dengan filter' : 'Belum ada mutasi aset' }}</p>
        <p class="empty-desc">{{ hasActiveFilters ? 'Coba ubah outlet atau rentang tanggal.' : 'Pindahkan aset antar outlet dari tombol di atas — riwayatnya akan tercatat di sini.' }}</p>
        <button v-if="hasActiveFilters" type="button" class="lnk mt-1" @click="resetFilters">Bersihkan filter</button>
        <AppButton v-else-if="canManage" class="mt-1" size="sm" @click="openCreate">
          <span class="btn-ic" v-html="ICONS.plus"></span>Mutasi aset pertama
        </AppButton>
      </div>
    </AppCard>

    <!-- ══════════════════ Modal: Mutasi Aset ══════════════════ -->
    <!-- Dua kolom berdampingan supaya modal ini tetap muat satu layar tanpa
         perlu menggulir. Di layar sempit (<md) kembali bertumpuk. -->
    <AppModal v-model="modal" title="Mutasi Aset" size="2xl">
      <form class="grid md:grid-cols-2 gap-x-8 gap-y-5" @submit.prevent="save">
        <div class="space-y-5">
          <!-- Aset & Tujuan -->
          <section class="f-section">
            <div class="f-section-hd">
              <span class="f-section-ic f-section-ic--indigo" v-html="ICONS.tag"></span>
              <div>
                <h3 class="f-section-title">Aset &amp; Tujuan</h3>
                <p class="f-section-desc">Aset mana yang dipindah, dan ke outlet mana.</p>
              </div>
            </div>
            <div class="space-y-3">
              <div>
                <label class="lbl">Aset <span class="req">*</span></label>
                <SearchSelect v-model="form.asset_id" :options="assetOptions" placeholder="Pilih aset…" searchPlaceholder="Cari aset…" />
              </div>
              <div>
                <label class="lbl">Outlet Tujuan <span class="req">*</span></label>
                <SearchSelect v-model="form.to_outlet_id" :options="destinationOptions" placeholder="Pilih outlet tujuan…" searchPlaceholder="Cari outlet…" />
              </div>
            </div>

            <!-- Pratinjau rute — hanya tampil bila aset & tujuan sudah dipilih -->
            <div v-if="selectedAsset && form.to_outlet_id" class="preview-box">
              <span class="preview-ic" v-html="ICONS.swap"></span>
              <div class="text-sm flex items-center gap-1.5 flex-wrap">
                <span class="outlet-chip">{{ selectedAsset.outlet_name }}</span>
                <span class="route-arrow" v-html="ICONS.arrowRight"></span>
                <span class="outlet-chip outlet-chip--to">{{ destinationName }}</span>
                <span class="text-xs text-gray-500 ml-1">· {{ selectedAsset.quantity }} {{ selectedAsset.unit }}</span>
              </div>
            </div>
            <p v-else-if="selectedAsset" class="hint-text">
              Saat ini di <strong>{{ selectedAsset.outlet_name }}</strong> · {{ selectedAsset.quantity }} {{ selectedAsset.unit }}. Pilih outlet tujuan untuk melihat pratinjau rute.
            </p>
          </section>
        </div>

        <div class="space-y-5">
          <!-- Detail Mutasi -->
          <section class="f-section">
            <div class="f-section-hd">
              <span class="f-section-ic f-section-ic--sky" v-html="ICONS.pin"></span>
              <div>
                <h3 class="f-section-title">Detail Mutasi</h3>
                <p class="f-section-desc">Kapan dipindah, siapa yang melaksanakan, dan alasannya.</p>
              </div>
            </div>
            <div class="space-y-3">
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
                <textarea v-model="form.reason" rows="3" class="form-input" placeholder="mis. Kebutuhan outlet baru"></textarea>
              </div>
            </div>
          </section>
        </div>
      </form>

      <template #footer>
        <button type="button" class="btn-ghost" @click="modal = false">Batal</button>
        <AppButton :loading="saving" @click="save">Pindahkan Aset</AppButton>
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

// ── Ikon — bahasa visual yang sama dengan halaman aset lain.
const ICONS = {
  plus: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4"/></svg>',
  tag: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><path stroke-linecap="round" stroke-linejoin="round" d="M11.5 3H6a2 2 0 00-2 2v5.5a2 2 0 00.586 1.414l8.5 8.5a2 2 0 002.828 0l5.5-5.5a2 2 0 000-2.828l-8.5-8.5A2 2 0 0011.5 3z"/><circle cx="7.5" cy="7.5" r="1.1" fill="currentColor" stroke="none"/></svg>',
  pin: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><path stroke-linecap="round" stroke-linejoin="round" d="M12 21c-4.5-4.5-7-8.14-7-11a7 7 0 1114 0c0 2.86-2.5 6.5-7 11z"/><circle cx="12" cy="10" r="2.4"/></svg>',
  swap: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6"><path stroke-linecap="round" stroke-linejoin="round" d="M8 7h12m0 0l-4-4m4 4l-4 4M16 17H4m0 0l4 4m-4-4l4-4"/></svg>',
  arrowRight: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2"><path stroke-linecap="round" stroke-linejoin="round" d="M4 12h16m0 0l-6-6m6 6l-6 6"/></svg>',
}

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
const destinationName = computed(() => outlets.value.find(o => o.id === form.value.to_outlet_id)?.name || '')
const totalQty = computed(() => rows.value.reduce((s, x) => s + Number(x.quantity || 0), 0))
const outletsInvolved = computed(() => new Set(rows.value.flatMap(x => [x.from_outlet_id, x.to_outlet_id])).size)
const hasActiveFilters = computed(() => !!(filterOutlet.value || from.value || to.value))

function resetFilters() {
  filterOutlet.value = ''; from.value = ''; to.value = ''
  load()
}

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
.req { color: #ef4444; }
.btn-ghost { padding: .5rem 1rem; border-radius: .6rem; font-size: .85rem; font-weight: 600; color: #374151; background: #f3f4f6; }
.btn-ghost:hover { background: #e5e7eb; }
.lnk { color: #047857; font-weight: 600; text-decoration: none; background: none; border: none; cursor: pointer; font-size: inherit; padding: 0; }
.lnk:hover { text-decoration: underline; }

.btn-ic { display: inline-flex; margin-right: .35rem; vertical-align: -2px; }
.btn-ic :deep(svg) { width: .85rem; height: .85rem; }

/* ── Kartu ringkasan — flat ── */
.kpi-grid { display: grid; gap: .75rem; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); }
.kpi { border-radius: .85rem; padding: .85rem 1rem; background: #fff; border: 1px solid rgba(17,24,39,.08); box-shadow: 0 1px 2px rgba(16,24,40,.04); }
.kpi-label { font-size: .68rem; font-weight: 700; color: #6b7280; text-transform: uppercase; letter-spacing: .03em; }
.kpi-val { font-size: 1.3rem; font-weight: 800; color: #111827; line-height: 1.2; margin-top: .2rem; }
.kpi-sub { font-size: .71rem; color: #6b7280; margin-top: .15rem; }

.filter-summary { display: flex; align-items: center; justify-content: space-between; gap: .5rem; margin-top: .7rem; padding-top: .7rem; border-top: 1px solid rgba(17,24,39,.06); font-size: .75rem; color: #6b7280; }

.outlet-chip { display: inline-block; padding: .1rem .5rem; border-radius: .45rem; font-size: .72rem; font-weight: 600; background: rgba(107,114,128,.12); color: #374151; white-space: nowrap; }
.outlet-chip--to { background: rgba(16,185,129,.14); color: #047857; }
.route-arrow { display: inline-flex; color: #9ca3af; flex-shrink: 0; }
.route-arrow :deep(svg) { width: .85rem; height: .85rem; }

/* ── Keadaan kosong ── */
.empty-block { padding: 2.5rem 1.5rem; text-align: center; }
.empty-ic { display: inline-flex; width: 2.75rem; height: 2.75rem; border-radius: .9rem; align-items: center; justify-content: center; color: #9ca3af; background: #f3f4f6; margin-bottom: .7rem; }
.empty-ic :deep(svg) { width: 1.4rem; height: 1.4rem; }
.empty-title { font-size: .88rem; font-weight: 700; color: #374151; }
.empty-desc { font-size: .78rem; color: #9ca3af; margin-top: .25rem; max-width: 26rem; margin-inline: auto; line-height: 1.5; }

/* ── Bagian form modal ── */
.f-section { padding-top: 1.1rem; border-top: 1px solid rgba(17,24,39,.06); }
.f-section:first-child { padding-top: 0; border-top: none; }
.f-section-hd { display: flex; align-items: flex-start; gap: .65rem; margin-bottom: .85rem; }
.f-section-ic { flex-shrink: 0; width: 2.1rem; height: 2.1rem; border-radius: .65rem; display: flex; align-items: center; justify-content: center; }
.f-section-ic :deep(svg) { width: 1.1rem; height: 1.1rem; }
.f-section-ic--indigo { background: rgba(79,70,229,.1); color: #4338ca; }
.f-section-ic--sky { background: rgba(14,165,233,.1); color: #0369a1; }
.f-section-title { font-size: .88rem; font-weight: 700; color: #111827; line-height: 1.3; }
.f-section-desc { font-size: .74rem; color: #9ca3af; margin-top: .05rem; }

.hint-text { font-size: .72rem; color: #9ca3af; margin-top: .7rem; line-height: 1.5; }
.hint-text strong { color: #374151; }

.preview-box {
  display: flex; align-items: flex-start; gap: .6rem; margin-top: .85rem;
  padding: .7rem .8rem; border-radius: .7rem; background: rgba(16,185,129,.06); border: 1px solid rgba(16,185,129,.18);
}
.preview-ic { flex-shrink: 0; color: #047857; margin-top: .1rem; }
.preview-ic :deep(svg) { width: 1.1rem; height: 1.1rem; }
</style>
