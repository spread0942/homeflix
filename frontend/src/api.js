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

export function listLibrary(q = '') {
  const query = q ? `?q=${encodeURIComponent(q)}` : ''
  return request(`/library${query}`)
}

export function listSeries(q = '') {
  const query = q ? `?q=${encodeURIComponent(q)}` : ''
  return request(`/series${query}`)
}

export function getSeries(id) {
  return request(`/series/${id}`)
}

export function createSeries(formData) {
  return request('/series', {
    method: 'POST',
    body: formData,
  })
}

export function updateSeries(id, formData) {
  return request(`/series/${id}`, {
    method: 'PUT',
    body: formData,
  })
}

export function deleteSeries(id) {
  return request(`/series/${id}`, { method: 'DELETE' })
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

export function updateAnimation(id, formData) {
  return request(`/animations/${id}`, {
    method: 'PUT',
    body: formData,
  })
}

export function transcodeAnimation(id) {
  return request(`/animations/${id}/transcode`, { method: 'POST' })
}
