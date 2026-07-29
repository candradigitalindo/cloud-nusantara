import { apiClient } from './client.js'

export const cashierShiftsApi = {
  getReport: (params = {}) => apiClient.get('/admin/cashier-shifts', { params }),

  // Download Excel — responseType blob melewati unwrap envelope di client.js;
  // timeout dinaikkan karena file dibuat on-the-fly di server.
  exportReport: (params = {}) =>
    apiClient.get('/admin/cashier-shifts/export', {
      params,
      responseType: 'blob',
      timeout: 120000,
    }),
}
