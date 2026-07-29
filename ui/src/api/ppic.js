import { apiClient } from './client.js'

export const ppicApi = {
  // Dashboard
  getDashboard: () =>
    apiClient.get('/admin/ppic/dashboard'),

  // Par Level & ROP
  listPlanningParams: (params) =>
    apiClient.get('/admin/ppic/planning-params', { params }),
  savePlanningParams: (rows) =>
    apiClient.put('/admin/ppic/planning-params', { rows }),
  autoFillPlanningParams: (warehouseId) =>
    apiClient.post('/admin/ppic/planning-params/auto-fill', { warehouse_id: warehouseId }),

  // Monitor Kedaluwarsa (FEFO)
  getExpiry: (params) =>
    apiClient.get('/admin/ppic/expiry', { params }),
  ackExpiryBatch: (batchId, note) =>
    apiClient.post(`/admin/ppic/expiry/${batchId}/ack`, { note }),

  // Demand Forecast (Fase 2)
  listForecasts: (params) =>
    apiClient.get('/admin/ppic/forecasts', { params }),
  getForecastHistory: (outletId, product) =>
    apiClient.get('/admin/ppic/forecasts/history', { params: { outlet_id: outletId, product } }),
  saveForecasts: (rows) =>
    apiClient.put('/admin/ppic/forecasts', { rows }),
  generateForecasts: (horizonDays = 7) =>
    apiClient.post('/admin/ppic/forecasts/generate', { horizon_days: horizonDays }, { timeout: 60000 }),

  // MRP (Fase 2)
  listMrpRuns: (params) =>
    apiClient.get('/admin/ppic/mrp/runs', { params }),
  getMrpRun: (id) =>
    apiClient.get(`/admin/ppic/mrp/runs/${id}`),
  runMrp: (data) =>
    apiClient.post('/admin/ppic/mrp/run', data, { timeout: 60000 }),
  mrpCreatePR: (runId, itemIds, notes = '') =>
    apiClient.post(`/admin/ppic/mrp/runs/${runId}/create-pr`, { item_ids: itemIds, notes }),
  mrpCreateTransfer: (runId, itemIds, notes = '') =>
    apiClient.post(`/admin/ppic/mrp/runs/${runId}/create-transfer`, { item_ids: itemIds, notes }),

  // Rencana Produksi & Work Order (Fase 3)
  listProducibleItems: (warehouseId) =>
    apiClient.get('/admin/ppic/producible-items', { params: { warehouse_id: warehouseId } }),
  listProductionPlans: (params) =>
    apiClient.get('/admin/ppic/production-plans', { params }),
  getProductionPlan: (id) =>
    apiClient.get(`/admin/ppic/production-plans/${id}`),
  createProductionPlan: (data) =>
    apiClient.post('/admin/ppic/production-plans', data),
  approveProductionPlan: (id) =>
    apiClient.post(`/admin/ppic/production-plans/${id}/approve`),
  releaseProductionPlan: (id) =>
    apiClient.post(`/admin/ppic/production-plans/${id}/release`),
  cancelProductionPlan: (id) =>
    apiClient.post(`/admin/ppic/production-plans/${id}/cancel`),
  listWorkOrders: (params) =>
    apiClient.get('/admin/ppic/work-orders', { params }),
  getWorkOrder: (id) =>
    apiClient.get(`/admin/ppic/work-orders/${id}`),
  createWorkOrder: (data) =>
    apiClient.post('/admin/ppic/work-orders', data),
  startWorkOrder: (id) =>
    apiClient.post(`/admin/ppic/work-orders/${id}/start`),
  finishWorkOrder: (id, data) =>
    apiClient.post(`/admin/ppic/work-orders/${id}/finish`, data),
  cancelWorkOrder: (id) =>
    apiClient.post(`/admin/ppic/work-orders/${id}/cancel`),

  // Laporan PPIC (Fase 3)
  getReport: (tab, params) =>
    apiClient.get(`/admin/ppic/reports/${tab}`, { params }),
  // Download Excel — responseType blob melewati unwrap envelope di client.js.
  exportReport: (tab, params) =>
    apiClient.get(`/admin/ppic/reports/${tab}/export`, { params, responseType: 'blob', timeout: 120000 }),

  // Stock Opname
  listOpnames: (params) =>
    apiClient.get('/admin/ppic/opnames', { params }),
  getOpname: (id) =>
    apiClient.get(`/admin/ppic/opnames/${id}`),
  createOpname: (data) =>
    apiClient.post('/admin/ppic/opnames', data),
  saveOpnameCounts: (id, items) =>
    apiClient.put(`/admin/ppic/opnames/${id}/items`, { items }),
  submitOpname: (id) =>
    apiClient.post(`/admin/ppic/opnames/${id}/submit`),
  approveOpname: (id) =>
    apiClient.post(`/admin/ppic/opnames/${id}/approve`),
  cancelOpname: (id) =>
    apiClient.post(`/admin/ppic/opnames/${id}/cancel`),
}
