<template>
  <div class="err-wrap">
    <div class="err-card">
      <!-- Illustration -->
      <div class="err-art">
        <div class="err-blob err-blob--rose" aria-hidden="true" />
        <svg class="err-ill" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
          <rect x="5" y="11" width="14" height="10" rx="2" />
          <path d="M8 11V7a4 4 0 018 0v4" />
          <circle cx="12" cy="16" r="1.2" />
        </svg>
      </div>

      <p class="err-code">403</p>
      <h1 class="err-title">Akses ditolak</h1>
      <p class="err-desc">Anda tidak memiliki izin untuk membuka halaman ini. Hubungi admin bila Anda merasa seharusnya punya akses.</p>

      <div class="err-actions">
        <button type="button" class="err-btn err-btn--ghost" @click="goBack">
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M10 19l-7-7m0 0l7-7m-7 7h18"/></svg>
          Kembali
        </button>
        <RouterLink to="/dashboard" class="err-btn err-btn--primary">
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6"/></svg>
          Ke Dashboard
        </RouterLink>
      </div>
    </div>
  </div>
</template>

<script setup>
import { useRouter } from 'vue-router'
const router = useRouter()
function goBack() {
  if (window.history.length > 1) router.back()
  else router.push('/dashboard')
}
</script>

<style scoped>
.err-wrap { min-height: 72vh; display: flex; align-items: center; justify-content: center; padding: 2rem 1rem; }
.err-card { width: 100%; max-width: 28rem; text-align: center; }

.err-art { position: relative; width: 7.5rem; height: 7.5rem; margin: 0 auto 1.5rem; display: flex; align-items: center; justify-content: center; }
.err-blob { position: absolute; inset: 0; border-radius: 50%; filter: blur(28px); opacity: .35; }
.err-blob--rose { background: radial-gradient(circle at 30% 30%, #fb7185, #f43f5e); }
.err-ill { position: relative; width: 4.25rem; height: 4.25rem; color: #e11d48; }

.err-code {
  font-size: 4.5rem; line-height: 1; font-weight: 900; letter-spacing: -.04em;
  background: linear-gradient(135deg, #e11d48, #fb7185); -webkit-background-clip: text;
  background-clip: text; -webkit-text-fill-color: transparent; margin-bottom: .25rem;
}
.err-title { font-size: 1.25rem; font-weight: 800; color: #0f2d1d; }
.err-desc { font-size: .9rem; color: #6b7280; margin-top: .55rem; line-height: 1.6; }

.err-actions { display: flex; flex-direction: column; gap: .65rem; margin-top: 1.75rem; }
.err-btn {
  display: inline-flex; align-items: center; justify-content: center; gap: .5rem;
  padding: .7rem 1.2rem; border-radius: .75rem; font-size: .9rem; font-weight: 700;
  cursor: pointer; transition: all .15s; border: 1px solid transparent; text-decoration: none;
}
.err-btn--primary { background: #059669; color: #fff; box-shadow: 0 4px 14px rgba(5,150,105,.3); }
.err-btn--primary:hover { background: #047857; }
.err-btn--ghost { background: #fff; color: #374151; border-color: rgba(0,0,0,.12); }
.err-btn--ghost:hover { background: #f9fafb; border-color: rgba(0,0,0,.2); }

@media (min-width: 640px) {
  .err-art { width: 9rem; height: 9rem; }
  .err-ill { width: 5.25rem; height: 5.25rem; }
  .err-code { font-size: 5.5rem; }
  .err-actions { flex-direction: row; justify-content: center; }
  .err-btn { padding: .7rem 1.5rem; }
}
</style>
