<template>
  <div class="space-y-5">
    <div class="flex items-start justify-between flex-wrap gap-3">
      <div>
        <h1 class="text-xl font-bold text-gray-900">Reservasi</h1>
        <p class="text-sm text-gray-500 mt-0.5">Kelola reservasi: tamu, menu dipesan, DP, total & sisa pembayaran.</p>
      </div>
      <AppButton v-if="canCreate" @click="openCreate">+ Reservasi Baru</AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Filters + public link -->
    <AppCard>
      <div class="grid grid-cols-1 sm:grid-cols-4 gap-3">
        <SearchSelect v-model="filterOutlet" :options="outletFilterOptions" placeholder="Semua outlet" searchPlaceholder="Cari outlet…" @change="load" />
        <select v-model="filterStatus" @change="load" class="form-input">
          <option value="">Semua status</option>
          <option v-for="(l,k) in STATUS" :key="k" :value="k">{{ l }}</option>
        </select>
        <input v-model="dateFrom" @change="load" type="date" class="form-input" />
        <input v-model="dateTo" @change="load" type="date" class="form-input" />
      </div>
      <div v-if="selectedOutletObj?.slug" class="mt-3 flex items-center gap-2 flex-wrap text-xs">
        <span class="text-gray-500">Link reservasi publik:</span>
        <input :value="publicUrl" readonly class="flex-1 min-w-[200px] font-mono text-gray-600 bg-gray-50 border border-gray-200 rounded px-2 py-1" />
        <button @click="copyLink" class="px-2 py-1 rounded font-medium" :class="copied ? 'text-emerald-600 bg-emerald-50' : 'text-gray-600 bg-gray-100'">{{ copied ? 'Tersalin!' : 'Salin' }}</button>
        <a :href="publicUrl" target="_blank" rel="noopener" class="px-2 py-1 rounded font-medium text-emerald-700 bg-emerald-50">Buka</a>
      </div>
    </AppCard>

    <!-- List -->
    <AppCard :padding="false">
      <!-- Mobile cards -->
      <div class="sm:hidden">
        <div v-if="loading" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
        <div v-else-if="!rows.length" class="p-6 text-center text-sm text-gray-400">Belum ada reservasi.</div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="r in rows" :key="r.id" class="p-4 space-y-2">
            <div class="flex items-start justify-between gap-2">
              <div class="min-w-0">
                <p class="font-semibold text-gray-900 break-words">{{ r.customer_name }}</p>
                <p class="text-xs text-gray-500">{{ r.outlet_name }} · {{ r.pax }} tamu<span v-if="r.customer_phone"> · {{ r.customer_phone }}</span></p>
              </div>
              <span class="st-badge shrink-0" :class="stCls(r.status)">{{ STATUS[r.status] || r.status }}</span>
            </div>
            <p class="text-xs text-gray-500">📅 {{ r.reservation_date ? formatDateStr(r.reservation_date) : '—' }} {{ r.reservation_time }}</p>
            <p class="text-xs text-gray-600">Total <b>{{ formatRupiah(r.total) }}</b> · DP {{ formatRupiah(r.down_payment) }} · Sisa <b class="text-amber-600">{{ formatRupiah(r.remaining) }}</b></p>
            <div class="flex gap-2 pt-1">
              <button @click="openEdit(r)" class="flex-1 text-center text-xs font-medium px-2 py-1.5 rounded-lg bg-gray-100 text-gray-700">Detail / Edit</button>
              <button v-if="canDelete" @click="confirmDelete(r)" class="text-center text-xs font-medium px-3 py-1.5 rounded-lg bg-red-50 text-red-600">Hapus</button>
            </div>
          </li>
        </ul>
      </div>

      <!-- Desktop table -->
      <AppTable class="hidden sm:block" :columns="COLUMNS" :rows="rows" :loading="loading" emptyText="Belum ada reservasi.">
        <template #cell-customer_name="{ row }">
          <div><p class="font-medium text-gray-900">{{ row.customer_name }}</p><p v-if="row.customer_phone" class="text-xs text-gray-400">{{ row.customer_phone }}</p></div>
        </template>
        <template #cell-schedule="{ row }">
          <span class="text-sm">{{ row.reservation_date ? formatDateStr(row.reservation_date) : '—' }}</span>
          <span class="text-xs text-gray-400 block">{{ row.reservation_time }}</span>
        </template>
        <template #cell-total="{ row }">{{ formatRupiah(row.total) }}</template>
        <template #cell-remaining="{ row }"><span :class="row.remaining > 0 ? 'text-amber-600 font-semibold' : 'text-gray-400'">{{ formatRupiah(row.remaining) }}</span></template>
        <template #cell-status="{ row }"><span class="st-badge" :class="stCls(row.status)">{{ STATUS[row.status] || row.status }}</span></template>
        <template #cell-source="{ row }"><span class="text-xs" :class="row.source==='public' ? 'text-emerald-600' : 'text-gray-400'">{{ row.source==='public' ? 'Publik' : 'Admin' }}</span></template>
        <template #cell-actions="{ row }">
          <div class="flex items-center gap-1 justify-end">
            <button @click="openEdit(row)" class="text-gray-600 hover:text-gray-900 text-xs font-medium px-2 py-1 rounded hover:bg-gray-100">Detail</button>
            <button v-if="canDelete" @click="confirmDelete(row)" class="text-red-600 hover:text-red-800 text-xs font-medium px-2 py-1 rounded hover:bg-red-50">Hapus</button>
          </div>
        </template>
      </AppTable>
    </AppCard>

    <!-- ── Public reservation URLs per outlet (sesuai role) ── -->
    <AppCard :padding="false">
      <div class="px-4 pt-4 pb-2">
        <h3 class="text-sm font-semibold text-gray-700">URL Reservasi per Outlet</h3>
        <p class="text-xs text-gray-500 mt-0.5">Bagikan link ini ke pelanggan untuk reservasi online (sesuai outlet yang Anda kelola).</p>
      </div>
      <div v-if="!outletsWithSlug.length" class="p-6 text-center text-sm text-gray-400">Belum ada outlet dengan link reservasi.</div>
      <ul v-else class="divide-y divide-gray-100">
        <li v-for="o in outletsWithSlug" :key="o.id" class="px-4 py-3 flex items-center gap-2 flex-wrap">
          <span class="font-medium text-gray-900 w-full sm:w-44 shrink-0 truncate">{{ o.name }}</span>
          <input :value="urlFor(o)" readonly @focus="$event.target.select()" class="flex-1 min-w-[160px] font-mono text-xs text-gray-600 bg-gray-50 border border-gray-200 rounded px-2 py-1" />
          <div class="flex gap-1.5 shrink-0">
            <button @click="copyOutlet(o)" class="px-2.5 py-1 rounded text-xs font-medium" :class="copiedId === o.id ? 'text-emerald-600 bg-emerald-50' : 'text-gray-600 bg-gray-100 hover:bg-gray-200'">{{ copiedId === o.id ? 'Tersalin!' : 'Salin' }}</button>
            <a :href="urlFor(o)" target="_blank" rel="noopener" class="px-2.5 py-1 rounded text-xs font-medium text-emerald-700 bg-emerald-50 hover:bg-emerald-100">Buka</a>
          </div>
        </li>
      </ul>
    </AppCard>

    <!-- ── Reservation Modal ── -->
    <AppModal v-model="modal" :title="editing ? 'Detail Reservasi' : 'Reservasi Baru'">
      <form class="space-y-3" @submit.prevent="save">
        <div v-if="!editing">
          <label class="lbl">Outlet <span class="text-red-500">*</span></label>
          <SearchSelect v-model="form.outlet_id" :options="outlets" placeholder="Pilih outlet…" searchPlaceholder="Cari outlet…" @change="onOutletChange" />
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="lbl">Nama Pemesan <span class="text-red-500">*</span></label>
            <input v-model="form.customer_name" class="form-input" required />
          </div>
          <div>
            <label class="lbl">No. HP</label>
            <input v-model="form.customer_phone" class="form-input" />
          </div>
        </div>
        <div class="grid grid-cols-3 gap-3">
          <div>
            <label class="lbl">Jumlah Tamu</label>
            <input v-model.number="form.pax" type="number" min="1" class="form-input" />
          </div>
          <div>
            <label class="lbl">Tanggal</label>
            <input v-model="form.reservation_date" type="date" class="form-input" />
          </div>
          <div>
            <label class="lbl">Jam</label>
            <input v-model="form.reservation_time" type="time" class="form-input" />
          </div>
        </div>

        <!-- Product picker -->
        <div>
          <label class="lbl">Menu Dipesan</label>
          <SearchSelect v-if="form.outlet_id" v-model="pickProduct" :options="productOptions" placeholder="+ Tambah produk…" searchPlaceholder="Cari produk…" @change="addProduct" />
          <p v-else class="text-xs text-gray-400">Pilih outlet dulu untuk memuat menu.</p>
          <ul v-if="form.items.length" class="mt-2 divide-y divide-gray-100 border border-gray-200 rounded-lg">
            <li v-for="(it,i) in form.items" :key="i" class="flex items-center gap-2 px-3 py-2">
              <span class="flex-1 min-w-0 text-sm truncate">{{ it.product_name }}<span class="text-xs text-gray-400 block">{{ formatRupiah(it.price) }}</span></span>
              <input v-model.number="it.qty" type="number" min="1" class="w-14 form-input !py-1 text-center" />
              <span class="w-24 text-right text-sm font-medium">{{ formatRupiah(it.price * it.qty) }}</span>
              <button type="button" @click="form.items.splice(i,1)" class="text-red-400 hover:text-red-600 text-lg leading-none">×</button>
            </li>
          </ul>
        </div>

        <!-- Money summary -->
        <div class="grid grid-cols-2 gap-3 items-end">
          <div>
            <label class="lbl">Down Payment (DP)</label>
            <input :value="dpDisplay" @input="onDpInput" type="text" inputmode="numeric" class="form-input" placeholder="Rp 0" />
          </div>
          <div>
            <label class="lbl">Status</label>
            <select v-model="form.status" class="form-input">
              <option v-for="(l,k) in STATUS" :key="k" :value="k">{{ l }}</option>
            </select>
          </div>
        </div>
        <div class="rounded-lg bg-gray-50 p-3 text-sm space-y-1">
          <div class="flex justify-between"><span class="text-gray-500">Subtotal</span><span class="font-medium">{{ formatRupiah(computedTotal) }}</span></div>
          <div class="flex justify-between"><span class="text-gray-500">DP</span><span>− {{ formatRupiah(form.down_payment || 0) }}</span></div>
          <div class="flex justify-between border-t border-gray-200 pt-1"><span class="font-semibold">Sisa Pembayaran</span><span class="font-bold text-amber-600">{{ formatRupiah(Math.max(0, computedTotal - (form.down_payment||0))) }}</span></div>
        </div>
        <div>
          <label class="lbl">Catatan</label>
          <textarea v-model="form.notes" rows="2" class="form-input" placeholder="Opsional"></textarea>
        </div>

        <div class="flex justify-end gap-2 pt-1">
          <button type="button" class="btn-ghost" @click="modal=false">Tutup</button>
          <AppButton type="submit" :loading="saving">{{ editing ? 'Simpan' : 'Buat Reservasi' }}</AppButton>
        </div>
      </form>
    </AppModal>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { reservationsApi } from '@/api/reservations.js'
import { productsApi } from '@/api/products.js'
import { outletsApi } from '@/api/outlets.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth.js'
import { formatRupiah, formatDateStr } from '@/utils/format.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppTable from '@/components/ui/AppTable.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppModal from '@/components/ui/AppModal.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'

const toast = useToastStore()
const auth = useAuthStore()
const canCreate = auth.hasPermission('reservations.create')
const canDelete = auth.hasPermission('reservations.delete')

const STATUS = { pending: 'Menunggu', confirmed: 'Dikonfirmasi', done: 'Selesai', cancelled: 'Dibatalkan' }
function stCls(s) { return { 'st-pending': s==='pending', 'st-confirmed': s==='confirmed', 'st-done': s==='done', 'st-cancelled': s==='cancelled' } }

const COLUMNS = [
  { key: 'customer_name', label: 'Pemesan' },
  { key: 'outlet_name',   label: 'Outlet' },
  { key: 'pax',           label: 'Tamu' },
  { key: 'schedule',      label: 'Jadwal' },
  { key: 'total',         label: 'Total' },
  { key: 'remaining',     label: 'Sisa' },
  { key: 'status',        label: 'Status' },
  { key: 'source',        label: 'Sumber' },
  { key: 'actions',       label: '' },
]

const rows = ref([])
const outlets = ref([])
const loading = ref(false)
const errorMsg = ref('')
const filterOutlet = ref('')
const filterStatus = ref('')
const dateFrom = ref('')
const dateTo = ref('')
const copied = ref(false)

const outletFilterOptions = computed(() => [{ id: '', name: 'Semua outlet' }, ...outlets.value])
const selectedOutletObj = computed(() => outlets.value.find(o => o.id === filterOutlet.value))
const publicUrl = computed(() => selectedOutletObj.value?.slug ? `${window.location.origin}/r/${selectedOutletObj.value.slug}` : '')
const outletsWithSlug = computed(() => outlets.value.filter(o => o.slug))
const copiedId = ref('')
function urlFor(o) { return `${window.location.origin}/r/${o.slug}` }
async function copyOutlet(o) {
  try { await navigator.clipboard.writeText(urlFor(o)); copiedId.value = o.id; setTimeout(() => { if (copiedId.value === o.id) copiedId.value = '' }, 1500) } catch {}
}

function asArray(d) { return Array.isArray(d) ? d : (d?.data || []) }

async function load() {
  loading.value = true; errorMsg.value = ''
  try {
    rows.value = asArray(await reservationsApi.list({
      outlet_id: filterOutlet.value || undefined,
      status: filterStatus.value || undefined,
      date_from: dateFrom.value || undefined,
      date_to: dateTo.value || undefined,
    }))
  } catch (e) { errorMsg.value = e?.message || 'Gagal memuat reservasi' } finally { loading.value = false }
}
async function loadOutlets() {
  try { const d = await outletsApi.myOutlets(); outlets.value = d?.outlets ?? d ?? [] } catch { outlets.value = [] }
}
async function copyLink() {
  try { await navigator.clipboard.writeText(publicUrl.value); copied.value = true; setTimeout(()=>copied.value=false, 1500) } catch {}
}

// ── Modal ──
const modal = ref(false)
const editing = ref(null)
const saving = ref(false)
const form = ref({})
const pickProduct = ref('')
const products = ref([])
const dpDisplay = ref('')

function fmtRupiahInput(v) { const d = String(v ?? '').replace(/[^\d]/g, ''); return d ? 'Rp ' + new Intl.NumberFormat('id-ID').format(Number(d)) : '' }
function onDpInput(e) {
  const d = e.target.value.replace(/[^\d]/g, '')
  const num = d ? parseInt(d, 10) : 0
  form.value.down_payment = num
  dpDisplay.value = num ? fmtRupiahInput(num) : ''
}

const productOptions = computed(() => products.value.map(p => ({ id: p.id, name: `${p.name} — ${formatRupiah(p.price)}` })))
const computedTotal = computed(() => form.value.items?.reduce((s, it) => s + it.price * (it.qty || 0), 0) || 0)

function blank() { return { outlet_id: filterOutlet.value || '', customer_name: '', customer_phone: '', pax: 1, reservation_date: '', reservation_time: '', items: [], down_payment: 0, status: 'pending', notes: '' } }

async function loadProducts(outletId) {
  products.value = []
  if (!outletId) return
  try { const res = await productsApi.listProducts({ outlet_id: outletId, limit: 500 }); products.value = res.items ?? [] } catch { products.value = [] }
}
function onOutletChange() { loadProducts(form.value.outlet_id) }
function addProduct() {
  const p = products.value.find(x => x.id === pickProduct.value)
  pickProduct.value = ''
  if (!p) return
  const ex = form.value.items.find(i => i.product_id === p.id)
  if (ex) ex.qty++
  else form.value.items.push({ product_id: p.id, product_name: p.name, price: p.price, qty: 1 })
}

async function openCreate() {
  editing.value = null; form.value = blank(); pickProduct.value = ''; dpDisplay.value = ''
  await loadProducts(form.value.outlet_id)
  modal.value = true
}
async function openEdit(r) {
  editing.value = r
  form.value = { outlet_id: r.outlet_id, customer_name: r.customer_name, customer_phone: r.customer_phone, pax: r.pax, reservation_date: r.reservation_date || '', reservation_time: r.reservation_time || '', items: (r.items||[]).map(i=>({...i})), down_payment: r.down_payment, status: r.status, notes: r.notes }
  dpDisplay.value = r.down_payment ? fmtRupiahInput(r.down_payment) : ''
  await loadProducts(r.outlet_id)
  modal.value = true
}
async function save() {
  if (!form.value.customer_name?.trim()) { toast.error('Nama pemesan wajib diisi'); return }
  if (!editing.value && !form.value.outlet_id) { toast.error('Pilih outlet'); return }
  saving.value = true
  try {
    const payload = { ...form.value, items: form.value.items.map(i => ({ product_id: i.product_id, qty: i.qty })) }
    if (editing.value) await reservationsApi.update(editing.value.id, payload)
    else await reservationsApi.create(payload)
    toast.success(editing.value ? 'Reservasi diperbarui' : 'Reservasi dibuat')
    modal.value = false
    await load()
  } catch (e) { toast.error(e?.message || 'Gagal menyimpan') } finally { saving.value = false }
}
async function confirmDelete(r) {
  if (!window.confirm(`Hapus reservasi "${r.customer_name}"?`)) return
  try { await reservationsApi.remove(r.id); toast.success('Reservasi dihapus'); await load() }
  catch (e) { toast.error(e?.message || 'Gagal menghapus') }
}

onMounted(async () => { await loadOutlets(); await load() })
</script>

<style scoped>
.form-input { width: 100%; padding: .5rem .7rem; border-radius: .6rem; font-size: .85rem; border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none; }
.form-input:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }
.lbl { display: block; font-size: .72rem; font-weight: 700; color: #4b5563; margin-bottom: .25rem; }
.btn-ghost { padding: .5rem 1rem; border-radius: .6rem; font-size: .85rem; font-weight: 600; color: #374151; background: #f3f4f6; }
.btn-ghost:hover { background: #e5e7eb; }
.st-badge { display: inline-block; padding: .12rem .5rem; border-radius: 999px; font-size: .68rem; font-weight: 700; white-space: nowrap; }
.st-pending { background: rgba(245,158,11,.15); color: #b45309; }
.st-confirmed { background: rgba(59,130,246,.13); color: #1d4ed8; }
.st-done { background: rgba(16,185,129,.14); color: #047857; }
.st-cancelled { background: rgba(239,68,68,.13); color: #b91c1c; }
</style>
