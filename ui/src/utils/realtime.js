import { onMounted, onUnmounted } from 'vue'

/**
 * Langganan event realtime (SSE) dari server — device sync memicu event
 * {type, outlet_id}, halaman me-refresh datanya sendiri.
 *
 * useRealtime(['transaction','order'], () => fetchData())
 * - types: jenis event yang dipedulikan halaman ini (null/[] = semua)
 * - callback dipanggil ter-debounce (default 2 dtk) supaya burst batch sync
 *   (banyak event beruntun) hanya memicu satu refresh.
 * EventSource reconnect otomatis saat koneksi putus; halaman tetap berfungsi
 * normal tanpa SSE (fallback: refresh manual / polling yang sudah ada).
 */
export function useRealtime(types, callback, { debounceMs = 2000 } = {}) {
  let es = null
  let timer = null
  const wanted = types && types.length ? new Set(types) : null

  function schedule() {
    clearTimeout(timer)
    timer = setTimeout(() => { try { callback() } catch {} }, debounceMs)
  }

  onMounted(() => {
    const token = localStorage.getItem('cloud_pos_token')
    if (!token || typeof EventSource === 'undefined') return
    es = new EventSource(`/api/v1/admin/events?token=${encodeURIComponent(token)}`)
    es.onmessage = (e) => {
      try {
        const ev = JSON.parse(e.data)
        if (!wanted || wanted.has(ev.type)) schedule()
      } catch { /* abaikan frame non-JSON */ }
    }
    // onerror: EventSource reconnect sendiri; tidak perlu penanganan khusus.
  })

  onUnmounted(() => {
    clearTimeout(timer)
    es?.close()
    es = null
  })
}
