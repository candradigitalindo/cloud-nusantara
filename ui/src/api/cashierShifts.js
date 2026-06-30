import { apiClient } from './client.js'

export const cashierShiftsApi = {
  getReport: (params = {}) => apiClient.get('/admin/cashier-shifts', { params }),
}
