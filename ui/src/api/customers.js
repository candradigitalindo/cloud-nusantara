import { apiClient } from './client.js'

export const customersApi = {
  list: (params = {}) => apiClient.get('/admin/customers', { params }),
  get:  (id)          => apiClient.get(`/admin/customers/${id}`),
}
