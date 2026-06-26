import { apiClient } from './client.js'

export const reservationsApi = {
  list:      (params = {}) => apiClient.get('/admin/reservations', { params }),
  get:       (id)          => apiClient.get(`/admin/reservations/${id}`),
  create:    (data)        => apiClient.post('/admin/reservations', data),
  update:    (id, data)    => apiClient.put(`/admin/reservations/${id}`, data),
  setStatus: (id, status)  => apiClient.patch(`/admin/reservations/${id}/status`, { status }),
  remove:    (id)          => apiClient.delete(`/admin/reservations/${id}`),
}
