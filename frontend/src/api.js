const API_BASE = '/api'

async function request(path, options = {}) {
  const res = await fetch(`${API_BASE}${path}`, options)
  if (res.status === 204) return null
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new Error(data.error || `Request failed (${res.status})`)
  }
  return data
}

export function listAnimations(q = '') {
  const query = q ? `?q=${encodeURIComponent(q)}` : ''
  return request(`/animations${query}`)
}

export function getAnimation(id) {
  return request(`/animations/${id}`)
}

export function deleteAnimation(id) {
  return request(`/animations/${id}`, { method: 'DELETE' })
}

export function createAnimation(formData) {
  return request('/animations', {
    method: 'POST',
    body: formData,
  })
}
