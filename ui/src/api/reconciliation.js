import { apiClient } from './client.js'

export const reconciliationApi = {
  // Rekonsiliasi shift: tutup kasir vs cloud
  report: (params = {}) => apiClient.get('/admin/shift-reconciliation', { params }),
  // Ikuti versi kasir (superadmin) — tercatat & bisa dibatalkan
  apply:  (shiftId) => apiClient.post(`/admin/shift-reconciliation/${shiftId}/apply`),
  revert: (adjId)   => apiClient.post(`/admin/shift-reconciliation/adjustments/${adjId}/revert`),
}
