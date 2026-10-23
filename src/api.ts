export interface Company { id: string; name: string }
export interface User { id: string; username: string; name: string; company?: Company }
export interface Event {
 id: string; title: string; address: string; startsAt: string; endsAt: string
 recurrenceMonths: number; description: string; createdBy: string; assignedTo?: string
}
export interface AddressSummary { address: string; nextEvent?: Event; previousEvent?: Event }

const tokenKey = 'buldez-session'

export function sessionToken() { return localStorage.getItem(tokenKey) }
export function setSessionToken(token: string) { localStorage.setItem(tokenKey, token) }
export function clearSession() { localStorage.removeItem(tokenKey) }

export async function api<T>(path: string, options: RequestInit = {}): Promise<T> {
 const headers = new Headers(options.headers)
 headers.set('Content-Type', 'application/json')
 const token = sessionToken()
 if (token) headers.set('Authorization', `Bearer ${token}`)
 const response = await fetch(`${import.meta.env.VITE_API_BASE_URL || '/api'}${path}`, { ...options, headers })
 if (!response.ok) {
  const body = await response.json().catch(() => ({ error: 'The server is unavailable.' }))
  if (response.status === 401 && path !== '/auth/login') {
   clearSession()
   window.dispatchEvent(new Event('session-expired'))
  }
  throw new Error(body.error || `Request failed (${response.status})`)
 }
 return response.json()
}

export function dateKey(date: Date) { return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}` }
export function parseDate(value: string) { return new Date(`${value}T12:00:00`) }
export function formatDate(value: string) { return new Intl.DateTimeFormat('bg-BG', { day: 'numeric', month: 'short', year: 'numeric' }).format(new Date(value)) }
export function formatTime(value: string) { return new Intl.DateTimeFormat('bg-BG', { hour: '2-digit', minute: '2-digit' }).format(new Date(value)) }
