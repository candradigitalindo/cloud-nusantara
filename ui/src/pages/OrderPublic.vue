<template>
  <div class="op">
    <div class="op-atmo" aria-hidden="true">
      <div class="orb o1" /><div class="orb o2" /><div class="orb o3" />
    </div>

    <header class="op-head">
      <div class="op-head-in">
        <div class="op-brand">
          <div class="op-logo">
            <svg viewBox="0 0 32 32" fill="none"><rect x="2" y="2" width="13" height="13" rx="3" fill="rgba(126,184,154,.9)"/><rect x="17" y="2" width="13" height="13" rx="3" fill="rgba(126,184,154,.55)"/><rect x="2" y="17" width="13" height="13" rx="3" fill="rgba(126,184,154,.55)"/><rect x="17" y="17" width="13" height="13" rx="3" fill="rgba(126,184,154,.75)"/></svg>
          </div>
          <div>
            <p class="op-eyebrow">Pesan dari Meja</p>
            <h1 class="op-title">{{ menu?.outlet_name || 'Memuat…' }}</h1>
          </div>
        </div>
        <div v-if="tableNumber" class="op-table">Meja {{ tableNumber }}</div>
      </div>
    </header>

    <div v-if="loading" class="op-state">Memuat menu…</div>
    <div v-else-if="loadError" class="op-state">{{ loadError }}</div>

    <!-- Pesanan terkirim -->
    <div v-else-if="done" class="op-wrap">
      <div class="glass op-success">
        <div class="op-check">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6L9 17l-5-5"/></svg>
        </div>
        <h2>Pesanan terkirim!</h2>
        <p>Pesanan Anda untuk <b>Meja {{ done.table_number }}</b> sudah diteruskan ke dapur. Silakan tunggu di meja.</p>
        <div class="op-status">
          <span>Status</span>
          <b>{{ statusLabel }}</b>
        </div>
        <button class="op-next" @click="startOver">Pesan Lagi</button>
      </div>
    </div>

    <template v-else>
      <!-- Kategori -->
      <div class="op-cats">
        <div class="op-cats-in">
          <button class="op-chip" :class="{ 'op-chip--on': activeCat === '' }" @click="activeCat = ''">
            Semua <span class="op-chip-n">{{ allProducts.length }}</span>
          </button>
          <button
            v-for="c in menu.categories" :key="c.name"
            class="op-chip" :class="{ 'op-chip--on': activeCat === c.name }"
            @click="activeCat = c.name"
          >
            {{ c.name }} <span class="op-chip-n">{{ c.products.length }}</span>
          </button>
        </div>
      </div>

      <div class="op-wrap">
        <div class="glass op-search">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7"/><path d="M21 21l-4.3-4.3"/></svg>
          <input v-model="searchQuery" type="search" placeholder="Cari menu…" />
        </div>

        <p v-if="!filteredProducts.length" class="op-empty">Menu tidak ditemukan.</p>

        <div class="op-grid">
          <div
            v-for="p in filteredProducts" :key="p.id"
            class="glass op-card" :class="{ 'op-card--active': qtyOf(p.id) > 0 }"
          >
            <div class="op-photo">
              <img v-if="p.photo_url" :src="p.photo_url" :alt="p.name" loading="lazy" />
              <div v-else class="op-initials">{{ initials(p.name) }}</div>
              <div v-if="qtyOf(p.id) > 0" class="op-qty-badge">{{ qtyOf(p.id) }}</div>
            </div>
            <div class="op-card-body">
              <div class="op-name">{{ p.name }}</div>
              <div class="op-price">{{ rupiah(p.price) }}</div>
              <div v-if="p.addons?.length" class="op-has-addons">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 21v-7M4 10V3M12 21v-9M12 8V3M20 21v-5M20 12V3M1 14h6M9 8h6M17 16h6"/></svg>
                {{ p.addons.length }} tambahan
              </div>
              <div class="op-add" @click="openItem(p)">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M12 5v14M5 12h14"/></svg>
                Tambah
              </div>
            </div>
          </div>
        </div>

        <!-- Ringkasan pesanan -->
        <div v-if="lines.length" class="glass op-order">
          <h2 class="op-section">Pesanan Anda</h2>
          <ul class="op-order-list">
            <li v-for="(l, i) in lines" :key="l.key">
              <span class="op-oi-qty">{{ l.qty }}×</span>
              <span class="op-oi-name">
                {{ l.product.name }}
                <em v-if="l.addons.length">+ {{ l.addons.map(a => a.name).join(', ') }}</em>
                <em v-if="l.notes" class="op-oi-note">{{ l.notes }}</em>
              </span>
              <span class="op-oi-sub">{{ rupiah(lineTotal(l)) }}</span>
              <button class="op-oi-del" @click="lines.splice(i, 1)" aria-label="Hapus">×</button>
            </li>
          </ul>
          <div class="op-order-total"><span>Total</span><b>{{ rupiah(grandTotal) }}</b></div>
          <p class="op-note-total">Belum termasuk pajak &amp; biaya layanan — dihitung kasir saat menagih.</p>
        </div>

        <!-- Identitas -->
        <div class="glass op-form">
          <h2 class="op-section">Data Pemesan</h2>
          <div class="op-fields">
            <div>
              <label>Nomor Meja <span>*</span></label>
              <input v-model="f.table_number" placeholder="Contoh: A5" :disabled="lockedTable" />
            </div>
            <div>
              <label>Nama (opsional)</label>
              <input v-model="f.customer_name" placeholder="Nama Anda" />
            </div>
            <div>
              <label>Catatan untuk dapur (opsional)</label>
              <textarea v-model="f.notes" rows="2" placeholder="Contoh: tidak pedas" />
            </div>
          </div>
          <p v-if="formError" class="op-err">{{ formError }}</p>
        </div>
      </div>

      <!-- Bar aksi -->
      <div class="op-bar">
        <div class="op-bar-in">
          <div class="op-bar-info">
            <span class="op-bar-count">{{ totalQty }} item</span>
            <span class="op-bar-total">{{ rupiah(grandTotal) }}</span>
          </div>
          <button class="op-btn" :disabled="submitting || !lines.length" @click="submit">
            {{ submitting ? 'Mengirim…' : 'Kirim Pesanan' }}
          </button>
        </div>
      </div>
    </template>

    <!-- Dialog pilih add-on -->
    <div v-if="picking" class="op-modal" @click.self="picking = null">
      <div class="glass op-modal-box">
        <h3>{{ picking.product.name }}</h3>
        <p class="op-modal-price">{{ rupiah(picking.product.price) }}</p>

        <div v-if="picking.product.addons?.length" class="op-addons">
          <label
            v-for="a in picking.product.addons" :key="a.local_id"
            class="op-addon" :class="{ 'op-addon--on': picking.selected.includes(a.local_id) }"
          >
            <input type="checkbox" :value="a.local_id" v-model="picking.selected" />
            <span class="op-addon-name">{{ a.name }}</span>
            <span class="op-addon-price">{{ a.price > 0 ? '+' + rupiah(a.price) : 'Gratis' }}</span>
          </label>
        </div>

        <div class="op-fields op-modal-notes">
          <div>
            <label>Catatan item (opsional)</label>
            <input v-model="picking.notes" placeholder="Contoh: es sedikit" />
          </div>
        </div>

        <div class="op-modal-qty">
          <button @click="picking.qty > 1 && picking.qty--">−</button>
          <span>{{ picking.qty }}</span>
          <button @click="picking.qty < 99 && picking.qty++">+</button>
        </div>

        <div class="op-modal-actions">
          <button class="op-modal-cancel" @click="picking = null">Batal</button>
          <button class="op-btn" @click="confirmItem">
            Tambahkan · {{ rupiah(pickingTotal) }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { publicApi } from '@/api/public.js'

const route = useRoute()
const slug = route.params.slug

// Nomor meja boleh datang dari QR (?meja=A5). Bila ada, field-nya dikunci
// supaya tamu tidak sengaja memesan atas nama meja lain.
const tableFromQr = String(route.query.meja || route.query.table || '').trim()
const lockedTable = !!tableFromQr

const menu = ref(null)
const loading = ref(true)
const loadError = ref('')
const submitting = ref(false)
const formError = ref('')
const done = ref(null)
const statusLabel = ref('Menunggu diproses')
const activeCat = ref('')
const searchQuery = ref('')

// Satu baris = satu racikan. Menu yang sama dengan add-on berbeda menjadi dua
// baris terpisah, jadi tamu bisa memesan dua nasi goreng dengan tambahan
// berbeda dalam satu kali kirim.
const lines = ref([])
const picking = ref(null)

const f = reactive({ table_number: tableFromQr, customer_name: '', notes: '' })
const tableNumber = computed(() => f.table_number)

const allProducts = computed(() => menu.value?.categories.flatMap(c => c.products) || [])
const baseProducts = computed(() =>
  activeCat.value
    ? (menu.value.categories.find(c => c.name === activeCat.value)?.products || [])
    : allProducts.value
)
const filteredProducts = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return q ? baseProducts.value.filter(p => p.name.toLowerCase().includes(q)) : baseProducts.value
})

const totalQty = computed(() => lines.value.reduce((s, l) => s + l.qty, 0))
const grandTotal = computed(() => lines.value.reduce((s, l) => s + lineTotal(l), 0))
const pickingTotal = computed(() => {
  if (!picking.value) return 0
  return unitPrice(picking.value.product, selectedAddons(picking.value)) * picking.value.qty
})

function rupiah(v) { return 'Rp ' + new Intl.NumberFormat('id-ID').format(Math.round(v || 0)) }
function initials(n) { return (n || '?').trim().slice(0, 2).toUpperCase() }
function qtyOf(productId) {
  return lines.value.filter(l => l.product.id === productId).reduce((s, l) => s + l.qty, 0)
}
function unitPrice(product, addons) {
  return (product.price || 0) + addons.reduce((s, a) => s + (a.price || 0), 0)
}
function lineTotal(l) { return unitPrice(l.product, l.addons) * l.qty }

function selectedAddons(p) {
  return (p.product.addons || []).filter(a => p.selected.includes(a.local_id))
}

function openItem(product) {
  picking.value = { product, selected: [], notes: '', qty: 1 }
}

function confirmItem() {
  const p = picking.value
  if (!p) return
  const addons = selectedAddons(p)
  // Racikan identik digabung agar daftar pesanan tidak penuh baris kembar.
  const key = p.product.id + '|' + addons.map(a => a.local_id).sort().join(',') + '|' + p.notes.trim()
  const existing = lines.value.find(l => l.key === key)
  if (existing) {
    existing.qty += p.qty
  } else {
    lines.value.push({
      key,
      product: p.product,
      addons,
      notes: p.notes.trim(),
      qty: p.qty,
    })
  }
  picking.value = null
}

function startOver() {
  done.value = null
  lines.value = []
  f.customer_name = ''
  f.notes = ''
  if (!lockedTable) f.table_number = ''
}

async function submit() {
  formError.value = ''
  if (!f.table_number.trim()) { formError.value = 'Nomor meja wajib diisi'; return }
  if (!lines.value.length) { formError.value = 'Pilih menu terlebih dahulu'; return }

  submitting.value = true
  try {
    const res = await publicApi.order(slug, {
      table_number: f.table_number.trim(),
      customer_name: f.customer_name.trim(),
      notes: f.notes.trim(),
      items: lines.value.map(l => ({
        product_local_id: l.product.id,
        qty: l.qty,
        notes: l.notes,
        addons: l.addons.map(a => ({ id: a.local_id })),
      })),
    })
    done.value = res.data
    pollStatus(res.data.id)
  } catch (e) {
    formError.value = e?.response?.data?.error || 'Gagal mengirim pesanan. Coba lagi.'
  } finally {
    submitting.value = false
  }
}

// Pantau sampai kasir memproses, lalu berhenti. Interval longgar karena ini
// hanya informasi untuk tamu, bukan jalur yang menentukan pesanan masuk.
let statusTimer = null
function pollStatus(id) {
  clearInterval(statusTimer)
  statusTimer = setInterval(async () => {
    try {
      const res = await publicApi.orderStatus(slug, id)
      const s = res.data?.status
      if (s === 'confirmed') {
        statusLabel.value = 'Diterima dapur'
        clearInterval(statusTimer)
      } else if (s === 'rejected') {
        statusLabel.value = 'Ditolak — hubungi pramusaji'
        clearInterval(statusTimer)
      }
    } catch { /* jaringan tamu putus — coba lagi di tick berikutnya */ }
  }, 5000)
}

onMounted(async () => {
  try {
    const res = await publicApi.menu(slug)
    menu.value = res.data
  } catch (e) {
    loadError.value = e?.response?.data?.error || 'Menu tidak dapat dimuat.'
  } finally {
    loading.value = false
  }
})

onUnmounted(() => clearInterval(statusTimer))
</script>

<style scoped>
.op { position: relative; min-height: 100vh; overflow: hidden; padding-bottom: 5.5rem; color: rgba(255,255,255,.92);
  background:
    radial-gradient(ellipse 110% 70% at 20% 100%, #253f2d 0%, transparent 55%),
    radial-gradient(ellipse 90% 60% at 85% 0%, #1e3427 0%, transparent 55%),
    #182b20;
}
.op-atmo { position: absolute; inset: 0; pointer-events: none; overflow: hidden; }
.orb { position: absolute; border-radius: 50%; filter: blur(80px); }
.o1 { width: 560px; height: 480px; top: -120px; left: -80px; background: radial-gradient(circle at 40% 40%, rgba(74,130,100,.16), rgba(50,100,78,.04)); }
.o2 { width: 420px; height: 420px; bottom: -100px; right: -60px; background: radial-gradient(circle at 60% 60%, rgba(126,184,154,.13), rgba(50,100,78,.03)); }
.o3 { width: 280px; height: 280px; top: 45%; left: 38%; background: radial-gradient(circle, rgba(120,175,145,.09), transparent 70%); }

.glass {
  background: linear-gradient(145deg, rgba(255,255,255,.12), rgba(255,255,255,.04)), rgba(126,184,154,.06);
  backdrop-filter: blur(22px) saturate(150%); -webkit-backdrop-filter: blur(22px) saturate(150%);
  border: 1px solid rgba(255,255,255,.16);
  box-shadow: 0 12px 36px rgba(0,0,0,.32), inset 0 1px 0 rgba(255,255,255,.18);
}

.op-head { position: relative; }
.op-head-in { max-width: 760px; margin: 0 auto; padding: 1.4rem 1.1rem 1.2rem; display: flex; align-items: center; gap: 1rem; }
.op-brand { display: flex; align-items: center; gap: .8rem; flex: 1; min-width: 0; }
.op-logo { width: 2.6rem; height: 2.6rem; border-radius: .7rem; display: flex; align-items: center; justify-content: center; background: rgba(126,184,154,.12); border: 1px solid rgba(255,255,255,.14); flex-shrink: 0; }
.op-logo svg { width: 1.5rem; height: 1.5rem; }
.op-eyebrow { font-size: .66rem; font-weight: 800; letter-spacing: .14em; text-transform: uppercase; color: rgba(168,203,191,.8); }
.op-title { font-size: 1.45rem; font-weight: 900; line-height: 1.1; letter-spacing: -.02em; }
.op-table { flex-shrink: 0; padding: .4rem .8rem; border-radius: 999px; background: rgba(126,184,154,.9); color: #14271d; font-size: .82rem; font-weight: 800; }
.op-state { position: relative; max-width: 760px; margin: 2rem auto; text-align: center; color: rgba(255,255,255,.6); padding: 0 1rem; }

.op-cats { position: sticky; top: 0; z-index: 20; background: rgba(24,43,32,.72); backdrop-filter: blur(14px); border-bottom: 1px solid rgba(255,255,255,.08); }
.op-cats-in { max-width: 760px; margin: 0 auto; padding: .7rem 1.1rem; display: flex; gap: .5rem; overflow-x: auto; scrollbar-width: none; }
.op-cats-in::-webkit-scrollbar { display: none; }
.op-chip { flex-shrink: 0; display: inline-flex; align-items: center; gap: .35rem; padding: .45rem .85rem; border-radius: 999px; border: 1px solid rgba(255,255,255,.16); background: rgba(255,255,255,.06); font-size: .82rem; font-weight: 600; color: rgba(255,255,255,.78); cursor: pointer; transition: all .15s; }
.op-chip--on { background: rgba(126,184,154,.9); color: #14271d; border-color: rgba(126,184,154,.9); }
.op-chip-n { font-size: .67rem; font-weight: 700; background: rgba(255,255,255,.14); border-radius: 999px; padding: .02rem .4rem; }
.op-chip--on .op-chip-n { background: rgba(20,39,29,.25); }

.op-wrap { position: relative; max-width: 760px; margin: 0 auto; padding: 1.1rem; }
.op-section { font-size: 1rem; font-weight: 800; margin: 0 0 .8rem; color: #fff; }

.op-search { display: flex; align-items: center; gap: .55rem; padding: .65rem .85rem; border-radius: .85rem; margin-bottom: 1.1rem; }
.op-search svg { width: 1.1rem; height: 1.1rem; color: rgba(168,203,191,.8); flex-shrink: 0; }
.op-search input { flex: 1; background: transparent; border: none; outline: none; color: #fff; font-size: .9rem; }
.op-search input::placeholder { color: rgba(255,255,255,.4); }

.op-empty { text-align: center; color: rgba(255,255,255,.5); padding: 2rem 0; font-size: .9rem; }
.op-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: .8rem; }
@media (min-width: 560px) { .op-grid { grid-template-columns: repeat(3, 1fr); } }
.op-card { border-radius: 1rem; overflow: hidden; display: flex; flex-direction: column; transition: box-shadow .15s, border-color .15s; }
.op-card--active { border-color: rgba(126,184,154,.7); box-shadow: 0 8px 22px rgba(126,184,154,.22), inset 0 1px 0 rgba(255,255,255,.2); }
.op-photo { position: relative; aspect-ratio: 1/1; background: rgba(255,255,255,.05); }
.op-photo img { width: 100%; height: 100%; object-fit: cover; }
.op-initials { width: 100%; height: 100%; display: flex; align-items: center; justify-content: center; font-size: 2.2rem; font-weight: 900; }
.op-qty-badge { position: absolute; top: .5rem; right: .5rem; min-width: 1.5rem; height: 1.5rem; padding: 0 .4rem; border-radius: 999px; background: #7eb89a; color: #14271d; font-size: .8rem; font-weight: 800; display: flex; align-items: center; justify-content: center; }
.op-card-body { padding: .6rem .65rem .65rem; display: flex; flex-direction: column; gap: .2rem; flex: 1; }
.op-name { font-size: .82rem; font-weight: 600; line-height: 1.25; color: rgba(255,255,255,.92); }
.op-price { font-size: .85rem; font-weight: 800; color: #9fd4ba; }
.op-has-addons { display: flex; align-items: center; gap: .25rem; font-size: .68rem; font-weight: 600; color: rgba(168,203,191,.75); margin-top: auto; }
.op-has-addons svg { width: .75rem; height: .75rem; }
.op-add { margin-top: .45rem; display: flex; align-items: center; justify-content: center; gap: .25rem; padding: .42rem; border-radius: .6rem; background: rgba(126,184,154,.18); color: #cdeede; font-size: .8rem; font-weight: 700; cursor: pointer; user-select: none; }
.op-add:hover { background: rgba(126,184,154,.28); }
.op-add svg { width: .9rem; height: .9rem; }

.op-order { border-radius: 1rem; padding: 1.1rem; margin-top: 1.1rem; }
.op-order-list { list-style: none; margin: 0; padding: 0; }
.op-order-list li { display: flex; align-items: flex-start; gap: .6rem; padding: .5rem 0; border-bottom: 1px dashed rgba(255,255,255,.12); font-size: .88rem; }
.op-oi-name { flex: 1; min-width: 0; }
.op-oi-name em { display: block; font-style: normal; font-size: .76rem; color: rgba(168,203,191,.8); }
.op-oi-note { color: rgba(255,255,255,.5) !important; font-style: italic !important; }
.op-oi-qty { color: rgba(255,255,255,.6); font-weight: 700; }
.op-oi-sub { font-weight: 700; min-width: 5rem; text-align: right; }
.op-oi-del { width: 1.5rem; height: 1.5rem; border: none; background: rgba(239,68,68,.22); color: #fca5a5; border-radius: 50%; font-size: 1rem; cursor: pointer; line-height: 1; flex-shrink: 0; }
.op-order-total { display: flex; justify-content: space-between; align-items: center; padding-top: .7rem; font-size: 1rem; }
.op-order-total b { font-weight: 900; color: #fff; }
.op-note-total { font-size: .72rem; color: rgba(255,255,255,.45); margin-top: .4rem; }

.op-form { border-radius: 1rem; padding: 1.1rem; margin-top: 1.1rem; }
.op-fields { display: flex; flex-direction: column; gap: .8rem; }
.op-fields label { display: block; font-size: .72rem; font-weight: 700; color: rgba(168,203,191,.85); margin-bottom: .25rem; }
.op-fields label span { color: #fca5a5; }
.op-fields input, .op-fields textarea { width: 100%; padding: .65rem .75rem; border-radius: .65rem; border: 1px solid rgba(255,255,255,.16); font-size: .9rem; outline: none; background: rgba(255,255,255,.07); color: #fff; }
.op-fields input:disabled { opacity: .7; }
.op-fields input::placeholder, .op-fields textarea::placeholder { color: rgba(255,255,255,.38); }
.op-fields input:focus, .op-fields textarea:focus { border-color: rgba(126,184,154,.6); box-shadow: 0 0 0 3px rgba(126,184,154,.18); }
.op-err { color: #fca5a5; font-size: .8rem; margin-top: .7rem; }

.op-bar { position: fixed; bottom: 0; left: 0; right: 0; z-index: 30; background: rgba(24,43,32,.82); backdrop-filter: blur(16px); border-top: 1px solid rgba(255,255,255,.1); }
.op-bar-in { max-width: 760px; margin: 0 auto; padding: .7rem 1.1rem; display: flex; align-items: center; gap: 1rem; }
.op-bar-info { display: flex; flex-direction: column; line-height: 1.1; }
.op-bar-count { font-size: .72rem; color: rgba(255,255,255,.6); font-weight: 600; }
.op-bar-total { font-size: 1.05rem; font-weight: 900; color: #fff; }
.op-btn { margin-left: auto; background: linear-gradient(145deg, #7eb89a, #5d9b78); color: #14271d; border: none; padding: .75rem 1.4rem; border-radius: .75rem; font-size: .92rem; font-weight: 800; cursor: pointer; }
.op-btn:disabled { opacity: .5; cursor: not-allowed; }
.op-next { width: 100%; margin-top: 1rem; background: linear-gradient(145deg, #7eb89a, #5d9b78); color: #14271d; border: none; padding: .85rem; border-radius: .8rem; font-size: .95rem; font-weight: 800; cursor: pointer; }

/* Sukses */
.op-success { border-radius: 1rem; padding: 2rem 1.4rem; text-align: center; }
.op-check { width: 3.4rem; height: 3.4rem; margin: 0 auto 1rem; border-radius: 50%; background: rgba(126,184,154,.22); color: #9fd4ba; display: flex; align-items: center; justify-content: center; }
.op-check svg { width: 1.8rem; height: 1.8rem; }
.op-success h2 { font-size: 1.3rem; font-weight: 900; margin-bottom: .5rem; }
.op-success p { color: rgba(255,255,255,.7); font-size: .9rem; }
.op-status { display: flex; justify-content: space-between; margin-top: 1.2rem; padding-top: .9rem; border-top: 1px dashed rgba(255,255,255,.15); font-size: .88rem; }
.op-status b { color: #9fd4ba; font-weight: 800; }

/* Dialog add-on */
.op-modal { position: fixed; inset: 0; z-index: 50; background: rgba(10,20,15,.7); backdrop-filter: blur(4px); display: flex; align-items: flex-end; justify-content: center; padding: 1rem; }
@media (min-width: 560px) { .op-modal { align-items: center; } }
.op-modal-box { width: 100%; max-width: 420px; border-radius: 1.1rem; padding: 1.2rem; max-height: 85vh; overflow-y: auto; }
.op-modal-box h3 { font-size: 1.05rem; font-weight: 800; }
.op-modal-price { color: #9fd4ba; font-weight: 800; font-size: .9rem; margin-bottom: 1rem; }
.op-addons { display: flex; flex-direction: column; gap: .45rem; }
.op-addon { display: flex; align-items: center; gap: .6rem; padding: .6rem .75rem; border-radius: .7rem; border: 1px solid rgba(255,255,255,.14); background: rgba(255,255,255,.05); cursor: pointer; font-size: .88rem; }
.op-addon--on { border-color: rgba(126,184,154,.75); background: rgba(126,184,154,.14); }
.op-addon input { accent-color: #7eb89a; width: 1.05rem; height: 1.05rem; }
.op-addon-name { flex: 1; }
.op-addon-price { font-size: .8rem; font-weight: 700; color: rgba(168,203,191,.9); }
.op-modal-notes { margin-top: 1rem; }
.op-modal-qty { display: flex; align-items: center; justify-content: center; gap: 1.2rem; margin: 1.2rem 0; }
.op-modal-qty button { width: 2.4rem; height: 2.4rem; border: none; background: rgba(255,255,255,.12); border-radius: .6rem; font-size: 1.3rem; font-weight: 700; color: #cdeede; cursor: pointer; }
.op-modal-qty span { font-size: 1.3rem; font-weight: 800; min-width: 2rem; text-align: center; }
.op-modal-actions { display: flex; gap: .6rem; }
.op-modal-actions .op-btn { margin-left: 0; flex: 1; }
.op-modal-cancel { flex: 1; background: rgba(255,255,255,.1); color: rgba(255,255,255,.8); border: none; padding: .75rem; border-radius: .75rem; font-size: .92rem; font-weight: 700; cursor: pointer; }
</style>
