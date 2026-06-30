import { apiClient } from './client.js'

export const devicesApi = {
  // Status perangkat per outlet (scoped). Optional { outlet_id }.
  monitor: (params = {}) => apiClient.get('/admin/devices', { params }),
  // Histori heartbeat (tren baterai/jaringan) satu outlet.
  history: (outletId) => apiClient.get(`/admin/devices/${outletId}/history`),
}
