<template>
  <div class="space-y-5">
    <!-- Header -->
    <div class="flex items-start justify-between flex-wrap gap-3">
      <div>
        <h1 class="text-xl font-bold text-gray-900">Dashboard Aset</h1>
        <p class="text-sm text-gray-500 mt-0.5">Nilai, penyusutan, dan kesehatan perawatan aset.</p>
      </div>
      <div class="flex items-center gap-2">
        <SearchSelect v-model="filterOutlet" :options="outletFilterOptions" placeholder="Semua outlet" searchPlaceholder="Cari outlet…" @change="load" class="min-w-[190px]" />
        <button class="refresh-btn" @click="load" :disabled="loading">
          <svg :class="{ spin: loading }" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg>
          Refresh
        </button>
      </div>
    </div>

    <AppAlert type="error" :message="errorMsg" />
    <div v-if="loading && !d" class="py-16 text-center"><AppSpinner size="lg" class="text-emerald-600" /></div>

    <template v-if="d">
      <!-- ══ Belum ada aset: satu penjelasan utuh, bukan dinding angka nol ══ -->
      <AppCard v-if="isEmpty">
        <div class="empty-hero">
          <div class="empty-hero-ic">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.4"><path stroke-linecap="round" stroke-linejoin="round" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"/></svg>
          </div>
          <h2 class="empty-hero-title">{{ emptyTitle }}</h2>
          <p class="empty-hero-desc">{{ emptyDesc }}</p>
          <ul class="empty-hero-list">
            <li><strong>Nilai</strong> — biaya perolehan, penyusutan berjalan, dan nilai buku terkini.</li>
            <li><strong>Perawatan</strong> — jadwal berikutnya, yang terlambat, dan biayanya.</li>
            <li><strong>Siklus hidup</strong> — perolehan, mutasi antar outlet, sampai pelepasan.</li>
          </ul>
          <div class="empty-hero-act">
            <router-link class="btn-primary" to="/aset/daftar">Tambah aset pertama</router-link>
            <button v-if="filterOutlet" class="btn-quiet" @click="clearOutlet">Lihat semua outlet</button>
          </div>
        </div>
      </AppCard>

      <template v-else>
        <!-- ══ Nilai ══ -->
        <section>
          <h2 class="sec-title">Nilai Aset</h2>
          <div class="grid-3">
            <div class="card">
              <div class="card-label">Nilai Buku</div>
              <div class="card-val">{{ formatRupiah(d.book_value) }}</div>
              <template v-if="hasDepreciation">
                <div class="meter"><div class="meter-fill" :style="{ width: deprecPct + '%' }"></div></div>
                <div class="card-sub">
                  {{ deprecPct }}% tersusut ·
                  <strong>{{ d.depreciating_count }} dari {{ d.total_assets }}</strong> aset disusutkan
                </div>
              </template>
              <div v-else class="card-sub">
                Sama dengan nilai perolehan — belum ada aset yang disusutkan.
              </div>
            </div>

            <div class="card">
              <div class="card-label">Nilai Perolehan</div>
              <div class="card-val">{{ formatRupiah(d.acquisition_cost) }}</div>
              <div class="card-sub">{{ d.total_assets }} aset · {{ d.total_quantity }} unit</div>
            </div>

            <div class="card">
              <div class="card-label">Penyusutan</div>
              <template v-if="hasDepreciation">
                <div class="card-val">{{ formatRupiah(d.monthly_deprec) }}<span class="card-unit">/bulan</span></div>
                <div class="card-sub">
                  akumulasi {{ formatRupiah(d.accumulated_deprec) }}
                  <template v-if="d.depreciating_count < d.total_assets">
                    · <router-link class="lnk" to="/aset/penyusutan">{{ d.total_assets - d.depreciating_count }} aset belum diatur</router-link>
                  </template>
                </div>
              </template>
              <template v-else>
                <div class="card-val card-val--muted">Belum diatur</div>
                <div class="card-sub">
                  Isi <strong>umur ekonomis</strong> aset agar nilai buku menyusut otomatis.
                  <router-link class="lnk" to="/aset/daftar">Atur di Daftar Aset</router-link>
                </div>
              </template>
            </div>
          </div>
        </section>

        <!-- ══ Perawatan & siklus hidup ══ -->
        <section>
          <h2 class="sec-title">Perawatan &amp; Siklus Hidup</h2>
          <div class="grid-4">
            <component :is="d.overdue > 0 ? 'router-link' : 'div'" to="/aset/daftar?due=overdue"
                       class="card" :class="d.overdue > 0 ? 'card--alert card--link' : 'card--good'">
              <div class="card-label">Perawatan Terlambat</div>
              <div class="card-val">{{ d.overdue }}</div>
              <div class="card-sub">{{ d.overdue > 0 ? 'lewat jadwal — perlu ditindak' : 'tidak ada yang lewat jadwal' }}</div>
            </component>

            <component :is="d.due_soon > 0 ? 'router-link' : 'div'" to="/aset/daftar?due=due_soon"
                       class="card" :class="d.due_soon > 0 ? 'card--warn card--link' : 'card--good'">
              <div class="card-label">Jatuh Tempo ≤{{ d.due_soon_days }} Hari</div>
              <div class="card-val">{{ d.due_soon }}</div>
              <div class="card-sub">{{ d.due_soon > 0 ? 'siapkan jadwal perawatannya' : 'aman sepekan ke depan' }}</div>
            </component>

            <!-- Angka utama = 12 bulan. Memakai bulan berjalan sebagai angka
                 utama membuat kartu terbaca "tidak ada data" tiap awal bulan. -->
            <div class="card">
              <div class="card-label">Biaya Perawatan</div>
              <template v-if="d.maint_cost_12m > 0">
                <div class="card-val">{{ formatRupiah(d.maint_cost_12m) }}<span class="card-unit">12 bulan</span></div>
                <div class="card-sub">bulan ini {{ formatRupiah(d.maint_cost_mtd) }}</div>
              </template>
              <template v-else>
                <div class="card-val card-val--muted">Belum ada</div>
                <div class="card-sub">
                  Belum ada perawatan tercatat.
                  <router-link class="lnk" to="/aset/perawatan">Catat perawatan</router-link>
                </div>
              </template>
            </div>

            <component :is="d.disposed_ytd > 0 ? 'router-link' : 'div'" to="/aset/penghapusan"
                       class="card" :class="d.disposed_ytd > 0 ? 'card--link' : ''">
              <div class="card-label">Pelepasan Tahun Ini</div>
              <template v-if="d.disposed_ytd > 0">
                <div class="card-val">{{ d.disposed_ytd }}<span class="card-unit">aset</span></div>
                <div class="card-sub">
                  hasil {{ formatRupiah(d.proceeds_ytd) }} ·
                  <span :class="d.gain_loss_ytd < 0 ? 'txt-loss' : 'txt-gain'">
                    {{ d.gain_loss_ytd < 0 ? 'rugi' : 'laba' }} {{ formatRupiah(Math.abs(d.gain_loss_ytd)) }}
                  </span>
                </div>
              </template>
              <template v-else>
                <div class="card-val card-val--muted">Tidak ada</div>
                <div class="card-sub">belum ada aset dilepas tahun ini</div>
              </template>
            </component>
          </div>

          <!-- Chip tipis: hanya muncul bila memang ada yang perlu diperhatikan -->
          <div v-if="attentionNotes.length" class="note-row">
            <router-link v-for="n in attentionNotes" :key="n.text" :to="n.to" class="note" :class="`note--${n.tone}`">
              <span class="note-dot"></span>{{ n.text }}
            </router-link>
          </div>
        </section>

        <!-- ══ Perawatan mendesak + kondisi ══ -->
        <div class="grid-wide">
          <AppCard>
            <h2 class="sec-title">Perawatan Paling Mendesak</h2>
            <ul v-if="d.upcoming_maint.length" class="divide-y divide-gray-100">
              <li v-for="a in d.upcoming_maint" :key="a.id" class="row">
                <div class="min-w-0">
                  <p class="row-name">{{ a.name }}</p>
                  <p class="row-meta">{{ a.outlet_name }}<span v-if="a.location"> · {{ a.location }}</span></p>
                </div>
                <div class="text-right shrink-0">
                  <span class="pill" :class="a.due_in_days < 0 ? 'pill--alert' : 'pill--warn'">{{ dueText(a) }}</span>
                  <span class="row-date">{{ formatDateStr(a.next_due_date) }}</span>
                </div>
              </li>
            </ul>
            <div v-else class="empty-note" :class="`empty-note--${upcomingEmpty.tone}`">
              <p class="empty-note-text">{{ upcomingEmpty.text }}</p>
              <p v-if="upcomingEmpty.hint" class="empty-note-hint">{{ upcomingEmpty.hint }}</p>
              <router-link v-if="upcomingEmpty.to" class="lnk" :to="upcomingEmpty.to">{{ upcomingEmpty.action }}</router-link>
            </div>
          </AppCard>

          <AppCard>
            <h2 class="sec-title">Kondisi Aset</h2>
            <ul v-if="conditionRows.length" class="space-y-2.5">
              <li v-for="b in conditionRows" :key="b.key">
                <div class="bar-hd">
                  <span class="bar-lbl-txt">{{ b.label }}</span>
                  <span class="bar-val">{{ b.count }} aset</span>
                </div>
                <div class="meter"><div class="meter-fill" :class="condFill(b.key)" :style="{ width: pctOf(b.count, d.total_assets) + '%' }"></div></div>
              </li>
            </ul>
            <div v-else class="empty-note">
              <p class="empty-note-text">Belum ada data kondisi.</p>
            </div>
          </AppCard>
        </div>

        <!-- ══ Sebaran nilai ══ -->
        <div class="grid-2">
          <AppCard>
            <h2 class="sec-title">Nilai Buku per Outlet</h2>
            <ul v-if="d.by_outlet.length" class="space-y-2.5">
              <li v-for="b in d.by_outlet" :key="b.key">
                <div class="bar-hd">
                  <span class="bar-lbl-txt">{{ b.label }}</span>
                  <span class="bar-val">{{ formatRupiah(b.value) }}</span>
                </div>
                <div class="meter"><div class="meter-fill" :style="{ width: pctOf(b.value, maxOutlet) + '%' }"></div></div>
                <p class="bar-meta">{{ b.count }} aset · {{ b.quantity }} unit</p>
              </li>
            </ul>
            <div v-else class="empty-note"><p class="empty-note-text">Belum ada aset di outlet mana pun.</p></div>
          </AppCard>

          <AppCard>
            <h2 class="sec-title">Nilai Buku per Kategori</h2>
            <ul v-if="d.by_category.length" class="space-y-2.5">
              <li v-for="b in d.by_category" :key="b.key">
                <div class="bar-hd">
                  <span class="bar-lbl-txt">{{ b.label }}</span>
                  <span class="bar-val">{{ formatRupiah(b.value) }}</span>
                </div>
                <div class="meter"><div class="meter-fill meter-fill--alt" :style="{ width: pctOf(b.value, maxCategory) + '%' }"></div></div>
                <p class="bar-meta">{{ b.count }} aset · {{ b.quantity }} unit</p>
              </li>
            </ul>
            <div v-else class="empty-note">
              <p class="empty-note-text">Aset belum dikelompokkan ke kategori mana pun.</p>
              <p class="empty-note-hint">Isi kolom Kategori saat menyunting aset agar sebarannya terbaca.</p>
              <router-link class="lnk" to="/aset/daftar">Buka Daftar Aset</router-link>
            </div>
          </AppCard>
        </div>

        <!-- ══ Tren 12 bulan ══ -->
        <div class="grid-2">
          <AppCard>
            <h2 class="sec-title sec-title--tight">Perolehan 12 Bulan Terakhir</h2>
            <p v-if="acqTrendHasValue" class="chart-max">tertinggi {{ formatRupiah(maxAcq) }}</p>
            <div v-if="acqTrendHasValue" class="bars">
              <div v-for="m in d.acquisition_trend" :key="m.month" class="bar-col"
                   :title="`${monthLabel(m.month)}: ${formatRupiah(m.value)} (${m.count}×)`">
                <div class="bar bar--acq" :class="{ 'bar--zero': !m.value }" :style="{ height: barH(m.value, maxAcq) }"></div>
                <span class="bar-lbl">{{ monthLabel(m.month) }}</span>
              </div>
            </div>
            <div v-else class="empty-note">
              <p class="empty-note-text">Tidak ada perolehan dalam 12 bulan terakhir.</p>
              <p class="empty-note-hint">Aset yang ada diperoleh lebih lama dari itu, atau perolehannya belum dicatat.</p>
              <router-link class="lnk" to="/aset/perolehan">Catat perolehan</router-link>
            </div>
          </AppCard>

          <AppCard>
            <h2 class="sec-title sec-title--tight">Biaya Perawatan 12 Bulan Terakhir</h2>
            <p v-if="maintTrendHasValue" class="chart-max">tertinggi {{ formatRupiah(maxMaint) }}</p>
            <div v-if="maintTrendHasValue" class="bars">
              <div v-for="m in d.maintenance_trend" :key="m.month" class="bar-col"
                   :title="`${monthLabel(m.month)}: ${formatRupiah(m.value)} (${m.count}×)`">
                <div class="bar bar--maint" :class="{ 'bar--zero': !m.value }" :style="{ height: barH(m.value, maxMaint) }"></div>
                <span class="bar-lbl">{{ monthLabel(m.month) }}</span>
              </div>
            </div>
            <div v-else class="empty-note">
              <p class="empty-note-text">
                {{ d.maintenance_trend.length ? 'Ada perawatan tercatat, tapi semuanya tanpa biaya.' : 'Belum ada perawatan tercatat.' }}
              </p>
              <p class="empty-note-hint">Setiap catatan perawatan beserta biayanya akan muncul di sini.</p>
              <router-link class="lnk" to="/aset/perawatan">Catat perawatan</router-link>
            </div>
          </AppCard>
        </div>
      </template>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { assetsApi } from '@/api/assets.js'
import { outletsApi } from '@/api/outlets.js'
import { formatRupiah, formatDateStr } from '@/utils/format.js'
import { useRealtime } from '@/utils/realtime.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppSpinner from '@/components/ui/AppSpinner.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'

const CONDITIONS = { baik: 'Baik', rusak_ringan: 'Rusak Ringan', rusak_berat: 'Rusak Berat', perbaikan: 'Dalam Perbaikan' }
const BULAN = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']

const d = ref(null)
const outlets = ref([])
const loading = ref(false)
const errorMsg = ref('')
// Outlet terpilih ikut di URL supaya tampilan tersaring bisa di-bookmark
// dan dibagikan, bukan hanya hidup di dalam komponen.
const route = useRoute()
const router = useRouter()
const filterOutlet = ref(typeof route.query.outlet_id === 'string' ? route.query.outlet_id : '')

const outletFilterOptions = computed(() => [{ id: '', name: 'Semua outlet' }, ...outlets.value])
const outletName = computed(() => outlets.value.find(o => o.id === filterOutlet.value)?.name || '')

const isEmpty = computed(() => (d.value?.total_assets ?? 0) === 0)
const emptyTitle = computed(() =>
  filterOutlet.value ? `${outletName.value || 'Outlet ini'} belum punya aset` : 'Belum ada aset tercatat')
const emptyDesc = computed(() =>
  filterOutlet.value
    ? 'Aset yang dicatat untuk outlet ini akan langsung terangkum di sini — nilai, jadwal perawatan, dan riwayatnya.'
    : 'Modul ini merangkum seluruh barang inventaris outlet — meja, kursi, elektronik, peralatan dapur — sejak dibeli sampai dilepas.')

const hasDepreciation = computed(() => (d.value?.monthly_deprec ?? 0) > 0 || (d.value?.accumulated_deprec ?? 0) > 0)
const deprecPct = computed(() => pctOf(d.value?.accumulated_deprec, d.value?.acquisition_cost))
const conditionRows = computed(() =>
  (d.value?.condition_breakdown || []).map(b => ({ ...b, label: CONDITIONS[b.key] || b.label })))

const maxOutlet = computed(() => Math.max(...(d.value?.by_outlet || []).map(b => b.value), 1))
const maxCategory = computed(() => Math.max(...(d.value?.by_category || []).map(b => b.value), 1))
const maxAcq = computed(() => Math.max(...(d.value?.acquisition_trend || []).map(m => m.value), 1))
const maxMaint = computed(() => Math.max(...(d.value?.maintenance_trend || []).map(m => m.value), 1))
// Bulan yang tercatat tapi semuanya berbiaya nol tetap menghasilkan grafik
// datar tak terbaca — perlakukan sebagai kosong, dengan penjelasannya sendiri.
const maintTrendHasValue = computed(() => (d.value?.maintenance_trend || []).some(m => Number(m.value) > 0))
const acqTrendHasValue = computed(() => (d.value?.acquisition_trend || []).some(m => Number(m.value) > 0))

function pctOf(val, total) {
  if (!total) return 0
  return Math.min(Math.round((Number(val || 0) / Number(total)) * 100), 100)
}
// Bulan bernilai nol digambar sebagai garis dasar tipis, bukan batang minimum —
// batang pendek terbaca sebagai "ada sedikit", padahal tidak ada sama sekali.
function barH(val, max) {
  const n = Number(val || 0)
  if (!n) return '2px'
  return `${Math.max((n / (max || 1)) * 100, 6)}%`
}
function monthLabel(ym) {
  const [y, m] = ym.split('-')
  const nama = BULAN[Number(m) - 1] || m
  return m === '01' ? `${nama} '${y.slice(2)}` : nama
}
function condFill(key) {
  return {
    'meter-fill--good': key === 'baik',
    'meter-fill--warn': key === 'rusak_ringan' || key === 'perbaikan',
    'meter-fill--alert': key === 'rusak_berat',
  }
}
function dueText(a) {
  if (a.due_in_days < 0) return `Terlambat ${Math.abs(a.due_in_days)} hari`
  if (a.due_in_days === 0) return 'Hari ini'
  return `${a.due_in_days} hari lagi`
}

const attentionNotes = computed(() => {
  const x = d.value
  if (!x) return []
  const notes = []
  if (x.needs_attention > 0) notes.push({ text: `${x.needs_attention} aset berkondisi tidak baik`, to: '/aset/daftar', tone: 'warn' })
  if (x.ending_soon_count > 0) notes.push({ text: `${x.ending_soon_count} aset mendekati akhir umur ekonomis`, to: '/aset/penyusutan', tone: 'warn' })
  if (x.unscheduled > 0) notes.push({ text: `${x.unscheduled} aset belum punya jadwal perawatan`, to: '/aset/daftar?due=none', tone: 'muted' })
  return notes
})

// Kosongnya daftar "paling mendesak" punya dua arti yang sangat berbeda.
const upcomingEmpty = computed(() => {
  const x = d.value
  if (x && x.unscheduled >= x.total_assets) {
    return {
      tone: 'muted',
      text: 'Belum ada jadwal perawatan sama sekali.',
      hint: 'Catat satu perawatan dan isi tanggal berikutnya — sejak itu aset akan mengingatkan sendiri saat jatuh tempo.',
      to: '/aset/perawatan', action: 'Catat perawatan pertama',
    }
  }
  return {
    tone: 'good',
    text: 'Tidak ada perawatan yang terlambat atau jatuh tempo dalam 7 hari.',
    hint: 'Jadwal yang mendekati tenggat akan muncul di sini secara otomatis.',
  }
})

function clearOutlet() { filterOutlet.value = ''; load() }

async function load() {
  loading.value = true; errorMsg.value = ''
  router.replace({ query: filterOutlet.value ? { outlet_id: filterOutlet.value } : {} }).catch(() => {})
  try {
    d.value = await assetsApi.dashboard({ outlet_id: filterOutlet.value || undefined })
  } catch (e) {
    errorMsg.value = e?.message || 'Gagal memuat dashboard aset'
  } finally {
    loading.value = false
  }
}

async function loadOutlets() {
  try {
    const r = await outletsApi.myOutlets()
    outlets.value = r?.outlets ?? r ?? []
  } catch { outlets.value = [] }
}

useRealtime(['asset_alert'], load)
onMounted(async () => { await loadOutlets(); await load() })
</script>

<style scoped>
/* ── Tata letak ── */
.grid-2 { display: grid; gap: .75rem; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); }
.grid-3 { display: grid; gap: .75rem; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); }
.grid-4 { display: grid; gap: .75rem; grid-template-columns: repeat(auto-fit, minmax(210px, 1fr)); }
/* align-items:start supaya kartu pendek tidak ikut meninggi mengikuti tetangga
   dan menyisakan ruang kosong di dalamnya. */
.grid-wide { display: grid; gap: .75rem; grid-template-columns: 1fr; align-items: start; }
@media (min-width: 1024px) { .grid-wide { grid-template-columns: 1.8fr 1fr; } }

.sec-title {
  font-size: .7rem; font-weight: 700; color: #6b7280; text-transform: uppercase;
  letter-spacing: .06em; margin-bottom: .55rem;
}

/* ── Kartu: putih polos; warna hanya dipakai saat menandakan sesuatu ── */
.card {
  display: block; border-radius: .85rem; padding: .85rem 1rem; background: #fff;
  border: 1px solid rgba(17,24,39,.08); box-shadow: 0 1px 2px rgba(16,24,40,.04);
}
.card--link { transition: box-shadow .12s ease, transform .12s ease; }
.card--link:hover { transform: translateY(-1px); box-shadow: 0 4px 14px rgba(16,24,40,.08); }
.card--alert { border-color: rgba(225,29,72,.28); background: rgba(225,29,72,.035); }
.card--alert .card-val { color: #be123c; }
.card--warn  { border-color: rgba(217,119,6,.3); background: rgba(217,119,6,.04); }
.card--warn .card-val { color: #b45309; }
.card--good .card-val { color: #047857; }

.card-label { font-size: .68rem; font-weight: 700; color: #6b7280; text-transform: uppercase; letter-spacing: .04em; }
.card-val { font-size: 1.45rem; font-weight: 800; color: #111827; line-height: 1.25; margin-top: .2rem; letter-spacing: -.01em; }
.card-val--muted { font-size: 1.05rem; color: #9ca3af; font-weight: 700; }
.card-unit { font-size: .7rem; font-weight: 600; color: #9ca3af; margin-left: .3rem; letter-spacing: 0; }
.card-sub { font-size: .72rem; color: #6b7280; margin-top: .25rem; line-height: 1.5; }
.card-sub strong { color: #374151; font-weight: 700; }

.txt-loss { color: #be123c; font-weight: 700; }
.txt-gain { color: #047857; font-weight: 700; }

/* ── Meter ── */
.meter { height: .3rem; border-radius: 999px; background: #f1f5f9; overflow: hidden; margin: .45rem 0 .25rem; }
.meter-fill { height: 100%; border-radius: 999px; background: #6366f1; transition: width .3s ease; }
.meter-fill--alt { background: #0ea5e9; }
.meter-fill--good { background: #10b981; }
.meter-fill--warn { background: #f59e0b; }
.meter-fill--alert { background: #f43f5e; }

.bar-hd { display: flex; align-items: baseline; justify-content: space-between; gap: .5rem; }
.bar-lbl-txt { font-size: .82rem; color: #374151; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.bar-val { font-size: .75rem; font-weight: 700; color: #111827; flex-shrink: 0; }
.bar-meta { font-size: .67rem; color: #9ca3af; }

.lnk { color: #047857; font-weight: 600; text-decoration: none; }
.lnk:hover { text-decoration: underline; }

/* ── Chip perhatian ── */
.note-row { display: flex; flex-wrap: wrap; gap: .4rem; margin-top: .6rem; }
.note {
  display: inline-flex; align-items: center; gap: .4rem; padding: .3rem .6rem; border-radius: 999px;
  font-size: .72rem; font-weight: 600; text-decoration: none; border: 1px solid transparent;
}
.note-dot { width: .35rem; height: .35rem; border-radius: 999px; background: currentColor; }
.note--warn { background: rgba(217,119,6,.08); color: #b45309; border-color: rgba(217,119,6,.2); }
.note--muted { background: rgba(107,114,128,.08); color: #4b5563; border-color: rgba(107,114,128,.18); }
.note:hover { filter: brightness(.97); }

/* ── Baris daftar ── */
.row { padding: .55rem 0; display: flex; align-items: center; justify-content: space-between; gap: .75rem; }
.row-name { font-size: .85rem; font-weight: 600; color: #111827; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.row-meta { font-size: .72rem; color: #6b7280; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.row-date { font-size: .68rem; color: #9ca3af; display: block; margin-top: .15rem; }

/* ── Keadaan kosong ── */
.empty-note { padding: 1.4rem .25rem; text-align: center; }
.empty-note-text { font-size: .85rem; font-weight: 600; color: #374151; }
.empty-note-hint { font-size: .74rem; color: #6b7280; margin-top: .3rem; line-height: 1.55; max-width: 30rem; margin-inline: auto; }
.empty-note .lnk { display: inline-block; margin-top: .6rem; font-size: .76rem; }
.empty-note--good .empty-note-text { color: #047857; }

.empty-hero { text-align: center; padding: 1.5rem .5rem .5rem; }
.empty-hero-ic {
  width: 3rem; height: 3rem; margin: 0 auto .8rem; border-radius: .9rem;
  display: grid; place-items: center; color: #047857; background: rgba(16,185,129,.1);
}
.empty-hero-ic svg { width: 1.6rem; height: 1.6rem; }
.empty-hero-title { font-size: 1.05rem; font-weight: 800; color: #111827; }
.empty-hero-desc { font-size: .82rem; color: #6b7280; margin: .35rem auto 0; max-width: 34rem; line-height: 1.6; }
.empty-hero-list { list-style: none; margin: 1rem auto 0; max-width: 30rem; text-align: left; display: grid; gap: .4rem; }
.empty-hero-list li { font-size: .78rem; color: #4b5563; padding-left: .9rem; position: relative; line-height: 1.5; }
.empty-hero-list li::before {
  content: ''; position: absolute; left: 0; top: .5rem;
  width: .3rem; height: .3rem; border-radius: 999px; background: #10b981;
}
.empty-hero-list strong { color: #111827; font-weight: 700; }
.empty-hero-act { display: flex; justify-content: center; gap: .5rem; flex-wrap: wrap; margin-top: 1.2rem; }
.btn-primary { padding: .5rem 1rem; border-radius: .6rem; font-size: .82rem; font-weight: 600; background: #059669; color: #fff; text-decoration: none; }
.btn-primary:hover { background: #047857; }
.btn-quiet { padding: .5rem 1rem; border-radius: .6rem; font-size: .82rem; font-weight: 600; background: #f3f4f6; color: #374151; }
.btn-quiet:hover { background: #e5e7eb; }

/* ── Grafik batang bulanan ── */
/* Caption ditaruh di barisnya sendiri, bukan sejajar judul — sejajar judul
   membuatnya bertabrakan dengan batang tertinggi yang mencapai puncak area. */
.sec-title--tight { margin-bottom: .15rem; }
.chart-max { font-size: .68rem; color: #9ca3af; font-weight: 600; margin-bottom: .6rem; }
.bars { display: flex; align-items: flex-end; gap: .3rem; height: 8.5rem; padding: 0 .15rem; }
.bar-col {
  flex: 1 1 0; min-width: 0; height: 100%;
  display: flex; flex-direction: column; align-items: center; justify-content: flex-end; gap: .3rem;
}
.bar { width: 100%; max-width: 2.25rem; border-radius: .25rem .25rem 0 0; flex-shrink: 0; transition: height .3s ease; }
.bar--acq { background: #6366f1; }
.bar--maint { background: #f59e0b; }
.bar--zero { background: #e5e7eb; border-radius: 999px; }
.bar-lbl { font-size: .6rem; color: #9ca3af; flex-shrink: 0; line-height: 1; white-space: nowrap; }

/* ── Lain-lain ── */
.pill { display: inline-block; padding: .12rem .5rem; border-radius: 999px; font-size: .68rem; font-weight: 700; white-space: nowrap; }
.pill--alert { background: rgba(225,29,72,.12); color: #be123c; }
.pill--warn { background: rgba(217,119,6,.14); color: #b45309; }

.refresh-btn {
  display: inline-flex; align-items: center; gap: .4rem; padding: .45rem .8rem; border-radius: .6rem;
  font-size: .78rem; font-weight: 600; color: #374151; background: #fff; border: 1px solid rgba(17,24,39,.1);
}
.refresh-btn:hover { background: #f9fafb; }
.refresh-btn:disabled { opacity: .55; }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>
