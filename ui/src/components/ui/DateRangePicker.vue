<template>
  <div class="drp">
    <!-- Trigger -->
    <button type="button" class="drp-trigger" @click="open = true">
      <svg class="drp-ico" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <rect x="3" y="4" width="18" height="18" rx="2"/><line x1="16" y1="2" x2="16" y2="6"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="3" y1="10" x2="21" y2="10"/>
      </svg>
      <span class="drp-label">{{ modelValue?.label || 'Pilih tanggal' }}</span>
      <span class="drp-range">{{ rangeText }}</span>
      <svg class="drp-chev" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="6 9 12 15 18 9"/></svg>
    </button>

    <!-- Panel -->
    <Teleport to="body">
      <Transition name="drp-fade">
        <div v-if="open" class="drp-backdrop" @click="open = false" />
      </Transition>
      <Transition name="drp-pop">
        <div v-if="open" class="drp-panel" role="dialog">
          <div class="drp-head">
            <span class="drp-title">Pilih Rentang Tanggal</span>
            <button class="drp-close" @click="open = false">&times;</button>
          </div>

          <div class="drp-body">
            <!-- Presets -->
            <div class="drp-presets">
              <button v-for="p in presets" :key="p.key" type="button"
                :class="['drp-preset', activePreset === p.key && 'is-active']"
                @click="applyPreset(p)">{{ p.label }}</button>
            </div>

            <!-- Calendar -->
            <div class="drp-cal">
              <div class="drp-cal-head">
                <button type="button" class="drp-nav" @click="shiftMonth(-1)">‹</button>
                <span class="drp-cal-title">{{ monthTitle }}</span>
                <button type="button" class="drp-nav" @click="shiftMonth(1)">›</button>
              </div>
              <div class="drp-dow"><span v-for="d in DOW" :key="d">{{ d }}</span></div>
              <div class="drp-grid">
                <span v-for="b in leading" :key="'b'+b" class="drp-blank" />
                <button v-for="day in daysInMonth" :key="day" type="button"
                  :class="['drp-day', dayCls(day)]" @click="pickDay(day)">{{ day }}</button>
              </div>
            </div>
          </div>

          <div class="drp-foot">
            <span class="drp-sel">{{ selText }}</span>
            <button type="button" class="drp-apply" :disabled="!pendFrom || !pendTo" @click="applyCustom">Terapkan</button>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'

const props = defineProps({
  modelValue: { type: Object, default: () => ({}) },
  // Tampilkan preset "Semua Tanggal" (from/to kosong = tanpa filter) — untuk
  // halaman yang default-nya menampilkan semua data (reservasi, shift, dll).
  clearable: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue'])

const open = ref(false)
const DOW = ['Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab', 'Min']
const MONTHS = ['Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni', 'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember']

function ymd(d) { return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}` }
function parseYmd(s) { const [y, m, d] = (s || '').split('-').map(Number); return y ? new Date(y, m - 1, d) : null }
function fmtShort(s) { const d = parseYmd(s); return d ? `${d.getDate()} ${MONTHS[d.getMonth()].slice(0, 3)} ${d.getFullYear()}` : '' }

// ── Presets ──
function startOfWeek(d) { const x = new Date(d); const wd = (x.getDay() + 6) % 7; x.setDate(x.getDate() - wd); return x } // Monday
const presets = computed(() => {
  const t = new Date(); t.setHours(0, 0, 0, 0)
  const y = new Date(t); y.setDate(t.getDate() - 1)
  const sow = startOfWeek(t)
  const sowLast = new Date(sow); sowLast.setDate(sow.getDate() - 7)
  const eowLast = new Date(sow); eowLast.setDate(sow.getDate() - 1)
  const som = new Date(t.getFullYear(), t.getMonth(), 1)
  const somLast = new Date(t.getFullYear(), t.getMonth() - 1, 1)
  const eomLast = new Date(t.getFullYear(), t.getMonth(), 0)
  const soy = new Date(t.getFullYear(), 0, 1)
  const soyLast = new Date(t.getFullYear() - 1, 0, 1)
  const eoyLast = new Date(t.getFullYear() - 1, 11, 31)
  const list = props.clearable ? [{ key: 'all', label: 'Semua Tanggal', from: '', to: '' }] : []
  return [
    ...list,
    { key: 'today', label: 'Hari Ini', from: ymd(t), to: ymd(t) },
    { key: 'yesterday', label: 'Kemarin', from: ymd(y), to: ymd(y) },
    { key: 'this_week', label: 'Minggu Ini', from: ymd(sow), to: ymd(t) },
    { key: 'last_week', label: 'Minggu Lalu', from: ymd(sowLast), to: ymd(eowLast) },
    { key: 'this_month', label: 'Bulan Ini', from: ymd(som), to: ymd(t) },
    { key: 'last_month', label: 'Bulan Lalu', from: ymd(somLast), to: ymd(eomLast) },
    { key: 'this_year', label: 'Tahun Ini', from: ymd(soy), to: ymd(t) },
    { key: 'last_year', label: 'Tahun Lalu', from: ymd(soyLast), to: ymd(eoyLast) },
  ]
})
const activePreset = computed(() => {
  const m = props.modelValue
  const p = presets.value.find(p => p.from === m?.from && p.to === m?.to)
  return p?.key || ''
})

function applyPreset(p) {
  emit('update:modelValue', { from: p.from, to: p.to, label: p.label })
  open.value = false
}

// ── Calendar ──
const view = ref(new Date())
const pendFrom = ref('')
const pendTo = ref('')
watch(open, (v) => {
  if (v) {
    pendFrom.value = props.modelValue?.from || ''
    pendTo.value = props.modelValue?.to || ''
    view.value = parseYmd(props.modelValue?.to) || new Date()
  }
})
const monthTitle = computed(() => `${MONTHS[view.value.getMonth()]} ${view.value.getFullYear()}`)
const daysInMonth = computed(() => new Date(view.value.getFullYear(), view.value.getMonth() + 1, 0).getDate())
const leading = computed(() => { const fd = new Date(view.value.getFullYear(), view.value.getMonth(), 1).getDay(); return (fd + 6) % 7 })
function shiftMonth(n) { view.value = new Date(view.value.getFullYear(), view.value.getMonth() + n, 1) }
function dateOf(day) { return ymd(new Date(view.value.getFullYear(), view.value.getMonth(), day)) }
function pickDay(day) {
  const d = dateOf(day)
  if (!pendFrom.value || (pendFrom.value && pendTo.value)) { pendFrom.value = d; pendTo.value = '' }
  else { if (d < pendFrom.value) { pendTo.value = pendFrom.value; pendFrom.value = d } else pendTo.value = d }
}
function dayCls(day) {
  const d = dateOf(day)
  const isToday = d === ymd(new Date())
  const from = pendFrom.value, to = pendTo.value
  return {
    'is-from': d === from,
    'is-to': d === to,
    'is-in': from && to && d > from && d < to,
    'is-today': isToday,
  }
}
const selText = computed(() => {
  if (pendFrom.value && pendTo.value) return `${fmtShort(pendFrom.value)} – ${fmtShort(pendTo.value)}`
  if (pendFrom.value) return `${fmtShort(pendFrom.value)} – …`
  return 'Pilih tanggal mulai & akhir'
})
function applyCustom() {
  if (!pendFrom.value || !pendTo.value) return
  const label = pendFrom.value === pendTo.value ? fmtShort(pendFrom.value)
    : `${fmtShort(pendFrom.value)} – ${fmtShort(pendTo.value)}`
  emit('update:modelValue', { from: pendFrom.value, to: pendTo.value, label })
  open.value = false
}

const rangeText = computed(() => {
  const m = props.modelValue
  if (!m?.from) return ''
  if (m.from === m.to) return fmtShort(m.from)
  return `${fmtShort(m.from)} – ${fmtShort(m.to)}`
})
</script>

<style scoped>
.drp { display: inline-block; }
.drp-trigger { display: inline-flex; align-items: center; gap: .5rem; padding: .5rem .85rem; border-radius: .8rem; background: rgba(255,255,255,.9); border: 1px solid rgba(0,0,0,.1); cursor: pointer; font-size: .82rem; color: #0f2d1d; box-shadow: 0 1px 2px rgba(0,0,0,.05); transition: border-color .15s; }
.drp-trigger:hover { border-color: rgba(45,143,86,.4); }
.drp-ico { color: #2d8f56; flex-shrink: 0; }
.drp-label { font-weight: 700; }
.drp-range { color: #6b8a7a; font-size: .75rem; display: none; }
@media (min-width: 480px) { .drp-range { display: inline; } }
.drp-chev { color: #6c8a7a; flex-shrink: 0; }

.drp-backdrop { position: fixed; inset: 0; background: rgba(0,0,0,.4); backdrop-filter: blur(3px); z-index: 80; }
.drp-panel { position: fixed; z-index: 81; left: 50%; top: 50%; transform: translate(-50%,-50%); width: calc(100% - 2rem); max-width: 30rem; max-height: 88vh; overflow-y: auto; background: #fff; border-radius: 1.2rem; box-shadow: 0 24px 70px rgba(0,0,0,.3); }
.drp-head { display: flex; align-items: center; justify-content: space-between; padding: 1rem 1.2rem .6rem; }
.drp-title { font-size: .95rem; font-weight: 800; color: #0f2d1d; }
.drp-close { width: 28px; height: 28px; border: none; background: rgba(0,0,0,.06); border-radius: 50%; font-size: 1.1rem; color: #5a7866; cursor: pointer; }
.drp-body { padding: 0 1.2rem; }

.drp-presets { display: grid; grid-template-columns: repeat(2, 1fr); gap: .5rem; margin-bottom: 1rem; }
@media (min-width: 420px) { .drp-presets { grid-template-columns: repeat(4, 1fr); } }
.drp-preset { padding: .5rem .4rem; border-radius: .65rem; border: 1px solid rgba(0,0,0,.1); background: #f8faf9; font-size: .76rem; font-weight: 600; color: #374151; cursor: pointer; transition: all .12s; }
.drp-preset:hover { background: #eef5f1; border-color: rgba(45,143,86,.35); }
.drp-preset.is-active { background: #2d8f56; color: #fff; border-color: #2d8f56; box-shadow: 0 3px 10px rgba(45,143,86,.3); }

.drp-cal { border-top: 1px solid rgba(0,0,0,.06); padding-top: .8rem; }
.drp-cal-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: .6rem; }
.drp-cal-title { font-size: .88rem; font-weight: 800; color: #14532d; }
.drp-nav { width: 32px; height: 32px; border: 1px solid rgba(0,0,0,.1); background: #fff; border-radius: .55rem; font-size: 1.1rem; color: #2d8f56; cursor: pointer; line-height: 1; }
.drp-nav:hover { background: #f1f5f3; }
.drp-dow { display: grid; grid-template-columns: repeat(7, 1fr); gap: .15rem; margin-bottom: .25rem; }
.drp-dow span { text-align: center; font-size: .65rem; font-weight: 700; color: #9ca3af; text-transform: uppercase; }
.drp-grid { display: grid; grid-template-columns: repeat(7, 1fr); gap: .15rem; }
.drp-blank { aspect-ratio: 1; }
.drp-day { aspect-ratio: 1; border: none; background: transparent; border-radius: .5rem; font-size: .82rem; color: #1a3d2a; cursor: pointer; display: flex; align-items: center; justify-content: center; position: relative; transition: background .1s; }
.drp-day:hover { background: rgba(45,143,86,.12); }
.drp-day.is-in { background: rgba(45,143,86,.12); border-radius: 0; }
.drp-day.is-from, .drp-day.is-to { background: #2d8f56; color: #fff; font-weight: 800; }
.drp-day.is-from { border-radius: .5rem 0 0 .5rem; }
.drp-day.is-to { border-radius: 0 .5rem .5rem 0; }
.drp-day.is-from.is-to { border-radius: .5rem; }
.drp-day.is-today:not(.is-from):not(.is-to) { box-shadow: inset 0 0 0 1.5px rgba(45,143,86,.5); }

.drp-foot { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding: 1rem 1.2rem 1.2rem; margin-top: .4rem; }
.drp-sel { font-size: .78rem; color: #6b7280; min-width: 0; }
.drp-apply { background: #2d8f56; color: #fff; border: none; padding: .6rem 1.3rem; border-radius: .7rem; font-size: .85rem; font-weight: 800; cursor: pointer; box-shadow: 0 4px 12px rgba(45,143,86,.3); flex-shrink: 0; }
.drp-apply:disabled { opacity: .45; cursor: default; box-shadow: none; }

.drp-fade-enter-active, .drp-fade-leave-active { transition: opacity .18s; }
.drp-fade-enter-from, .drp-fade-leave-to { opacity: 0; }
.drp-pop-enter-active, .drp-pop-leave-active { transition: opacity .18s, transform .18s; }
.drp-pop-enter-from, .drp-pop-leave-to { opacity: 0; transform: translate(-50%,-46%) scale(.97); }
</style>
