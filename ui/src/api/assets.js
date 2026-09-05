import { apiClient } from './client.js'

export const assetsApi = {
  // Daftar aset & perawatan per aset
  list:              (params = {}) => apiClient.get('/admin/assets', { params }),
  summary:           (params = {}) => apiClient.get('/admin/assets/summary', { params }),
  dashboard:         (params = {}) => apiClient.get('/admin/assets/dashboard', { params }),
  get:               (id)          => apiClient.get(`/admin/assets/${id}`),
  create:            (data)        => apiClient.post('/admin/assets', data),
  update:            (id, data)    => apiClient.put(`/admin/assets/${id}`, data),
  remove:            (id)          => apiClient.delete(`/admin/assets/${id}`),
  restore:           (id)          => apiClient.post(`/admin/assets/${id}/restore`),
  maintenances:      (id)          => apiClient.get(`/admin/assets/${id}/maintenances`),
  addMaintenance:    (id, data)    => apiClient.post(`/admin/assets/${id}/maintenances`, data),
  removeMaintenance: (id, mid)     => apiClient.delete(`/admin/assets/${id}/maintenances/${mid}`),

  // Perawatan lintas aset
  allMaintenances:   (params = {}) => apiClient.get('/admin/assets/maintenances', { params }),

  // Histori perolehan
  acquisitions:       (params = {}) => apiClient.get('/admin/assets/acquisitions', { params }),
  addAcquisition:     (data)        => apiClient.post('/admin/assets/acquisitions', data),
  removeAcquisition:  (id)          => apiClient.delete(`/admin/assets/acquisitions/${id}`),

  // Mutasi antar outlet
  transfers:      (params = {}) => apiClient.get('/admin/assets/transfers', { params }),
  transfer:       (data)        => apiClient.post('/admin/assets/transfers', data),

  // Penghapusan aset
  disposals:      (params = {}) => apiClient.get('/admin/assets/disposals', { params }),
  dispose:        (data)        => apiClient.post('/admin/assets/disposals', data),
  restoreDisposal:(id)          => apiClient.delete(`/admin/assets/disposals/${id}`),
}
