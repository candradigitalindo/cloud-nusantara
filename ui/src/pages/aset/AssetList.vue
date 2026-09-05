<template>
  <div class="space-y-5">
    <!-- Header -->
    <div class="flex items-start justify-between flex-wrap gap-3">
      <div>
        <h1 class="text-xl font-bold text-gray-900">Daftar Aset</h1>
        <p class="text-sm text-gray-500 mt-0.5">Register aset per outlet — identitas, kondisi, nilai, dan jadwal perawatan.</p>
      </div>
      <AppButton v-if="canCreate" @click="openCreate">
        <span class="btn-ic" v-html="ICONS.plus"></span>Tambah Aset
      </AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Aset yang dihapus (is_deleted) tidak masuk perhitungan ringkasan mana
         pun — tampilkan itu secara eksplisit alih-alih kartu KPI yang aktif-saja
         supaya tidak terbaca seolah aset terhapus ikut terhitung. -->
    <AppAlert v-if="isTrashView" type="info" message="Menampilkan aset yang telah dihapus. Aset ini tidak muncul di laporan, dashboard, atau perhitungan nilai aset mana pun sampai dipulihkan." />

    <!-- Ringkasan — angka jadwal perawatan bisa diklik untuk menyaring daftar -->
    <div v-if="summary && !isTrashView" class="kpi-grid">
      <div class="kpi">
        <div class="kpi-label">Total Aset</div>
        <div class="kpi-val">{{ summary.total_assets }}</div>
        <div class="kpi-sub">{{ summary.total_quantity }} unit · {{ formatRupiah(summary.total_value) }} nilai perolehan</div>
      </div>

      <button type="button" class="kpi kpi--btn" :class="summary.overdue > 0 ? 'kpi--alert' : 'kpi--good'" @click="toggleDue('overdue')">
        <div class="kpi-label">Perawatan Terlambat<span v-if="filterDue === 'overdue'" class="kpi-on">disaring</span></div>
        <div class="kpi-val">{{ summary.overdue }}</div>
        <div class="kpi-sub">{{ summary.overdue > 0 ? 'lewat dari jadwal berikutnya' : 'tidak ada yang lewat jadwal' }}</div>
      </button>

      <button type="button" class="kpi kpi--btn" :class="summary.due_soon > 0 ? 'kpi--warn' : 'kpi--good'" @click="toggleDue('due_soon')">
        <div class="kpi-label">Jatuh Tempo ≤{{ summary.due_soon_days }} Hari<span v-if="filterDue === 'due_soon'" class="kpi-on">disaring</span></div>
        <div class="kpi-val">{{ summary.due_soon }}</div>
        <div class="kpi-sub">{{ summary.scheduled }} terjadwal lebih jauh · {{ summary.unscheduled }} belum dijadwalkan</div>
      </button>

      <div class="kpi" :class="summary.needs_attention > 0 ? 'kpi--warn' : 'kpi--good'">
        <div class="kpi-label">Perlu Perhatian</div>
        <div class="kpi-val">{{ summary.needs_attention }}</div>
        <div class="kpi-sub">{{ summary.needs_attention > 0 ? 'kondisi selain baik' : 'semua aset berkondisi baik' }}</div>
      </div>
    </div>

    <!-- Filters -->
    <AppCard>
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-3">
        <SearchSelect v-model="filterOutlet" :options="outletFilterOptions" placeholder="Semua outlet" searchPlaceholder="Cari outlet…" @change="loadAll" />
        <select v-model="filterStatus" @change="load" class="form-input" :class="{ 'form-input--trash': isTrashView }">
          <option value="">Aktif</option>
          <option value="terhapus">Terhapus</option>
        </select>
        <select v-model="filterCondition" @change="load" class="form-input">
          <option value="">Semua kondisi</option>
          <option v-for="(lbl, key) in CONDITIONS" :key="key" :value="key">{{ lbl }}</option>
        </select>
        <select v-model="filterDue" @change="load" class="form-input">
          <option value="">Semua jadwal perawatan</option>
          <option v-for="(lbl, key) in DUE_FILTERS" :key="key" :value="key">{{ lbl }}</option>
        </select>
        <input v-model="search" @input="debouncedLoad" type="search" placeholder="Cari nama / kode / kategori…" class="form-input" />
      </div>
      <div v-if="hasActiveFilters" class="filter-summary">
        <span>{{ assets.length }} aset ditemukan</span>
        <button type="button" class="lnk" @click="resetFilters">Bersihkan filter</button>
      </div>
    </AppCard>

    <!-- List -->
    <AppCard :padding="false">
      <!-- Mobile cards -->
      <div class="sm:hidden">
        <div v-if="loading" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
        <div v-else-if="!assets.length" class="empty-block">
          <span class="empty-ic" v-html="ICONS.trash"></span>
          <p class="empty-title">{{ emptyTitle }}</p>
          <p class="empty-desc">{{ emptyDesc }}</p>
          <button v-if="isTrashView" type="button" class="lnk mt-1" @click="filterStatus = ''; load()">Kembali ke aset aktif</button>
        </div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="a in assets" :key="a.id" class="p-4 space-y-2">
            <div class="flex items-start justify-between gap-2">
              <div class="min-w-0">
                <p class="font-semibold text-gray-900 break-words">{{ a.name }}</p>
                <p class="text-xs text-gray-500 mt-0.5">
                  <span v-if="a.code" class="font-mono">{{ a.code }}</span>
                  <span v-if="a.category"> · {{ a.category }}</span>
                  · {{ a.quantity }} {{ a.unit }}
                </p>
              </div>
              <span class="cond-badge shrink-0" :class="condCls(a.condition)">{{ CONDITIONS[a.condition] || a.condition }}</span>
            </div>
            <p class="text-xs text-gray-500">{{ a.outlet_name }}<span v-if="a.location"> · {{ a.location }}</span></p>
            <template v-if="!isTrashView">
              <p class="text-xs text-gray-500">Perawatan: {{ a.maintenance_count }}× · Terakhir: {{ a.last_maintenance ? formatDateStr(a.last_maintenance) : '—' }}</p>
              <p class="text-xs text-gray-500 flex items-center gap-1.5 flex-wrap">
                <span>Berikutnya: {{ a.next_due_date ? formatDateStr(a.next_due_date) : '—' }}</span>
                <span v-if="a.due_status !== 'none'" class="due-badge" :class="dueCls(a.due_status)">{{ dueText(a) }}</span>
                <span v-else class="due-badge due-none">Belum dijadwalkan</span>
              </p>
            </template>
            <div class="flex gap-2 pt-1">
              <template v-if="isTrashView">
                <button v-if="canDelete" @click="confirmRestoreAsset(a)" class="pill-btn pill-btn--restore">
                  <span class="btn-ic" v-html="ICONS.undo"></span>Pulihkan
                </button>
              </template>
              <template v-else>
                <button @click="openHistory(a)" class="pill-btn pill-btn--history">
                  <span class="btn-ic" v-html="ICONS.history"></span>Riwayat
                </button>
                <button v-if="canUpdate" @click="openEdit(a)" class="pill-btn pill-btn--edit">
                  <span class="btn-ic" v-html="ICONS.edit"></span>Edit
                </button>
                <button v-if="canDelete" @click="confirmDelete(a)" class="pill-btn pill-btn--delete">
                  <span class="btn-ic" v-html="ICONS.trash"></span>Hapus
                </button>
              </template>
            </div>
          </li>
        </ul>
      </div>

      <!-- Desktop table -->
      <!-- v-if di sini, bukan hanya emptyText — AppTable selalu merender baris
           "Tidak ada data" bawaannya sendiri saat rows kosong, jadi tanpa v-if
           blok kosong kustom di bawah akan tampil dobel dengan baris bawaan itu. -->
      <AppTable v-if="loading || assets.length" class="hidden sm:block" :columns="COLUMNS" :rows="assets" :loading="loading">
        <template #cell-name="{ row }">
          <div>
            <p class="font-medium text-gray-900">{{ row.name }}</p>
            <p v-if="row.code" class="text-xs text-gray-400 font-mono">{{ row.code }}</p>
          </div>
        </template>
        <template #cell-quantity="{ row }">{{ row.quantity }} {{ row.unit }}</template>
        <template #cell-condition="{ row }">
          <span class="cond-badge" :class="condCls(row.condition)">{{ CONDITIONS[row.condition] || row.condition }}</span>
        </template>
        <template #cell-maintenance="{ row }">
          <span class="text-sm">{{ row.maintenance_count }}×</span>
          <span class="text-xs text-gray-400 block">{{ row.last_maintenance ? formatDateStr(row.last_maintenance) : 'belum ada' }}</span>
        </template>
        <template #cell-due="{ row }">
          <template v-if="row.due_status !== 'none'">
            <span class="due-badge" :class="dueCls(row.due_status)">{{ dueText(row) }}</span>
            <span class="text-xs text-gray-400 block mt-0.5">{{ formatDateStr(row.next_due_date) }}</span>
          </template>
          <span v-else class="due-badge due-none">Belum dijadwalkan</span>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex items-center gap-1 justify-end">
            <template v-if="isTrashView">
              <button v-if="canDelete" class="icon-btn icon-btn--restore" title="Pulihkan aset" aria-label="Pulihkan aset" @click="confirmRestoreAsset(row)">
                <span v-html="ICONS.undo"></span>
              </button>
            </template>
            <template v-else>
              <button class="icon-btn icon-btn--history" title="Lihat riwayat perawatan" aria-label="Riwayat perawatan" @click="openHistory(row)">
                <span v-html="ICONS.history"></span>
              </button>
              <button v-if="canUpdate" class="icon-btn icon-btn--edit" title="Ubah data aset" aria-label="Edit aset" @click="openEdit(row)">
                <span v-html="ICONS.edit"></span>
              </button>
              <button v-if="canDelete" class="icon-btn icon-btn--delete" title="Hapus aset" aria-label="Hapus aset" @click="confirmDelete(row)">
                <span v-html="ICONS.trash"></span>
              </button>
            </template>
          </div>
        </template>
      </AppTable>

      <!-- Empty state (desktop) -->
      <div v-if="!loading && !assets.length" class="hidden sm:block empty-block">
        <span class="empty-ic" v-html="ICONS.trash"></span>
        <p class="empty-title">{{ emptyTitle }}</p>
        <p class="empty-desc">{{ emptyDesc }}</p>
        <button v-if="isTrashView" type="button" class="lnk mt-1" @click="filterStatus = ''; load()">Kembali ke aset aktif</button>
        <button v-else-if="hasActiveFilters" type="button" class="lnk mt-1" @click="resetFilters">Bersihkan filter</button>
        <AppButton v-else-if="canCreate" class="mt-1" size="sm" @click="openCreate">
          <span class="btn-ic" v-html="ICONS.plus"></span>Tambah aset pertama
        </AppButton>
      </div>
    </AppCard>

    <!-- ══════════════════ Modal: Tambah / Edit Aset ══════════════════ -->
    <!-- Dua kolom berdampingan (bukan bertumpuk) — supaya modal yang besar ini
         tetap muat dalam satu layar tanpa perlu menggulir. Di layar sempit
         (<md) kolom kembali bertumpuk otomatis lewat grid-cols-1. -->
    <AppModal v-model="assetModal" :title="editing ? `Edit Aset — ${editing.name}` : 'Tambah Aset Baru'" size="2xl">
      <form class="grid md:grid-cols-2 gap-x-8 gap-y-5" @submit.prevent="saveAsset">
        <div class="space-y-5">
          <!-- Identitas -->
          <section class="f-section">
            <div class="f-section-hd">
              <span class="f-section-ic f-section-ic--indigo" v-html="ICONS.tag"></span>
              <div>
                <h3 class="f-section-title">Identitas Aset</h3>
                <p class="f-section-desc">Nama, kode, dan outlet pemilik aset ini.</p>
              </div>
            </div>
            <div class="grid sm:grid-cols-2 gap-3">
              <div v-if="!editing" class="sm:col-span-2">
                <label class="lbl">Outlet <span class="req">*</span></label>
                <SearchSelect v-model="form.outlet_id" :options="outlets" placeholder="Pilih outlet…" searchPlaceholder="Cari outlet…" />
              </div>
              <div class="sm:col-span-2">
                <label class="lbl">Nama Aset <span class="req">*</span></label>
                <input v-model="form.name" class="form-input" placeholder="Contoh: Meja Kayu Jati" required />
                <!-- Peringatan non-blokir — nama sama tidak dilarang (kondisi/lokasi
                     boleh beda per baris), tapi kalau ini sebenarnya unit baru untuk
                     barang yang SAMA PERSIS, sebaiknya tambah lewat Perolehan supaya
                     penyusutannya tetap dihitung per batch, bukan baris baru. -->
                <div v-if="duplicateSummary" class="dup-warning">
                  <span class="dup-warning-ic" v-html="ICONS.alertTriangle"></span>
                  <div class="text-xs">
                    <p class="text-gray-800">
                      Sudah ada <strong>"{{ form.name.trim() }}"</strong> di outlet ini —
                      <span v-for="(m, i) in duplicateSummary.items" :key="m.id">{{ i > 0 ? ', ' : '' }}{{ m.quantity }} {{ m.unit }} ({{ (CONDITIONS[m.condition] || m.condition).toLowerCase() }})</span>,
                      total {{ duplicateSummary.totalQty }} unit.
                    </p>
                    <p class="text-gray-500 mt-0.5">
                      Kalau ini unit baru untuk barang yang sama persis, sebaiknya tambah lewat
                      <router-link to="/aset/perolehan" class="lnk" target="_blank">Histori Perolehan</router-link>
                      pada baris yang sudah ada — bukan baris baru — supaya penyusutannya tetap dihitung per batch.
                    </p>
                  </div>
                </div>
              </div>
              <div>
                <label class="lbl">Kode / Tag</label>
                <input v-model="form.code" class="form-input" placeholder="mis. MJ-001" />
              </div>
              <div>
                <label class="lbl">Kategori</label>
                <input v-model="form.category" class="form-input" placeholder="mis. Furniture" list="asset-cats" />
                <datalist id="asset-cats">
                  <option v-for="c in categorySuggestions" :key="c" :value="c" />
                </datalist>
              </div>
              <div>
                <label class="lbl">Nomor Seri</label>
                <input v-model="form.serial_number" class="form-input" placeholder="Opsional" />
              </div>
              <div class="grid grid-cols-2 gap-3">
                <div>
                  <label class="lbl">Jumlah</label>
                  <input v-model.number="form.quantity" type="number" min="1" class="form-input" />
                </div>
                <div>
                  <label class="lbl">Satuan</label>
                  <input v-model="form.unit" class="form-input" placeholder="unit" />
                </div>
              </div>
            </div>
          </section>

          <!-- Kondisi & Lokasi -->
          <section class="f-section">
            <div class="f-section-hd">
              <span class="f-section-ic f-section-ic--sky" v-html="ICONS.pin"></span>
              <div>
                <h3 class="f-section-title">Kondisi &amp; Lokasi</h3>
                <p class="f-section-desc">Keadaan fisik dan tempat aset ini berada.</p>
              </div>
            </div>
            <div class="grid sm:grid-cols-2 gap-3">
              <div>
                <label class="lbl">Kondisi</label>
                <select v-model="form.condition" class="form-input">
                  <option v-for="(lbl, key) in CONDITIONS" :key="key" :value="key">{{ lbl }}</option>
                </select>
              </div>
              <div>
                <label class="lbl">Lokasi / Ruang</label>
                <input v-model="form.location" class="form-input" placeholder="mis. Lantai 1 – Area Indoor" />
              </div>
            </div>
          </section>
        </div>

        <div class="space-y-5">
          <!-- Nilai & Penyusutan -->
          <section class="f-section">
            <div class="f-section-hd">
              <span class="f-section-ic f-section-ic--emerald" v-html="ICONS.banknote"></span>
              <div>
                <h3 class="f-section-title">Nilai &amp; Penyusutan</h3>
                <p class="f-section-desc">Harga perolehan dan umur ekonomis untuk hitung nilai buku otomatis.</p>
              </div>
            </div>
            <div class="grid sm:grid-cols-2 gap-3">
              <div>
                <label class="lbl">Tgl Pembelian</label>
                <input v-model="form.purchase_date" type="date" class="form-input" />
              </div>
              <div>
                <label class="lbl">Harga Beli <span class="hint">per unit</span></label>
                <RupiahInput v-model="form.purchase_price" placeholder="0" />
              </div>
              <div>
                <label class="lbl">Umur Ekonomis <span class="hint">bulan</span></label>
                <input v-model.number="form.useful_life_months" type="number" min="0" class="form-input" placeholder="0 = tidak disusutkan" />
              </div>
              <div>
                <label class="lbl">Nilai Residu</label>
                <RupiahInput v-model="form.residual_value" placeholder="0" />
              </div>
            </div>

            <!-- Pratinjau penyusutan — hanya tampil bila umur ekonomis diisi -->
            <div v-if="depreciationPreview" class="preview-box">
              <span class="preview-ic" v-html="ICONS.trendDown"></span>
              <div class="text-sm">
                <p class="text-gray-800">
                  Estimasi susut <strong>{{ formatRupiah(depreciationPreview.monthly) }}/bulan</strong>
                  dari total perolehan <strong>{{ formatRupiah(depreciationPreview.total) }}</strong>.
                </p>
                <p class="text-xs text-gray-500 mt-0.5">
                  Nilai buku turun ke {{ formatRupiah(form.residual_value || 0) }} setelah {{ form.useful_life_months }} bulan.
                </p>
              </div>
            </div>
            <p v-else class="hint-text">Isi umur ekonomis untuk menyusutkan nilai aset secara otomatis setiap bulan.</p>

            <p v-if="!editing" class="hint-text hint-text--muted">
              <span class="btn-ic btn-ic--inline" v-html="ICONS.info"></span>
              Perolehan pertama akan dicatat otomatis dari tanggal &amp; harga beli di atas.
            </p>
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
            <textarea v-model="form.notes" rows="2" class="form-input" placeholder="mis. Dibeli untuk area VIP lantai 2"></textarea>
          </section>
        </div>
      </form>

      <template #footer>
        <button type="button" class="btn-ghost" @click="assetModal = false">Batal</button>
        <AppButton :loading="saving" @click="saveAsset">
          {{ editing ? 'Simpan Perubahan' : 'Tambah Aset' }}
        </AppButton>
      </template>
    </AppModal>

    <!-- ══════════════════ Modal: Riwayat Perawatan ══════════════════ -->
    <AppModal v-model="historyModal" :title="`Riwayat Perawatan — ${activeAsset?.name || ''}`" size="xl">
      <div v-if="activeAsset" class="space-y-4">
        <div class="flex items-center justify-between flex-wrap gap-2 text-xs text-gray-500 bg-gray-50 rounded-lg px-3 py-2">
          <span>{{ activeAsset.outlet_name }}<span v-if="activeAsset.location"> · {{ activeAsset.location }}</span></span>
          <span class="cond-badge" :class="condCls(activeAsset.condition)">{{ CONDITIONS[activeAsset.condition] || activeAsset.condition }}</span>
        </div>

        <!-- Add maintenance -->
        <details v-if="canUpdate" class="rounded-lg border border-gray-200" :open="!history.length">
          <summary class="cursor-pointer select-none px-3 py-2.5 text-sm font-medium text-emerald-700 bg-emerald-50 rounded-lg flex items-center gap-2">
            <span class="btn-ic" v-html="ICONS.wrench"></span>Catat Perawatan Baru
          </summary>
          <form class="p-3 space-y-3 border-t border-gray-100" @submit.prevent="saveMaintenance">
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
              <textarea v-model="mForm.description" rows="2" class="form-input" placeholder="Pekerjaan yang dilakukan" required></textarea>
            </div>
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
            <div class="flex justify-end">
              <AppButton type="submit" :loading="savingM">Simpan Perawatan</AppButton>
            </div>
          </form>
        </details>

        <!-- Timeline -->
        <div v-if="loadingHistory" class="text-center text-sm text-gray-400 py-4">Memuat histori…</div>
        <div v-else-if="!history.length" class="empty-block empty-block--inline">
          <span class="empty-ic" v-html="ICONS.history"></span>
          <p class="empty-title">Belum ada catatan perawatan</p>
          <p class="empty-desc">Catatan yang ditambahkan akan tersusun sebagai linimasa di sini.</p>
        </div>
        <ol v-else class="space-y-3">
          <li v-for="m in history" :key="m.id" class="timeline-item">
            <span class="timeline-dot" :class="`timeline-dot--${m.type}`" v-html="maintIcon(m.type)"></span>
            <div class="flex items-start justify-between gap-2">
              <div class="min-w-0">
                <div class="flex items-center gap-2 flex-wrap">
                  <span class="text-sm font-semibold text-gray-900">{{ formatDateStr(m.maintenance_date) }}</span>
                  <span class="mtype-badge" :class="`mtype-badge--${m.type}`">{{ MTYPES[m.type] || m.type }}</span>
                </div>
                <p class="text-sm text-gray-700 mt-0.5 break-words">{{ m.description }}</p>
                <p class="text-xs text-gray-500 mt-1 space-x-2">
                  <span v-if="m.cost > 0">Biaya: {{ formatRupiah(m.cost) }}</span>
                  <span v-if="m.performed_by">Oleh: {{ m.performed_by }}</span>
                  <span v-if="m.condition_after">→ {{ CONDITIONS[m.condition_after] || m.condition_after }}</span>
                  <span v-if="m.next_due_date">Berikutnya: {{ formatDateStr(m.next_due_date) }}</span>
                </p>
              </div>
              <button v-if="canUpdate" @click="deleteMaintenance(m)" title="Hapus catatan" aria-label="Hapus catatan perawatan" class="icon-btn icon-btn--delete icon-btn--sm shrink-0">
                <span v-html="ICONS.trash"></span>
              </button>
            </div>
          </li>
        </ol>
      </div>
    </AppModal>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import Swal from 'sweetalert2'
import { assetsApi } from '@/api/assets.js'
import { outletsApi } from '@/api/outlets.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth.js'
import { formatRupiah, formatDateStr, todayDateString } from '@/utils/format.js'
import { useRealtime } from '@/utils/realtime.js'
import AppCard   from '@/components/ui/AppCard.vue'
import AppTable  from '@/components/ui/AppTable.vue'
import AppAlert  from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppModal  from '@/components/ui/AppModal.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'
import RupiahInput from '@/components/ui/RupiahInput.vue'

const toast = useToastStore()
const auth = useAuthStore()
const canCreate = auth.hasPermission('assets.create')
const canUpdate = auth.hasPermission('assets.update')
const canDelete = auth.hasPermission('assets.delete')

const CONDITIONS = { baik: 'Baik', rusak_ringan: 'Rusak Ringan', rusak_berat: 'Rusak Berat', perbaikan: 'Dalam Perbaikan' }
const MTYPES = { rutin: 'Rutin', perbaikan: 'Perbaikan', penggantian: 'Penggantian Part', inspeksi: 'Inspeksi' }
const DUE_FILTERS = {
  overdue: 'Terlambat',
  due_soon: 'Jatuh tempo ≤7 hari',
  scheduled: 'Terjadwal (>7 hari)',
  none: 'Belum dijadwalkan',
}

// ── Ikon aksi — inline SVG, stroke-based, mengikuti bahasa visual sidebar ──
// Ukuran diatur lewat CSS pembungkus (.icon-btn / .btn-ic / dst.) dengan
// selector :deep(svg), karena konten v-html tidak ikut ter-scope oleh Vue.
const ICONS = {
  plus: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4"/></svg>',
  history: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M3 3v5h5"/><path stroke-linecap="round" stroke-linejoin="round" d="M3.05 13a9 9 0 106.02-9.36"/><path stroke-linecap="round" stroke-linejoin="round" d="M12 7v5l3.5 2"/></svg>',
  edit: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5"/><path stroke-linecap="round" stroke-linejoin="round" d="M18.5 2.5a2.12 2.12 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>',
  trash: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>',
  tag: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><path stroke-linecap="round" stroke-linejoin="round" d="M11.5 3H6a2 2 0 00-2 2v5.5a2 2 0 00.586 1.414l8.5 8.5a2 2 0 002.828 0l5.5-5.5a2 2 0 000-2.828l-8.5-8.5A2 2 0 0011.5 3z"/><circle cx="7.5" cy="7.5" r="1.1" fill="currentColor" stroke="none"/></svg>',
  pin: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><path stroke-linecap="round" stroke-linejoin="round" d="M12 21c-4.5-4.5-7-8.14-7-11a7 7 0 1114 0c0 2.86-2.5 6.5-7 11z"/><circle cx="12" cy="10" r="2.4"/></svg>',
  banknote: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><rect x="2.5" y="6" width="19" height="12" rx="2"/><circle cx="12" cy="12" r="2.3"/><path stroke-linecap="round" d="M5.5 9.5h.01M18.5 14.5h.01"/></svg>',
  doc: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><path stroke-linecap="round" stroke-linejoin="round" d="M9 2h6a1 1 0 011 1v1h2a2 2 0 012 2v13a2 2 0 01-2 2H6a2 2 0 01-2-2V6a2 2 0 012-2h2V3a1 1 0 011-1z"/><line x1="8" y1="11" x2="16" y2="11"/><line x1="8" y1="15" x2="13" y2="15"/></svg>',
  trendDown: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M3 6l7 7 4-4 7 7"/><path stroke-linecap="round" stroke-linejoin="round" d="M15 16h6v-6"/></svg>',
  info: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="9"/><path stroke-linecap="round" d="M12 11v5"/><path stroke-linecap="round" d="M12 8h.01"/></svg>',
  wrench: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M14.7 6.3a1 1 0 000 1.4l1.6 1.6a1 1 0 001.4 0l3.35-3.35a6 6 0 01-7.94 7.94l-6.7 6.7a2.12 2.12 0 01-3-3l6.7-6.7a6 6 0 017.94-7.94L14.7 6.3z"/></svg>',
  refresh: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg>',
  swap: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M8 7h12m0 0l-4-4m4 4l-4 4M16 17H4m0 0l4 4m-4-4l4-4"/></svg>',
  checkCircle: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>',
  undo: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M3 10h10a5 5 0 010 10h-2"/><path stroke-linecap="round" stroke-linejoin="round" d="M7 5L3 10l4 5"/></svg>',
  alertTriangle: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v4m0 4h.01M10.29 3.86l-8.18 14.18A1.5 1.5 0 003.5 20.5h17a1.5 1.5 0 001.39-2.46L13.71 3.86a1.5 1.5 0 00-2.42 0z"/></svg>',
}
// Ikon per jenis perawatan pada linimasa — memberi isyarat visual sekilas
// tanpa perlu membaca label teksnya.
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
function dueCls(s) {
  return { 'due-overdue': s === 'overdue', 'due-soon': s === 'due_soon', 'due-scheduled': s === 'scheduled' }
}
// Jarak hari dihitung server (zona waktu server), supaya tidak ikut jam browser.
function dueText(a) {
  const d = a.due_in_days
  if (d < 0) return `Terlambat ${Math.abs(d)} hari`
  if (d === 0) return 'Jatuh tempo hari ini'
  return `${d} hari lagi`
}

// Dialog konfirmasi bermerek — dipakai untuk aksi hapus supaya konsekuensinya
// dijelaskan, bukan cuma "Yakin?" bawaan browser. Warna mengikuti konvensi
// yang sudah dipakai di halaman lain (mis. Vendors.vue).
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
  { key: 'name',        label: 'Aset' },
  { key: 'category',    label: 'Kategori' },
  { key: 'outlet_name', label: 'Outlet' },
  { key: 'quantity',    label: 'Jumlah' },
  { key: 'condition',   label: 'Kondisi' },
  { key: 'location',    label: 'Lokasi' },
  { key: 'maintenance', label: 'Perawatan' },
  { key: 'due',         label: 'Jadwal Berikutnya' },
  { key: 'actions',     label: '' },
]

const assets = ref([])
const outlets = ref([])
const summary = ref(null)
const loading = ref(false)
const errorMsg = ref('')
const filterOutlet = ref('')
const filterCondition = ref('')
// Dashboard menautkan ke sini dengan ?due=overdue / due_soon.
const route = useRoute()
const filterDue = ref(DUE_FILTERS[route.query.due] ? String(route.query.due) : '')
const search = ref('')
// '' = aset aktif (bawaan), 'terhapus' = lihat aset yang dihapus (is_deleted)
// untuk dipulihkan. Sumbu terpisah dari kondisi/jadwal/pencarian, makanya
// tidak ikut hasActiveFilters — resetFilters tetap mengembalikannya ke aktif.
const filterStatus = ref('')

const outletFilterOptions = computed(() => [{ id: '', name: 'Semua outlet' }, ...outlets.value])
const categorySuggestions = computed(() => [...new Set(assets.value.map(a => a.category).filter(Boolean))])
const hasActiveFilters = computed(() =>
  !!(filterOutlet.value || filterCondition.value || filterDue.value || search.value.trim()))
const isTrashView = computed(() => filterStatus.value === 'terhapus')

const emptyTitle = computed(() => {
  if (isTrashView.value) return 'Tidak ada aset yang dihapus'
  return hasActiveFilters.value ? 'Tidak ada aset yang cocok dengan filter' : 'Belum ada aset tercatat'
})
const emptyDesc = computed(() => {
  if (isTrashView.value) return 'Aset yang dihapus dari Daftar Aset akan muncul di sini untuk dipulihkan.'
  return hasActiveFilters.value
    ? 'Coba ubah kondisi, jadwal, atau kata kunci pencarian.'
    : 'Aset yang ditambahkan akan muncul di sini lengkap dengan nilai dan jadwal perawatannya.'
})

function resetFilters() {
  filterOutlet.value = ''; filterCondition.value = ''; filterDue.value = ''; search.value = ''; filterStatus.value = ''
  loadAll()
}

function asArray(d) { return Array.isArray(d) ? d : (d?.data || []) }

async function load() {
  loading.value = true; errorMsg.value = ''
  try {
    const data = await assetsApi.list({
      outlet_id: filterOutlet.value || undefined,
      condition: filterCondition.value || undefined,
      due: filterDue.value || undefined,
      search: search.value.trim() || undefined,
      status: filterStatus.value || undefined,
    })
    assets.value = asArray(data)
  } catch (e) {
    errorMsg.value = e?.message || 'Gagal memuat perlengkapan'
  } finally {
    loading.value = false
  }
}
let _t = null
function debouncedLoad() { clearTimeout(_t); _t = setTimeout(load, 350) }

// Ringkasan hanya mengikuti outlet — bukan filter kondisi/jadwal/pencarian,
// supaya kartu KPI tetap menjadi acuan tetap saat daftar sedang disaring.
async function loadSummary() {
  try {
    summary.value = await assetsApi.summary({ outlet_id: filterOutlet.value || undefined })
  } catch { summary.value = null }
}
async function loadAll() { await Promise.all([load(), loadSummary()]) }

function toggleDue(status) {
  filterDue.value = filterDue.value === status ? '' : status
  load()
}

// Jadwal berpindah status di pergantian hari — server menyiarkan asset_alert
// setelah evaluasi harian, halaman menyegar sendiri tanpa perlu di-refresh.
useRealtime(['asset_alert'], loadAll)

async function loadOutlets() {
  try {
    const d = await outletsApi.myOutlets()
    outlets.value = d?.outlets ?? d ?? []
  } catch { outlets.value = [] }
}

// ── Asset CRUD ──
const assetModal = ref(false)
const editing = ref(null)
const saving = ref(false)
const form = ref({})
function blankForm() {
  return {
    outlet_id: filterOutlet.value || '', code: '', name: '', category: '', quantity: 1, unit: 'unit',
    condition: 'baik', location: '', purchase_date: todayDateString(), purchase_price: 0, notes: '',
    serial_number: '', useful_life_months: 0, residual_value: 0,
  }
}
function openCreate() { editing.value = null; form.value = blankForm(); duplicateMatches.value = []; assetModal.value = true }
function openEdit(a) {
  editing.value = a
  form.value = {
    outlet_id: a.outlet_id, code: a.code, name: a.name, category: a.category, quantity: a.quantity,
    unit: a.unit, condition: a.condition, location: a.location, purchase_date: a.purchase_date || '',
    purchase_price: a.purchase_price, notes: a.notes, serial_number: a.serial_number || '',
    useful_life_months: a.useful_life_months || 0, residual_value: a.residual_value || 0,
  }
  duplicateMatches.value = []
  assetModal.value = true
}

// ── Peringatan nama duplikat (create saja) ──────────────────
// Non-blokir: nama sama TIDAK dilarang (kondisi/lokasi boleh beda per baris),
// ini cuma mengingatkan supaya unit baru dari barang yang sama persis tidak
// tercecer jadi baris baru alih-alih ditambahkan lewat Perolehan — yang mana
// penyusutannya dihitung per batch (lihat services/asset.go).
const duplicateMatches = ref([])
const duplicateSummary = computed(() => {
  if (!duplicateMatches.value.length) return null
  return {
    items: duplicateMatches.value,
    totalQty: duplicateMatches.value.reduce((s, a) => s + Number(a.quantity || 0), 0),
  }
})
let _dupTimer = null
async function checkDuplicateName() {
  if (editing.value) { duplicateMatches.value = []; return }
  const name = (form.value.name || '').trim()
  const outletId = form.value.outlet_id
  if (name.length < 2 || !outletId) { duplicateMatches.value = []; return }
  try {
    const list = asArray(await assetsApi.list({ outlet_id: outletId, search: name }))
    duplicateMatches.value = list.filter(a => a.name.trim().toLowerCase() === name.toLowerCase())
  } catch { duplicateMatches.value = [] }
}
watch(() => [form.value.name, form.value.outlet_id], () => {
  clearTimeout(_dupTimer)
  _dupTimer = setTimeout(checkDuplicateName, 450)
})

// Pratinjau penyusutan garis lurus — dihitung sama seperti backend
// (services/asset.go: assetMonthlyDeprecExpr), murni untuk gambaran di form.
const depreciationPreview = computed(() => {
  const months = Number(form.value.useful_life_months) || 0
  if (months <= 0) return null
  const qty = Number(form.value.quantity) || 1
  const total = (Number(form.value.purchase_price) || 0) * qty
  const residual = Number(form.value.residual_value) || 0
  const depreciable = Math.max(total - residual, 0)
  if (depreciable <= 0) return null
  return { monthly: depreciable / months, total }
})

async function saveAsset() {
  if (!form.value.name?.trim()) { toast.error('Nama aset wajib diisi'); return }
  if (!editing.value && !form.value.outlet_id) { toast.error('Pilih outlet'); return }
  saving.value = true
  try {
    if (editing.value) await assetsApi.update(editing.value.id, form.value)
    else await assetsApi.create(form.value)
    toast.success(editing.value ? 'Aset diperbarui' : 'Aset ditambahkan')
    assetModal.value = false
    await loadAll()
  } catch (e) { toast.error(e?.message || 'Gagal menyimpan') } finally { saving.value = false }
}

async function confirmDelete(a) {
  const r = await swalBase({
    icon: 'warning',
    title: `Hapus "${a.name}"?`,
    html: `Aset ini akan <strong>disembunyikan dari daftar aktif</strong> — histori perawatan &amp; perolehannya tetap tersimpan.<br>` +
          `Bisa dipulihkan kapan saja lewat filter <strong>Terhapus</strong> di atas.`,
    showCancelButton: true,
    confirmButtonColor: '#dc2626',
    confirmButtonText: 'Ya, hapus aset',
    cancelButtonText: 'Batal',
  })
  if (!r.isConfirmed) return
  try {
    await assetsApi.remove(a.id)
    await loadAll()
    swalBase({ icon: 'success', title: 'Aset dihapus', text: `"${a.name}" sudah tidak tampil di daftar aktif.`, showConfirmButton: false, timer: 1800 })
  } catch (e) {
    swalBase({ icon: 'error', title: 'Gagal menghapus aset', text: e?.message || 'Terjadi kendala saat menghapus data.' })
  }
}

async function confirmRestoreAsset(a) {
  const r = await swalBase({
    icon: 'question',
    title: `Pulihkan "${a.name}"?`,
    html: `Aset akan <strong>aktif kembali</strong> di Daftar Aset beserta seluruh riwayat perolehan dan perawatannya.`,
    showCancelButton: true,
    confirmButtonText: 'Ya, pulihkan aset',
    cancelButtonText: 'Tutup',
  })
  if (!r.isConfirmed) return
  try {
    await assetsApi.restore(a.id)
    await loadAll()
    swalBase({ icon: 'success', title: 'Aset dipulihkan', text: `"${a.name}" sudah aktif kembali.`, showConfirmButton: false, timer: 1800 })
  } catch (e) {
    swalBase({ icon: 'error', title: 'Gagal memulihkan aset', text: e?.message || 'Terjadi kendala saat memulihkan data.' })
  }
}

// ── Maintenance history ──
const historyModal = ref(false)
const activeAsset = ref(null)
const history = ref([])
const loadingHistory = ref(false)
const savingM = ref(false)
const mForm = ref({})
function blankM() { return { maintenance_date: todayDateString(), type: 'rutin', description: '', cost: 0, performed_by: '', condition_after: '', next_due_date: '' } }

async function openHistory(a) {
  activeAsset.value = a
  history.value = []
  mForm.value = blankM()
  historyModal.value = true
  loadingHistory.value = true
  try { history.value = asArray(await assetsApi.maintenances(a.id)) }
  catch (e) { toast.error(e?.message || 'Gagal memuat histori') }
  finally { loadingHistory.value = false }
}
async function saveMaintenance() {
  if (!mForm.value.description?.trim()) { toast.error('Deskripsi wajib diisi'); return }
  savingM.value = true
  try {
    await assetsApi.addMaintenance(activeAsset.value.id, mForm.value)
    toast.success('Perawatan dicatat')
    mForm.value = blankM()
    history.value = asArray(await assetsApi.maintenances(activeAsset.value.id))
    await loadAll() // refresh count/last/kondisi/jadwal di daftar + ringkasan
    const fresh = assets.value.find(x => x.id === activeAsset.value.id)
    if (fresh) activeAsset.value = fresh
  } catch (e) { toast.error(e?.message || 'Gagal menyimpan perawatan') } finally { savingM.value = false }
}

async function deleteMaintenance(m) {
  // Catatan TERBARU menentukan jadwal berikutnya (lihat services/asset.go) —
  // menghapusnya bisa memindahkan atau menghilangkan jadwal yang tampil di daftar.
  const isLatest = history.value[0]?.id === m.id
  const r = await swalBase({
    icon: 'warning',
    title: 'Hapus catatan perawatan ini?',
    html: isLatest
      ? `Ini catatan <strong>terbaru</strong> — menghapusnya dapat mengubah status jadwal perawatan berikutnya untuk aset ini.`
      : `Catatan tanggal ${formatDateStr(m.maintenance_date)} akan dihapus permanen dari histori.`,
    showCancelButton: true,
    confirmButtonColor: '#dc2626',
    confirmButtonText: 'Ya, hapus catatan',
    cancelButtonText: 'Batal',
  })
  if (!r.isConfirmed) return
  try {
    await assetsApi.removeMaintenance(activeAsset.value.id, m.id)
    history.value = history.value.filter(x => x.id !== m.id)
    await loadAll()
    // Menghapus catatan terbaru bisa mengubah jadwal berikutnya — segarkan kartu aktif.
    const fresh = assets.value.find(x => x.id === activeAsset.value.id)
    if (fresh) activeAsset.value = fresh
  } catch (e) {
    swalBase({ icon: 'error', title: 'Gagal menghapus catatan', text: e?.message || 'Terjadi kendala saat menghapus data.' })
  }
}

onMounted(async () => { await loadOutlets(); await loadAll() })
</script>

<style scoped>
.form-input {
  width: 100%; padding: .5rem .7rem; border-radius: .6rem; font-size: .85rem;
  border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none;
}
.form-input:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }
.lbl { display: block; font-size: .72rem; font-weight: 700; color: #4b5563; margin-bottom: .25rem; }
.req { color: #ef4444; }
.hint { color: #9ca3af; font-weight: 400; }
.btn-ghost { padding: .5rem 1rem; border-radius: .6rem; font-size: .85rem; font-weight: 600; color: #374151; background: #f3f4f6; }
.btn-ghost:hover { background: #e5e7eb; }
.lnk { color: #047857; font-weight: 600; text-decoration: none; background: none; border: none; cursor: pointer; font-size: inherit; padding: 0; }
.lnk:hover { text-decoration: underline; }

/* Ikon kecil yang ditempel di depan label tombol (v-html) */
.btn-ic { display: inline-flex; margin-right: .35rem; vertical-align: -2px; }
.btn-ic :deep(svg) { width: .85rem; height: .85rem; }
.btn-ic--inline { margin-right: .25rem; vertical-align: -3px; }
.btn-ic--inline :deep(svg) { width: .8rem; height: .8rem; }

.cond-badge { display: inline-block; padding: .12rem .5rem; border-radius: 999px; font-size: .68rem; font-weight: 700; white-space: nowrap; }
.cond-baik { background: rgba(16,185,129,.13); color: #047857; }
.cond-ringan { background: rgba(245,158,11,.15); color: #b45309; }
.cond-berat { background: rgba(239,68,68,.13); color: #b91c1c; }
.cond-perbaikan { background: rgba(59,130,246,.13); color: #1d4ed8; }

.mtype-badge { display: inline-block; padding: .05rem .45rem; border-radius: 999px; font-size: .65rem; font-weight: 700; background: rgba(99,102,241,.12); color: #4338ca; }
.mtype-badge--rutin { background: rgba(99,102,241,.12); color: #4338ca; }
.mtype-badge--perbaikan { background: rgba(245,158,11,.14); color: #b45309; }
.mtype-badge--penggantian { background: rgba(14,165,233,.13); color: #0369a1; }
.mtype-badge--inspeksi { background: rgba(16,185,129,.13); color: #047857; }

/* ── Kartu ringkasan — flat, warna hanya untuk menandakan sesuatu ── */
.kpi-grid { display: grid; gap: .75rem; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); }
.kpi {
  border-radius: .85rem; padding: .85rem 1rem; background: #fff; text-align: left; display: block;
  border: 1px solid rgba(17,24,39,.08); box-shadow: 0 1px 2px rgba(16,24,40,.04);
}
.kpi--btn { cursor: pointer; transition: transform .12s ease, box-shadow .12s ease; }
.kpi--btn:hover { transform: translateY(-1px); box-shadow: 0 4px 12px rgba(16,24,40,.09); }
.kpi-label { font-size: .68rem; font-weight: 700; color: #6b7280; text-transform: uppercase; letter-spacing: .03em; }
.kpi-val { font-size: 1.45rem; font-weight: 800; color: #111827; line-height: 1.2; margin-top: .15rem; }
.kpi-sub { font-size: .71rem; color: #6b7280; margin-top: .15rem; }
.kpi-on { margin-left: .35rem; padding: .05rem .35rem; border-radius: 999px; background: rgba(17,24,39,.08); color: #374151; font-size: .6rem; }
.kpi--good .kpi-val { color: #047857; }
.kpi--alert { border-color: rgba(225,29,72,.28); background: rgba(225,29,72,.035); }
.kpi--alert .kpi-val { color: #be123c; }
.kpi--warn { border-color: rgba(217,119,6,.3); background: rgba(217,119,6,.04); }
.kpi--warn .kpi-val { color: #b45309; }

.filter-summary { display: flex; align-items: center; justify-content: space-between; gap: .5rem; margin-top: .7rem; padding-top: .7rem; border-top: 1px solid rgba(17,24,39,.06); font-size: .75rem; color: #6b7280; }

/* ── Badge jadwal perawatan ── */
.due-badge { display: inline-block; padding: .12rem .5rem; border-radius: 999px; font-size: .68rem; font-weight: 700; white-space: nowrap; }
.due-overdue { background: rgba(239,68,68,.13); color: #b91c1c; }
.due-soon { background: rgba(245,158,11,.15); color: #b45309; }
.due-scheduled { background: rgba(16,185,129,.13); color: #047857; }
.due-none { background: rgba(107,114,128,.12); color: #4b5563; }

/* ── Tombol aksi ikon (tabel desktop) ── */
.icon-btn {
  display: inline-flex; align-items: center; justify-content: center;
  width: 2rem; height: 2rem; border-radius: .6rem; color: #6b7280;
  background: transparent; transition: background .12s ease, color .12s ease;
}
.icon-btn :deep(svg) { width: 1.05rem; height: 1.05rem; }
.icon-btn--sm { width: 1.6rem; height: 1.6rem; }
.icon-btn--sm :deep(svg) { width: .85rem; height: .85rem; }
.icon-btn--history:hover { background: rgba(16,185,129,.1); color: #047857; }
.icon-btn--edit:hover { background: rgba(79,70,229,.1); color: #4338ca; }
.icon-btn--delete:hover { background: rgba(225,29,72,.1); color: #be123c; }
.icon-btn--restore:hover { background: rgba(14,165,233,.1); color: #0369a1; }

/* ── Tombol aksi pil (kartu mobile) ── */
.pill-btn {
  flex: 1; display: inline-flex; align-items: center; justify-content: center;
  padding: .4rem .5rem; border-radius: .6rem; font-size: .72rem; font-weight: 600;
}
.pill-btn--history { background: rgba(16,185,129,.1); color: #047857; }
.pill-btn--history:hover { background: rgba(16,185,129,.16); }
.pill-btn--edit { background: rgba(79,70,229,.1); color: #4338ca; }
.pill-btn--edit:hover { background: rgba(79,70,229,.16); }
.pill-btn--delete { background: rgba(225,29,72,.1); color: #be123c; }
.pill-btn--delete:hover { background: rgba(225,29,72,.16); }
.pill-btn--restore { background: rgba(14,165,233,.1); color: #0369a1; }
.pill-btn--restore:hover { background: rgba(14,165,233,.16); }

/* ── Peringatan nama duplikat (modal Tambah Aset) ── */
.dup-warning {
  display: flex; align-items: flex-start; gap: .5rem; margin-top: .5rem;
  padding: .6rem .7rem; border-radius: .6rem; background: rgba(217,119,6,.06); border: 1px solid rgba(217,119,6,.2);
}
.dup-warning-ic { flex-shrink: 0; color: #b45309; margin-top: .1rem; }
.dup-warning-ic :deep(svg) { width: 1rem; height: 1rem; }

/* Filter status="terhapus" ditandai visual supaya jelas bukan tampilan biasa */
.form-input--trash { border-color: rgba(225,29,72,.4); background: rgba(225,29,72,.04); color: #be123c; font-weight: 600; }

/* ── Keadaan kosong ── */
.empty-block { padding: 2.5rem 1.5rem; text-align: center; }
.empty-block--inline { padding: 1.75rem 1rem; }
.empty-ic { display: inline-flex; width: 2.75rem; height: 2.75rem; border-radius: .9rem; align-items: center; justify-content: center; color: #9ca3af; background: #f3f4f6; margin-bottom: .7rem; }
.empty-ic :deep(svg) { width: 1.4rem; height: 1.4rem; }
.empty-title { font-size: .88rem; font-weight: 700; color: #374151; }
.empty-desc { font-size: .78rem; color: #9ca3af; margin-top: .25rem; max-width: 24rem; margin-inline: auto; line-height: 1.5; }

/* ── Bagian form modal ── */
.f-section { padding-top: 1.1rem; border-top: 1px solid rgba(17,24,39,.06); }
.f-section:first-child { padding-top: 0; border-top: none; }
.f-section-hd { display: flex; align-items: flex-start; gap: .65rem; margin-bottom: .85rem; }
.f-section-ic {
  flex-shrink: 0; width: 2.1rem; height: 2.1rem; border-radius: .65rem;
  display: flex; align-items: center; justify-content: center;
}
.f-section-ic :deep(svg) { width: 1.1rem; height: 1.1rem; }
.f-section-ic--indigo { background: rgba(79,70,229,.1); color: #4338ca; }
.f-section-ic--sky { background: rgba(14,165,233,.1); color: #0369a1; }
.f-section-ic--emerald { background: rgba(16,185,129,.1); color: #047857; }
.f-section-ic--slate { background: rgba(100,116,139,.12); color: #475569; }
.f-section-title { font-size: .88rem; font-weight: 700; color: #111827; line-height: 1.3; }
.f-section-desc { font-size: .74rem; color: #9ca3af; margin-top: .05rem; }

.hint-text { font-size: .72rem; color: #9ca3af; margin-top: .55rem; line-height: 1.5; }
.hint-text--muted { display: flex; align-items: center; color: #6b7280; }

.preview-box {
  display: flex; align-items: flex-start; gap: .6rem; margin-top: .7rem;
  padding: .7rem .8rem; border-radius: .7rem; background: rgba(16,185,129,.06); border: 1px solid rgba(16,185,129,.18);
}
.preview-ic { flex-shrink: 0; color: #047857; margin-top: .1rem; }
.preview-ic :deep(svg) { width: 1.1rem; height: 1.1rem; }

/* ── Linimasa perawatan ── */
.timeline-item { position: relative; padding-left: 2.2rem; }
.timeline-item::before {
  content: ''; position: absolute; left: .85rem; top: 1.9rem; bottom: -.75rem; width: 2px; background: rgba(17,24,39,.08);
}
.timeline-item:last-child::before { display: none; }
.timeline-dot {
  position: absolute; left: 0; top: 0; width: 1.75rem; height: 1.75rem; border-radius: 999px;
  display: flex; align-items: center; justify-content: center; color: #fff; box-shadow: 0 0 0 3px #fff;
}
.timeline-dot :deep(svg) { width: .9rem; height: .9rem; }
.timeline-dot--rutin { background: #6366f1; }
.timeline-dot--perbaikan { background: #f59e0b; }
.timeline-dot--penggantian { background: #0ea5e9; }
.timeline-dot--inspeksi { background: #10b981; }
</style>
