<template>
  <div class="space-y-5">
    <div class="flex items-start justify-between flex-wrap gap-3">
      <div>
        <h1 class="text-xl font-bold text-gray-900">Histori Perolehan</h1>
        <p class="text-sm text-gray-500 mt-0.5">Asal-usul setiap aset: pembelian, hibah, sewa, atau produksi sendiri.</p>
      </div>
      <AppButton v-if="canManage" @click="openCreate">
        <span class="btn-ic" v-html="ICONS.plus"></span>Catat Perolehan
      </AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Ringkasan — flat, tanpa gradien, konsisten dengan Daftar Aset -->
    <div class="kpi-grid">
      <div class="kpi">
        <div class="kpi-label">Total Nilai Perolehan</div>
        <div class="kpi-val">{{ formatRupiah(totalCost) }}</div>
        <div class="kpi-sub">{{ rows.length }} catatan · {{ totalQty }} unit</div>
      </div>
      <div v-for="s in sourceTotals" :key="s.key" class="kpi">
        <div class="kpi-label">
          <span class="kpi-dot" :class="`kpi-dot--${s.key}`"></span>{{ SOURCES[s.key] || s.key }}
        </div>
        <div class="kpi-val">{{ formatRupiah(s.value) }}</div>
        <div class="kpi-sub">{{ s.count }} catatan · {{ s.qty }} unit</div>
      </div>
    </div>

    <!-- Filters -->
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
      <div v-if="hasActiveFilters" class="filter-summary">
        <span>{{ rows.length }} catatan ditemukan</span>
        <button type="button" class="lnk" @click="resetFilters">Bersihkan filter</button>
      </div>
    </AppCard>

    <!-- List -->
    <AppCard :padding="false">
      <!-- Mobile -->
      <div class="sm:hidden">
        <div v-if="loading" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
        <div v-else-if="!rows.length" class="empty-block">
          <span class="empty-ic" v-html="ICONS.receipt"></span>
          <p class="empty-title">{{ hasActiveFilters ? 'Tidak ada catatan yang cocok' : 'Belum ada catatan perolehan' }}</p>
          <p class="empty-desc">{{ hasActiveFilters ? 'Coba ubah atau bersihkan filter pencarian.' : 'Perolehan pertama otomatis tercatat saat menambah aset baru.' }}</p>
        </div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="x in rows" :key="x.id" class="p-4 space-y-1.5">
            <div class="flex items-start justify-between gap-2">
              <div class="min-w-0">
                <p class="font-semibold text-gray-900 break-words">{{ x.asset_name }}</p>
                <p class="text-xs text-gray-500">{{ formatDateStr(x.acquisition_date) }} · {{ x.outlet_name }}</p>
              </div>
              <span class="src-badge shrink-0" :class="`src-badge--${x.source}`">{{ SOURCES[x.source] || x.source }}</span>
            </div>
            <p class="text-sm text-gray-800">{{ x.quantity }} × {{ formatRupiah(x.unit_price) }} = <strong>{{ formatRupiah(x.total_cost) }}</strong></p>
            <p v-if="x.vendor_name || x.document_no" class="text-xs text-gray-500">
              <span v-if="x.vendor_name">{{ x.vendor_name }}</span><span v-if="x.document_no"> · {{ x.document_no }}</span>
            </p>
            <button v-if="canManage" @click="confirmDelete(x)" class="pill-btn pill-btn--delete">
              <span class="btn-ic" v-html="ICONS.trash"></span>Hapus catatan
            </button>
          </li>
        </ul>
      </div>

      <!-- v-if di sini, bukan hanya emptyText — AppTable selalu merender baris
           "Tidak ada data" bawaannya sendiri saat rows kosong; tanpa v-if,
           blok kosong kustom di bawah akan tampil dobel dengan baris bawaan itu. -->
      <AppTable v-if="loading || rows.length" class="hidden sm:block" :columns="COLUMNS" :rows="rows" :loading="loading">
        <template #cell-date="{ row }">{{ formatDateStr(row.acquisition_date) }}</template>
        <template #cell-asset="{ row }">
          <div>
            <p class="font-medium text-gray-900">{{ row.asset_name }}</p>
            <p v-if="row.asset_code" class="text-xs text-gray-400 font-mono">{{ row.asset_code }}</p>
          </div>
        </template>
        <template #cell-source="{ row }"><span class="src-badge" :class="`src-badge--${row.source}`">{{ SOURCES[row.source] || row.source }}</span></template>
        <template #cell-qty="{ row }">{{ row.quantity }} × {{ formatRupiah(row.unit_price) }}</template>
        <template #cell-total="{ row }"><strong>{{ formatRupiah(row.total_cost) }}</strong></template>
        <template #cell-vendor="{ row }">
          <span>{{ row.vendor_name || '—' }}</span>
          <span v-if="row.document_no" class="text-xs text-gray-400 block font-mono">{{ row.document_no }}</span>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex justify-end">
            <button v-if="canManage" class="icon-btn icon-btn--delete" title="Hapus catatan perolehan" aria-label="Hapus catatan" @click="confirmDelete(row)">
              <span v-html="ICONS.trash"></span>
            </button>
          </div>
        </template>
      </AppTable>

      <div v-if="!loading && !rows.length" class="hidden sm:block empty-block">
        <span class="empty-ic" v-html="ICONS.receipt"></span>
        <p class="empty-title">{{ hasActiveFilters ? 'Tidak ada catatan yang cocok dengan filter' : 'Belum ada catatan perolehan' }}</p>
        <p class="empty-desc">{{ hasActiveFilters ? 'Coba ubah outlet, sumber, rentang tanggal, atau kata kunci.' : 'Perolehan pertama tercatat otomatis saat menambah aset baru dari Daftar Aset, atau catat langsung dari sini.' }}</p>
        <button v-if="hasActiveFilters" type="button" class="lnk mt-1" @click="resetFilters">Bersihkan filter</button>
        <AppButton v-else-if="canManage" class="mt-1" size="sm" @click="openCreate">
          <span class="btn-ic" v-html="ICONS.plus"></span>Catat perolehan pertama
        </AppButton>
      </div>
    </AppCard>

    <!-- ══════════════════ Modal: Catat Perolehan ══════════════════ -->
    <!-- Dua kolom berdampingan supaya modal besar ini tetap muat satu layar
         tanpa perlu menggulir. Di layar sempit (<md) kembali bertumpuk. -->
    <AppModal v-model="modal" title="Catat Perolehan" size="2xl">
      <form class="grid md:grid-cols-2 gap-x-8 gap-y-5" @submit.prevent="save">
        <div class="space-y-5">
          <!-- Aset & Sumber -->
          <section class="f-section">
            <div class="f-section-hd">
              <span class="f-section-ic f-section-ic--indigo" v-html="ICONS.tag"></span>
              <div>
                <h3 class="f-section-title">Aset &amp; Sumber</h3>
                <p class="f-section-desc">Aset mana yang bertambah, dan dari mana asalnya.</p>
              </div>
            </div>
            <div class="space-y-3">
              <div>
                <label class="lbl">Aset <span class="req">*</span></label>
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
            </div>
          </section>

          <!-- Vendor & Dokumen -->
          <section class="f-section">
            <div class="f-section-hd">
              <span class="f-section-ic f-section-ic--sky" v-html="ICONS.truck"></span>
              <div>
                <h3 class="f-section-title">Vendor &amp; Dokumen</h3>
                <p class="f-section-desc">Opsional — memudahkan penelusuran di kemudian hari.</p>
              </div>
            </div>
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
          </section>
        </div>

        <div class="space-y-5">
          <!-- Jumlah & Harga -->
          <section class="f-section">
            <div class="f-section-hd">
              <span class="f-section-ic f-section-ic--emerald" v-html="ICONS.banknote"></span>
              <div>
                <h3 class="f-section-title">Jumlah &amp; Harga</h3>
                <p class="f-section-desc">Menentukan total biaya perolehan yang tercatat.</p>
              </div>
            </div>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="lbl">Jumlah Unit</label>
                <input v-model.number="form.quantity" type="number" min="1" class="form-input" />
              </div>
              <div>
                <label class="lbl">Harga Satuan</label>
                <RupiahInput v-model="form.unit_price" placeholder="0" />
              </div>
            </div>

            <div class="preview-box">
              <span class="preview-ic" v-html="ICONS.banknote"></span>
              <div class="text-sm">
                <p class="text-gray-800">
                  Total perolehan <strong>{{ formatRupiah((form.quantity || 0) * (form.unit_price || 0)) }}</strong>
                </p>
                <p class="text-xs text-gray-500 mt-0.5">{{ form.quantity || 0 }} unit × {{ formatRupiah(form.unit_price || 0) }} per unit</p>
              </div>
            </div>

            <label class="checkbox-box">
              <input v-model="form.add_to_quantity" type="checkbox" class="mt-0.5" />
              <span>
                Tambahkan {{ form.quantity || 0 }} unit ke jumlah aset
                <span class="block text-xs text-gray-500 mt-0.5">Aktifkan bila ini unit baru yang benar-benar masuk. Matikan bila hanya merapikan catatan perolehan lama yang unitnya sudah terhitung.</span>
              </span>
            </label>
          </section>

          <!-- Catatan -->
          <section class="f-section">
            <div class="f-section-hd">
              <span class="f-section-ic f-section-ic--slate" v-html="ICONS.doc"></span>
              <div>
                <h3 class="f-section-title">Catatan</h3>
                <p class="f-section-desc">Keterangan tambahan, opsional.</p>
              </div>
            </div>
            <textarea v-model="form.notes" rows="2" class="form-input" placeholder="Opsional"></textarea>
          </section>
        </div>
      </form>

      <template #footer>
        <button type="button" class="btn-ghost" @click="modal = false">Batal</button>
        <AppButton :loading="saving" @click="save">Simpan Perolehan</AppButton>
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
const canManage = auth.hasPermission('assets.acquisition.manage')

const SOURCES = { pembelian: 'Pembelian', hibah: 'Hibah', sewa: 'Sewa', produksi_sendiri: 'Produksi Sendiri' }

// ── Ikon — sama seperti Daftar Aset (bahasa visual sidebar), plus dua ikon
// baru khusus halaman ini. Ukuran dikendalikan CSS lewat :deep(svg) karena
// v-html tidak ikut ter-scope oleh Vue.
const ICONS = {
  plus: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4"/></svg>',
  trash: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>',
  tag: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><path stroke-linecap="round" stroke-linejoin="round" d="M11.5 3H6a2 2 0 00-2 2v5.5a2 2 0 00.586 1.414l8.5 8.5a2 2 0 002.828 0l5.5-5.5a2 2 0 000-2.828l-8.5-8.5A2 2 0 0011.5 3z"/><circle cx="7.5" cy="7.5" r="1.1" fill="currentColor" stroke="none"/></svg>',
  banknote: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><rect x="2.5" y="6" width="19" height="12" rx="2"/><circle cx="12" cy="12" r="2.3"/><path stroke-linecap="round" d="M5.5 9.5h.01M18.5 14.5h.01"/></svg>',
  doc: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><path stroke-linecap="round" stroke-linejoin="round" d="M9 2h6a1 1 0 011 1v1h2a2 2 0 012 2v13a2 2 0 01-2 2H6a2 2 0 01-2-2V6a2 2 0 012-2h2V3a1 1 0 011-1z"/><line x1="8" y1="11" x2="16" y2="11"/><line x1="8" y1="15" x2="13" y2="15"/></svg>',
  // Truk — merepresentasikan vendor/pemasok yang mengirimkan barang.
  truck: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><path stroke-linecap="round" stroke-linejoin="round" d="M3 7h11v9H3z"/><path stroke-linecap="round" stroke-linejoin="round" d="M14 10h4l3 3v3h-7z"/><circle cx="7" cy="18" r="1.6"/><circle cx="17.5" cy="18" r="1.6"/></svg>',
  // Struk/nota — merepresentasikan catatan perolehan pada linimasa & keadaan kosong.
  receipt: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M6 3h12v18l-2.5-1.5L13 21l-1.5-1.5L10 21l-2.5-1.5L6 21V3z"/><path stroke-linecap="round" d="M9 8h6M9 12h6M9 16h3"/></svg>',
}

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
const hasActiveFilters = computed(() =>
  !!(filterOutlet.value || filterSource.value || from.value || to.value || search.value.trim()))
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

function resetFilters() {
  filterOutlet.value = ''; filterSource.value = ''; from.value = ''; to.value = ''; search.value = ''
  load()
}

// Dialog konfirmasi bermerek — konsisten dengan Daftar Aset & Vendors.vue,
// bukan confirm() bawaan browser.
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
  const r = await swalBase({
    icon: 'warning',
    title: 'Hapus catatan perolehan ini?',
    html: `Catatan <strong>${x.asset_name}</strong> tanggal ${formatDateStr(x.acquisition_date)}` +
          ` (${formatRupiah(x.total_cost)}) akan dihapus permanen.<br>` +
          `Jumlah unit pada aset <strong>tidak ikut berkurang</strong> — sesuaikan manual bila perlu.`,
    showCancelButton: true,
    confirmButtonColor: '#dc2626',
    confirmButtonText: 'Ya, hapus catatan',
    cancelButtonText: 'Batal',
  })
  if (!r.isConfirmed) return
  try {
    await assetsApi.removeAcquisition(x.id)
    await load()
    swalBase({ icon: 'success', title: 'Catatan dihapus', showConfirmButton: false, timer: 1600 })
  } catch (e) {
    swalBase({ icon: 'error', title: 'Gagal menghapus catatan', text: e?.message || 'Terjadi kendala saat menghapus data.' })
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
.kpi-label { display: flex; align-items: center; gap: .4rem; font-size: .68rem; font-weight: 700; color: #6b7280; text-transform: uppercase; letter-spacing: .03em; }
.kpi-val { font-size: 1.3rem; font-weight: 800; color: #111827; line-height: 1.2; margin-top: .2rem; }
.kpi-sub { font-size: .71rem; color: #6b7280; margin-top: .15rem; }
.kpi-dot { width: .5rem; height: .5rem; border-radius: 999px; flex-shrink: 0; }
.kpi-dot--pembelian { background: #6366f1; }
.kpi-dot--hibah { background: #10b981; }
.kpi-dot--sewa { background: #0ea5e9; }
.kpi-dot--produksi_sendiri { background: #f59e0b; }

.filter-summary { display: flex; align-items: center; justify-content: space-between; gap: .5rem; margin-top: .7rem; padding-top: .7rem; border-top: 1px solid rgba(17,24,39,.06); font-size: .75rem; color: #6b7280; }

.src-badge { display: inline-block; padding: .12rem .5rem; border-radius: 999px; font-size: .68rem; font-weight: 700; white-space: nowrap; }
.src-badge--pembelian { background: rgba(99,102,241,.12); color: #4338ca; }
.src-badge--hibah { background: rgba(16,185,129,.13); color: #047857; }
.src-badge--sewa { background: rgba(14,165,233,.13); color: #0369a1; }
.src-badge--produksi_sendiri { background: rgba(245,158,11,.15); color: #b45309; }

/* ── Tombol aksi ikon (tabel desktop) ── */
.icon-btn {
  display: inline-flex; align-items: center; justify-content: center;
  width: 2rem; height: 2rem; border-radius: .6rem; color: #6b7280;
  background: transparent; transition: background .12s ease, color .12s ease;
}
.icon-btn :deep(svg) { width: 1.05rem; height: 1.05rem; }
.icon-btn--delete:hover { background: rgba(225,29,72,.1); color: #be123c; }

/* ── Tombol aksi pil (kartu mobile) ── */
.pill-btn {
  display: inline-flex; align-items: center; justify-content: center;
  padding: .4rem .6rem; border-radius: .6rem; font-size: .72rem; font-weight: 600;
}
.pill-btn--delete { background: rgba(225,29,72,.1); color: #be123c; }
.pill-btn--delete:hover { background: rgba(225,29,72,.16); }

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
.f-section-ic--emerald { background: rgba(16,185,129,.1); color: #047857; }
.f-section-ic--slate { background: rgba(100,116,139,.12); color: #475569; }
.f-section-title { font-size: .88rem; font-weight: 700; color: #111827; line-height: 1.3; }
.f-section-desc { font-size: .74rem; color: #9ca3af; margin-top: .05rem; }

.preview-box {
  display: flex; align-items: flex-start; gap: .6rem; margin-top: .85rem;
  padding: .7rem .8rem; border-radius: .7rem; background: rgba(16,185,129,.06); border: 1px solid rgba(16,185,129,.18);
}
.preview-ic { flex-shrink: 0; color: #047857; margin-top: .1rem; }
.preview-ic :deep(svg) { width: 1.1rem; height: 1.1rem; }

.checkbox-box {
  display: flex; align-items: flex-start; gap: .5rem; margin-top: .7rem;
  font-size: .82rem; color: #374151; background: #f9fafb; border-radius: .7rem; padding: .6rem .7rem;
}
</style>
