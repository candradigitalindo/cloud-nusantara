import axios from 'axios'

// Bare client for public (no-auth) endpoints — no token header, no 401 redirect.
const publicClient = axios.create({ baseURL: '/api/v1', timeout: 20000 })

export const publicApi = {
  menu:    (slug)       => publicClient.get(`/public/outlets/${slug}/menu`).then(r => r.data),
  reserve: (slug, data) => publicClient.post(`/public/outlets/${slug}/reservations`, data).then(r => r.data),
}
