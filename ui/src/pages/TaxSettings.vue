<template>
  <div class="space-y-5">
    <div class="flex items-center justify-between">
      <h1 class="text-xl font-bold text-gray-900">Pengaturan Pajak</h1>
    </div>

    <AppAlert type="info" message="Pajak diatur per outlet. Aktifkan/nonaktifkan langsung lewat tombol, atau ubah tarif & nama lalu Simpan. Pajak inklusif & berlaku untuk transaksi baru di outlet tersebut." />
    <AppAlert type="error" :message="errorMsg" />

    <AppCard :padding="false">
      <div v-if="loading" class="p-8 text-center text-sm text-gray-400">Memuat…</div>
      <div v-else-if="!rows.length" class="p-8 text-center text-sm text-gray-400">Belum ada outlet.</div>

      <template v-else>
        <!-- Mobile cards -->
        <ul class="sm:hidden divide-y divide-gray-100">
          <li v-for="r in rows" :key="r.outlet_id" class="p-4 space-y-3">
            <div class="flex items-start justify-between gap-2">
              <p class="font-semibold text-gray-900">{{ r.outlet_name }}</p>
              <ToggleBtn :on="r.tax_enabled" :loading="r.toggling" @click="toggle(r)" />
            </div>
            <div class="grid grid-cols-2 gap-2">
              <div>
                <label class="lbl">Tarif (%)</label>
                <input v-model.number="r.tax_rate" type="number" step="0.01" min="0" max="100" class="form-input" />
              </div>
              <div>
                <label class="lbl">Nama Pajak</label>
                <input v-model="r.tax_name" type="text" class="form-input" placeholder="Pajak Restoran (PB1)" />
              </div>
            </div>
            <div class="flex justify-end">
              <AppButton size="sm" :loading="r.saving" :disabled="!dirty(r)" @click="saveRow(r)">Simpan</AppButton>
            </div>
          </li>
        </ul>

        <!-- Desktop table -->
        <div class="hidden sm:block overflow-x-auto">
          <table class="min-w-full text-sm">
            <thead>
              <tr class="border-b border-gray-200 text-left text-gray-500">
                <th class="py-2.5 px-4 font-medium">Outlet</th>
                <th class="py-2.5 px-4 font-medium w-32 text-right">Tarif (%)</th>
                <th class="py-2.5 px-4 font-medium">Nama Pajak</th>
                <th class="py-2.5 px-4 font-medium text-center w-28">Aktif</th>
                <th class="py-2.5 px-4 font-medium text-right w-28"></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="r in rows" :key="r.outlet_id" class="border-b border-gray-50 hover:bg-gray-50">
                <td class="py-2.5 px-4 font-medium text-gray-900">{{ r.outlet_name }}</td>
                <td class="py-2.5 px-4 text-right">
                  <input v-model.number="r.tax_rate" type="number" step="0.01" min="0" max="100" class="form-input text-right w-24 ml-auto" />
                </td>
                <td class="py-2.5 px-4">
                  <input v-model="r.tax_name" type="text" class="form-input" placeholder="Pajak Restoran (PB1)" />
                </td>
                <td class="py-2.5 px-4 text-center">
                  <ToggleBtn :on="r.tax_enabled" :loading="r.toggling" @click="toggle(r)" />
                </td>
                <td class="py-2.5 px-4 text-right">
                  <AppButton size="sm" :loading="r.saving" :disabled="!dirty(r)" @click="saveRow(r)">Simpan</AppButton>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>
    </AppCard>
  </div>
</template>

<script setup>
import { ref, onMounted, h } from 'vue'
import { settingsApi } from '@/api/settings.js'
import { useToastStore } from '@/stores/toast.js'
import AppCard   from '@/components/ui/AppCard.vue'
import AppAlert  from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'

const toast    = useToastStore()
const loading  = ref(true)
const errorMsg = ref('')
const rows     = ref([])

// Toggle switch kecil sebagai komponen inline (render function, tanpa file terpisah).
const ToggleBtn = (props, { emit }) =>
  h('button', {
    type: 'button',
    disabled: props.loading,
    onClick: () => emit('click'),
    class: ['relative inline-flex h-6 w-11 flex-shrink-0 rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none',
      props.loading ? 'opacity-60 cursor-wait' : 'cursor-pointer',
      props.on ? 'bg-emerald-600' : 'bg-gray-300'],
  }, [
    h('span', { class: ['pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow transition duration-200 ease-in-out',
      props.on ? 'translate-x-5' : 'translate-x-0'] }),
  ])
ToggleBtn.props = ['on', 'loading']
ToggleBtn.emits = ['click']

function snapshot(r) { return `${r.tax_rate}|${r.tax_name}` }
function dirty(r) { return r._orig !== snapshot(r) }

async function load() {
  loading.value = true; errorMsg.value = ''
  try {
    const d = await settingsApi.listOutletTax()
    const list = d?.data ?? d ?? []
    rows.value = list.map(o => ({
      ...o,
      tax_name: o.tax_name || 'Pajak Restoran (PB1)',
      saving: false,
      toggling: false,
      _orig: `${o.tax_rate}|${o.tax_name || 'Pajak Restoran (PB1)'}`,
    }))
  } catch (e) {
    errorMsg.value = e?.message || 'Gagal memuat daftar pajak outlet'
  } finally {
    loading.value = false
  }
}

async function persist(r) {
  return settingsApi.updateOutletTax(r.outlet_id, {
    tax_enabled: r.tax_enabled,
    tax_rate:    r.tax_rate,
    tax_name:    r.tax_name,
  })
}

// Toggle Aktif → langsung simpan.
async function toggle(r) {
  const next = !r.tax_enabled
  r.toggling = true
  try {
    r.tax_enabled = next
    await persist(r)
    toast.success(`Pajak ${r.outlet_name} ${next ? 'diaktifkan' : 'dinonaktifkan'}`)
  } catch (e) {
    r.tax_enabled = !next // rollback
    toast.error(e?.response?.data?.error || 'Gagal mengubah status pajak')
  } finally {
    r.toggling = false
  }
}

// Simpan perubahan tarif / nama.
async function saveRow(r) {
  if (r.tax_rate < 0 || r.tax_rate > 100) {
    toast.error('Tarif pajak harus 0-100%')
    return
  }
  r.saving = true
  try {
    await persist(r)
    r._orig = snapshot(r)
    toast.success(`Pajak ${r.outlet_name} disimpan`)
  } catch (e) {
    toast.error(e?.response?.data?.error || 'Gagal menyimpan')
  } finally {
    r.saving = false
  }
}

onMounted(load)
</script>

<style scoped>
.form-input { width: 100%; padding: .4rem .6rem; border-radius: .5rem; font-size: .85rem; border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none; }
.form-input:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }
.lbl { display: block; font-size: .68rem; font-weight: 600; color: #6b7280; margin-bottom: .15rem; }
</style>
