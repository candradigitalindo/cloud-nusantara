<template>
  <div class="space-y-5">
    <div class="flex items-start justify-between flex-wrap gap-3">
      <div>
        <h1 class="text-xl font-bold text-gray-900">Penghapusan Aset</h1>
        <p class="text-sm text-gray-500 mt-0.5">Pelepasan aset — dijual, dimusnahkan, dihibahkan, atau hilang — beserta laba/ruginya.</p>
      </div>
      <AppButton v-if="canManage" @click="openCreate">
        <span class="btn-ic" v-html="ICONS.trash"></span>Hapus Aset
      </AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Ringkasan — flat, konsisten dengan halaman aset lain -->
    <div class="kpi-grid">
      <div class="kpi">
        <div class="kpi-label">Aset Dilepas</div>
        <div class="kpi-val">{{ rows.length }}</div>
        <div class="kpi-sub">{{ totalQty }} unit pada rentang ini</div>
      </div>
      <div class="kpi">
        <div class="kpi-label">Nilai Buku Dilepas</div>
        <div class="kpi-val">{{ formatRupiah(totals.book) }}</div>
        <div class="kpi-sub">nilai tercatat saat dihapus</div>
      </div>
      <div class="kpi">
        <div class="kpi-label">Hasil Penjualan</div>
        <div class="kpi-val">{{ formatRupiah(totals.proceeds) }}</div>
        <div class="kpi-sub">uang masuk dari pelepasan</div>
      </div>
      <div class="kpi" :class="totals.gainLoss < 0 ? 'kpi--alert' : 'kpi--good'">
        <div class="kpi-label">{{ totals.gainLoss < 0 ? 'Rugi Pelepasan' : 'Laba Pelepasan' }}</div>
        <div class="kpi-val">{{ formatRupiah(Math.abs(totals.gainLoss)) }}</div>
        <div class="kpi-sub">hasil − nilai buku</div>
      </div>
    </div>

    <!-- Filters -->
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
      <div v-if="hasActiveFilters" class="filter-summary">
        <span>{{ rows.length }} penghapusan ditemukan</span>
        <button type="button" class="lnk" @click="resetFilters">Bersihkan filter</button>
      </div>
    </AppCard>

    <!-- List -->
    <AppCard :padding="false">
      <div class="sm:hidden">
        <div v-if="loading" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
        <div v-else-if="!rows.length" class="empty-block">
          <span class="empty-ic" v-html="ICONS.trash"></span>
          <p class="empty-title">{{ hasActiveFilters ? 'Tidak ada penghapusan yang cocok' : 'Belum ada penghapusan aset' }}</p>
          <p class="empty-desc">{{ hasActiveFilters ? 'Coba ubah outlet, cara, atau rentang tanggal.' : 'Aset yang dijual, dimusnahkan, dihibahkan, atau hilang akan tercatat di sini.' }}</p>
        </div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="x in rows" :key="x.id" class="p-4 space-y-1">
            <div class="flex items-start justify-between gap-2">
              <p class="font-semibold text-gray-900 break-words">{{ x.asset_name }}</p>
              <span class="method-badge shrink-0" :class="`method-badge--${x.method}`">
                <span class="method-ic" v-html="methodIcon(x.method)"></span>{{ METHODS[x.method] || x.method }}
              </span>
            </div>
            <p class="text-xs text-gray-500">{{ formatDateStr(x.disposal_date) }} · {{ x.outlet_name }} · {{ x.quantity }} unit</p>
            <p class="text-sm text-gray-800">
              Hasil {{ formatRupiah(x.proceeds) }} − buku {{ formatRupiah(x.book_value_at_disposal) }} =
              <strong :class="x.gain_loss < 0 ? 'txt-loss' : 'txt-gain'">{{ formatRupiah(x.gain_loss) }}</strong>
            </p>
            <p v-if="x.reason" class="text-xs text-gray-500">{{ x.reason }}</p>
            <button v-if="canManage" @click="confirmRestore(x)" class="pill-btn pill-btn--restore">
              <span class="btn-ic" v-html="ICONS.undo"></span>Batalkan penghapusan
            </button>
          </li>
        </ul>
      </div>

      <!-- v-if di sini, bukan hanya emptyText — AppTable selalu merender baris
           "Tidak ada data" bawaannya sendiri saat rows kosong; tanpa v-if,
           blok kosong kustom di bawah akan tampil dobel dengan baris bawaan itu. -->
      <AppTable v-if="loading || rows.length" class="hidden sm:block" :columns="COLUMNS" :rows="rows" :loading="loading">
        <template #cell-date="{ row }">{{ formatDateStr(row.disposal_date) }}</template>
        <template #cell-asset="{ row }">
          <div>
            <p class="font-medium text-gray-900">{{ row.asset_name }}</p>
            <p v-if="row.asset_code" class="text-xs text-gray-400 font-mono">{{ row.asset_code }}</p>
          </div>
        </template>
        <template #cell-method="{ row }">
          <span class="method-badge" :class="`method-badge--${row.method}`">
            <span class="method-ic" v-html="methodIcon(row.method)"></span>{{ METHODS[row.method] || row.method }}
          </span>
        </template>
        <template #cell-book="{ row }">{{ formatRupiah(row.book_value_at_disposal) }}</template>
        <template #cell-proceeds="{ row }">{{ formatRupiah(row.proceeds) }}</template>
        <template #cell-gainloss="{ row }">
          <strong :class="row.gain_loss < 0 ? 'txt-loss' : 'txt-gain'">{{ formatRupiah(row.gain_loss) }}</strong>
        </template>
        <template #cell-reason="{ row }">
          <span>{{ row.reason || '—' }}</span>
          <span v-if="row.approved_by" class="text-xs text-gray-400 block">disetujui {{ row.approved_by }}</span>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex justify-end">
            <button v-if="canManage" class="icon-btn icon-btn--restore" title="Batalkan penghapusan" aria-label="Batalkan penghapusan" @click="confirmRestore(row)">
              <span v-html="ICONS.undo"></span>
            </button>
          </div>
        </template>
      </AppTable>

      <div v-if="!loading && !rows.length" class="hidden sm:block empty-block">
        <span class="empty-ic" v-html="ICONS.trash"></span>
        <p class="empty-title">{{ hasActiveFilters ? 'Tidak ada penghapusan yang cocok dengan filter' : 'Belum ada penghapusan aset' }}</p>
        <p class="empty-desc">{{ hasActiveFilters ? 'Coba ubah outlet, cara, atau rentang tanggal.' : 'Aset yang dijual, dimusnahkan, dihibahkan, atau hilang akan tercatat di sini beserta laba/ruginya.' }}</p>
        <button v-if="hasActiveFilters" type="button" class="lnk mt-1" @click="resetFilters">Bersihkan filter</button>
      </div>
    </AppCard>

    <!-- ══════════════════ Modal: Hapus Aset ══════════════════ -->
    <!-- Dua kolom berdampingan supaya modal ini tetap muat satu layar tanpa
         perlu menggulir. Di layar sempit (<md) kembali bertumpuk. -->
    <AppModal v-model="modal" title="Hapus Aset" size="2xl">
      <form class="grid md:grid-cols-2 gap-x-8 gap-y-5" @submit.prevent="save">
        <div class="space-y-5">
          <!-- Aset & Cara -->
          <section class="f-section">
            <div class="f-section-hd">
              <span class="f-section-ic f-section-ic--indigo" v-html="ICONS.tag"></span>
              <div>
                <h3 class="f-section-title">Aset &amp; Cara Pelepasan</h3>
                <p class="f-section-desc">Aset mana yang dilepas, dan bagaimana caranya.</p>
              </div>
            </div>
            <div class="space-y-3">
              <div>
                <label class="lbl">Aset <span class="req">*</span></label>
                <SearchSelect v-model="form.asset_id" :options="assetOptions" placeholder="Pilih aset…" searchPlaceholder="Cari aset…" />
                <p v-if="selectedAsset" class="hint-text">
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
            </div>
          </section>
        </div>

        <div class="space-y-5">
          <!-- Hasil & Persetujuan -->
          <section class="f-section">
            <div class="f-section-hd">
              <span class="f-section-ic f-section-ic--emerald" v-html="ICONS.banknote"></span>
              <div>
                <h3 class="f-section-title">Hasil &amp; Persetujuan</h3>
                <p class="f-section-desc">Uang masuk (bila dijual) dan siapa yang menyetujui.</p>
              </div>
            </div>
            <div class="space-y-3">
              <div class="grid grid-cols-2 gap-3">
                <div>
                  <label class="lbl">Hasil Penjualan</label>
                  <RupiahInput v-model="form.proceeds" placeholder="0" :disabled="form.method !== 'dijual'" />
                </div>
                <div>
                  <label class="lbl">Disetujui Oleh</label>
                  <input v-model="form.approved_by" class="form-input" placeholder="Nama penyetuju" />
                </div>
              </div>

              <div v-if="selectedAsset" class="preview-box" :class="previewGainLoss < 0 ? 'preview-box--loss' : 'preview-box--gain'">
                <span class="preview-ic" v-html="previewGainLoss < 0 ? ICONS.trendDown : ICONS.trendUp"></span>
                <div class="text-sm">
                  <p :class="previewGainLoss < 0 ? 'txt-loss' : 'txt-gain'">
                    {{ previewGainLoss < 0 ? 'Rugi' : 'Laba' }} pelepasan: <strong>{{ formatRupiah(Math.abs(previewGainLoss)) }}</strong>
                  </p>
                  <p class="text-xs text-gray-500 mt-0.5">{{ formatRupiah(form.proceeds || 0) }} − {{ formatRupiah(selectedAsset.book_value) }}</p>
                </div>
              </div>

              <div>
                <label class="lbl">Alasan <span class="req">*</span></label>
                <textarea v-model="form.reason" rows="2" class="form-input" required placeholder="mis. Rusak berat tidak ekonomis diperbaiki"></textarea>
              </div>
              <p class="hint-text">
                <span class="btn-ic btn-ic--inline" v-html="ICONS.info"></span>
                Aset akan keluar dari daftar aktif namun riwayatnya tetap tersimpan, dan penghapusan ini bisa dibatalkan.
              </p>
            </div>
          </section>
        </div>
      </form>

      <template #footer>
        <button type="button" class="btn-ghost" @click="modal = false">Batal</button>
        <AppButton :loading="saving" @click="save">Hapus Aset</AppButton>
      </template>
    </AppModal>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import Swal from 'sweetalert2'
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
import RupiahInput from '@/components/ui/RupiahInput.vue'

const toast = useToastStore()
const auth = useAuthStore()
const canManage = auth.hasPermission('assets.disposal.manage')

const METHODS = { dijual: 'Dijual', dimusnahkan: 'Dimusnahkan', hibah: 'Dihibahkan', hilang: 'Hilang' }

// ── Ikon — bahasa visual yang sama dengan halaman aset lain.
const ICONS = {
  trash: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>',
  undo: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M3 10h10a5 5 0 010 10h-2"/><path stroke-linecap="round" stroke-linejoin="round" d="M7 5L3 10l4 5"/></svg>',
  tag: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><path stroke-linecap="round" stroke-linejoin="round" d="M11.5 3H6a2 2 0 00-2 2v5.5a2 2 0 00.586 1.414l8.5 8.5a2 2 0 002.828 0l5.5-5.5a2 2 0 000-2.828l-8.5-8.5A2 2 0 0011.5 3z"/><circle cx="7.5" cy="7.5" r="1.1" fill="currentColor" stroke="none"/></svg>',
  banknote: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><rect x="2.5" y="6" width="19" height="12" rx="2"/><circle cx="12" cy="12" r="2.3"/><path stroke-linecap="round" d="M5.5 9.5h.01M18.5 14.5h.01"/></svg>',
  info: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="9"/><path stroke-linecap="round" d="M12 11v5"/><path stroke-linecap="round" d="M12 8h.01"/></svg>',
  trendDown: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M3 6l7 7 4-4 7 7"/><path stroke-linecap="round" stroke-linejoin="round" d="M15 16h6v-6"/></svg>',
  trendUp: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M3 18l7-7 4 4 7-7"/><path stroke-linecap="round" stroke-linejoin="round" d="M15 8h6v6"/></svg>',
  // Ikon per cara pelepasan — memberi isyarat visual sekilas pada badge.
  sold: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="2.5" y="6" width="19" height="12" rx="2"/><circle cx="12" cy="12" r="2.3"/></svg>',
  flame: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M12 2c1 3-2 4-2 7a4 4 0 008 0c0-1-.5-2-1-2 .5 2-1 3-1 3 .3-2-1-3-1-4-1 1-3 2-3 4a4 4 0 01-3-4c0-2 1.5-3 3-4z"/></svg>',
  gift: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><rect x="3" y="8" width="18" height="13" rx="1"/><path stroke-linecap="round" d="M3 12h18M12 8v13"/><path stroke-linecap="round" stroke-linejoin="round" d="M12 8c-1.5 0-3-1-3-2.5S10 3 11 3s1 2 1 5zm0 0c1.5 0 3-1 3-2.5S13 3 12 3s-1 2 0 5z"/></svg>',
  question: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="12" r="9"/><path stroke-linecap="round" stroke-linejoin="round" d="M9.5 9a2.5 2.5 0 015 .5c0 1.5-2.5 2-2.5 3.5"/><path stroke-linecap="round" d="M12 17h.01"/></svg>',
}
const METHOD_ICONS = { dijual: ICONS.sold, dimusnahkan: ICONS.flame, hibah: ICONS.gift, hilang: ICONS.question }
function methodIcon(m) { return METHOD_ICONS[m] || ICONS.question }

// Dialog konfirmasi bermerek — konsisten dengan halaman aset lain, bukan
// confirm() bawaan browser.
function swalBase(options = {}) {
  return Swal.fire({
    background: '#ffffff',
    color: '#0f172a',
    confirmButtonColor: '#0f766e',
    cancelButtonColor: '#64748b',
    reverseButtons: true,
    buttonsStyling: true,
    ...options,
  })
}

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
const hasActiveFilters = computed(() => !!(filterOutlet.value || filterMethod.value || from.value || to.value))
const totals = computed(() => rows.value.reduce((t, x) => ({
  book: t.book + Number(x.book_value_at_disposal || 0),
  proceeds: t.proceeds + Number(x.proceeds || 0),
  gainLoss: t.gainLoss + Number(x.gain_loss || 0),
}), { book: 0, proceeds: 0, gainLoss: 0 }))

function resetFilters() {
  filterOutlet.value = ''; filterMethod.value = ''; from.value = ''; to.value = ''
  load()
}

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
  const r = await swalBase({
    icon: 'question',
    title: `Batalkan penghapusan "${x.asset_name}"?`,
    html: `Aset akan <strong>kembali aktif</strong> di daftar, dan catatan pelepasan ini (${formatRupiah(x.gain_loss)}) akan dihapus.`,
    showCancelButton: true,
    confirmButtonText: 'Ya, batalkan penghapusan',
    cancelButtonText: 'Tutup',
  })
  if (!r.isConfirmed) return
  try {
    await assetsApi.restoreDisposal(x.id)
    toast.success('Penghapusan dibatalkan')
    await Promise.all([load(), loadRefs()])
  } catch (e) {
    swalBase({ icon: 'error', title: 'Gagal membatalkan', text: e?.message || 'Terjadi kendala saat membatalkan penghapusan.' })
  }
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
.req { color: #ef4444; }
.btn-ghost { padding: .5rem 1rem; border-radius: .6rem; font-size: .85rem; font-weight: 600; color: #374151; background: #f3f4f6; }
.btn-ghost:hover { background: #e5e7eb; }
.lnk { color: #047857; font-weight: 600; text-decoration: none; background: none; border: none; cursor: pointer; font-size: inherit; padding: 0; }
.lnk:hover { text-decoration: underline; }

.btn-ic { display: inline-flex; margin-right: .35rem; vertical-align: -2px; }
.btn-ic :deep(svg) { width: .85rem; height: .85rem; }
.btn-ic--inline { margin-right: .25rem; vertical-align: -3px; }
.btn-ic--inline :deep(svg) { width: .8rem; height: .8rem; }

.txt-loss { color: #be123c; }
.txt-gain { color: #047857; }

/* ── Kartu ringkasan — flat ── */
.kpi-grid { display: grid; gap: .75rem; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); }
.kpi { border-radius: .85rem; padding: .85rem 1rem; background: #fff; border: 1px solid rgba(17,24,39,.08); box-shadow: 0 1px 2px rgba(16,24,40,.04); }
.kpi-label { font-size: .68rem; font-weight: 700; color: #6b7280; text-transform: uppercase; letter-spacing: .03em; }
.kpi-val { font-size: 1.3rem; font-weight: 800; color: #111827; line-height: 1.2; margin-top: .2rem; }
.kpi-sub { font-size: .71rem; color: #6b7280; margin-top: .15rem; }
.kpi--good .kpi-val { color: #047857; }
.kpi--alert { border-color: rgba(225,29,72,.28); background: rgba(225,29,72,.035); }
.kpi--alert .kpi-val { color: #be123c; }

.filter-summary { display: flex; align-items: center; justify-content: space-between; gap: .5rem; margin-top: .7rem; padding-top: .7rem; border-top: 1px solid rgba(17,24,39,.06); font-size: .75rem; color: #6b7280; }

.method-badge { display: inline-flex; align-items: center; gap: .3rem; padding: .1rem .5rem; border-radius: 999px; font-size: .68rem; font-weight: 700; white-space: nowrap; }
.method-ic { display: inline-flex; }
.method-ic :deep(svg) { width: .7rem; height: .7rem; }
.method-badge--dijual { background: rgba(16,185,129,.13); color: #047857; }
.method-badge--dimusnahkan { background: rgba(225,29,72,.12); color: #be123c; }
.method-badge--hibah { background: rgba(14,165,233,.13); color: #0369a1; }
.method-badge--hilang { background: rgba(107,114,128,.14); color: #4b5563; }

/* ── Tombol aksi ikon (tabel desktop) ── */
.icon-btn {
  display: inline-flex; align-items: center; justify-content: center;
  width: 2rem; height: 2rem; border-radius: .6rem; color: #6b7280;
  background: transparent; transition: background .12s ease, color .12s ease;
}
.icon-btn :deep(svg) { width: 1.05rem; height: 1.05rem; }
.icon-btn--restore:hover { background: rgba(16,185,129,.1); color: #047857; }

/* ── Tombol aksi pil (kartu mobile) ── */
.pill-btn {
  display: inline-flex; align-items: center; justify-content: center;
  padding: .4rem .6rem; border-radius: .6rem; font-size: .72rem; font-weight: 600;
}
.pill-btn--restore { background: rgba(16,185,129,.1); color: #047857; }
.pill-btn--restore:hover { background: rgba(16,185,129,.16); }

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
.f-section-ic--emerald { background: rgba(16,185,129,.1); color: #047857; }
.f-section-title { font-size: .88rem; font-weight: 700; color: #111827; line-height: 1.3; }
.f-section-desc { font-size: .74rem; color: #9ca3af; margin-top: .05rem; }

/* Bukan display:flex — .btn-ic--inline sudah inline-flex sendiri; membuat
   .hint-text jadi flex container malah menghilangkan spasi antara teks polos
   dan elemen (mis. sebelum <strong>) karena anonymous flex item boundary. */
.hint-text { font-size: .72rem; color: #9ca3af; margin-top: .4rem; line-height: 1.5; }
.hint-text strong { color: #374151; }

.preview-box {
  display: flex; align-items: flex-start; gap: .6rem;
  padding: .7rem .8rem; border-radius: .7rem; border: 1px solid transparent;
}
.preview-box--gain { background: rgba(16,185,129,.06); border-color: rgba(16,185,129,.18); }
.preview-box--loss { background: rgba(225,29,72,.06); border-color: rgba(225,29,72,.18); }
.preview-ic { flex-shrink: 0; margin-top: .1rem; }
.preview-ic :deep(svg) { width: 1.1rem; height: 1.1rem; }
.preview-box--gain .preview-ic { color: #047857; }
.preview-box--loss .preview-ic { color: #be123c; }
</style>
