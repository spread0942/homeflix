<script setup>
import { computed, nextTick, onMounted, ref } from 'vue'
import {
  createAnimation,
  createSeries,
  deleteAnimation,
  deleteSeries,
  listAnimations,
  listSeries,
  updateAnimation,
  updateSeries,
} from '../api'

const tab = ref('films') // 'films' | 'series'

const films = ref([])
const seriesList = ref([])
const loading = ref(true)
const submitting = ref(false)
const seriesSubmitting = ref(false)
const error = ref('')
const success = ref('')

const editingFilmId = ref(null)
const name = ref('')
const description = ref('')
const seriesId = ref('')
const season = ref('')
const episode = ref('')
const sortOrder = ref('0')
const videoFile = ref(null)
const posterFile = ref(null)
const filmFormEl = ref(null)

const editingSeriesId = ref(null)
const seriesName = ref('')
const seriesDescription = ref('')
const seriesKind = ref('franchise')
const seriesPosterFile = ref(null)
const seriesFormEl = ref(null)

const inSeries = computed(() => Boolean(seriesId.value))
const editingFilm = computed(() => Boolean(editingFilmId.value))
const editingSeries = computed(() => Boolean(editingSeriesId.value))

async function refresh() {
  loading.value = true
  error.value = ''
  try {
    const [a, s] = await Promise.all([listAnimations(), listSeries()])
    films.value = a
    seriesList.value = s
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

onMounted(refresh)

function onVideoChange(e) {
  videoFile.value = e.target.files?.[0] || null
}

function onPosterChange(e) {
  posterFile.value = e.target.files?.[0] || null
}

function onSeriesPosterChange(e) {
  seriesPosterFile.value = e.target.files?.[0] || null
}

/** Prefill season / next episode / sort from existing series entries. */
function applySeriesDefaults(id) {
  if (!id) {
    season.value = ''
    episode.value = ''
    sortOrder.value = '0'
    return
  }
  const entries = films.value.filter((f) => f.series_id === id)
  sortOrder.value = String(entries.length + 1)

  const seasons = entries.map((f) => f.season).filter((s) => s != null)
  if (seasons.length) {
    const lastSeason = Math.max(...seasons)
    season.value = String(lastSeason)
    const epsInSeason = entries
      .filter((f) => f.season === lastSeason)
      .map((f) => f.episode)
      .filter((e) => e != null)
    episode.value = String(epsInSeason.length ? Math.max(...epsInSeason) + 1 : 1)
    return
  }

  const kind = seriesList.value.find((s) => s.id === id)?.kind
  if (kind === 'anime' || kind === 'tv') {
    season.value = '1'
    episode.value = '1'
  } else {
    // Franchise / unknown: leave season & episode blank; sort is enough.
    season.value = ''
    episode.value = ''
  }
}

function onSeriesChange(e) {
  if (editingFilmId.value) return
  applySeriesDefaults(e.target.value)
}

function resetFilmForm(keepSeries = '') {
  editingFilmId.value = null
  name.value = ''
  description.value = ''
  seriesId.value = keepSeries
  videoFile.value = null
  posterFile.value = null
  if (filmFormEl.value) filmFormEl.value.reset()
  seriesId.value = keepSeries
  applySeriesDefaults(keepSeries)
}

function resetSeriesForm() {
  editingSeriesId.value = null
  seriesName.value = ''
  seriesDescription.value = ''
  seriesKind.value = 'franchise'
  seriesPosterFile.value = null
  if (seriesFormEl.value) seriesFormEl.value.reset()
}

function startEditFilm(film) {
  tab.value = 'films'
  editingFilmId.value = film.id
  name.value = film.name || ''
  description.value = film.description || ''
  seriesId.value = film.series_id || ''
  season.value = film.season != null ? String(film.season) : ''
  episode.value = film.episode != null ? String(film.episode) : ''
  sortOrder.value = film.sort_order != null ? String(film.sort_order) : '0'
  videoFile.value = null
  posterFile.value = null
  success.value = ''
  error.value = ''
  nextTick(() => {
    filmFormEl.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  })
}

function cancelEditFilm() {
  resetFilmForm(seriesId.value)
  success.value = ''
  error.value = ''
}

function startEditSeries(s) {
  tab.value = 'series'
  editingSeriesId.value = s.id
  seriesName.value = s.name || ''
  seriesDescription.value = s.description || ''
  seriesKind.value = s.kind || 'franchise'
  seriesPosterFile.value = null
  success.value = ''
  error.value = ''
  nextTick(() => {
    seriesFormEl.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  })
}

function cancelEditSeries() {
  resetSeriesForm()
  success.value = ''
  error.value = ''
}

async function onCreateSeries(e) {
  e.preventDefault()
  success.value = ''
  error.value = ''
  if (!seriesName.value.trim()) {
    error.value = 'Series name is required.'
    return
  }
  const form = new FormData()
  form.append('name', seriesName.value.trim())
  form.append('description', seriesDescription.value.trim())
  form.append('kind', seriesKind.value)
  if (seriesPosterFile.value) form.append('poster', seriesPosterFile.value)

  seriesSubmitting.value = true
  try {
    if (editingSeriesId.value) {
      const updated = await updateSeries(editingSeriesId.value, form)
      success.value = `Series "${updated.name}" updated.`
      resetSeriesForm()
    } else {
      const created = await createSeries(form)
      success.value = `Series "${created.name}" created.`
      resetSeriesForm()
      seriesId.value = created.id
      if (!editingFilmId.value) applySeriesDefaults(created.id)
    }
    await refresh()
  } catch (err) {
    error.value = err.message
  } finally {
    seriesSubmitting.value = false
  }
}

async function onSubmit(e) {
  e.preventDefault()
  success.value = ''
  error.value = ''

  if (!name.value.trim()) {
    error.value = 'Name is required.'
    return
  }
  if (!editingFilm.value && !videoFile.value) {
    error.value = 'Name and video are required.'
    return
  }

  const form = new FormData()
  form.append('name', name.value.trim())
  form.append('description', description.value.trim())
  form.append('series_id', seriesId.value)
  if (season.value !== '') form.append('season', season.value)
  else form.append('season', '')
  if (episode.value !== '') form.append('episode', episode.value)
  else form.append('episode', '')
  form.append('sort_order', sortOrder.value || '0')
  if (!editingFilm.value) {
    form.append('video', videoFile.value)
  }
  if (posterFile.value) {
    form.append('poster', posterFile.value)
  }

  submitting.value = true
  try {
    if (editingFilm.value) {
      await updateAnimation(editingFilmId.value, form)
      success.value = `"${name.value.trim()}" updated.`
      const keepSeries = seriesId.value
      resetFilmForm(keepSeries)
      await refresh()
      applySeriesDefaults(keepSeries)
    } else {
      await createAnimation(form)
      success.value = `"${name.value.trim()}" uploaded.`
      const keepSeries = seriesId.value
      resetFilmForm(keepSeries)
      await refresh()
      applySeriesDefaults(keepSeries)
    }
  } catch (err) {
    error.value = err.message
  } finally {
    submitting.value = false
  }
}

async function removeFilm(id, filmName) {
  if (!confirm(`Delete "${filmName}"?`)) return
  error.value = ''
  try {
    await deleteAnimation(id)
    if (editingFilmId.value === id) resetFilmForm()
    await refresh()
  } catch (e) {
    error.value = e.message
  }
}

async function removeSeries(id, name) {
  if (!confirm(`Delete series "${name}" and all its entries?`)) return
  error.value = ''
  try {
    await deleteSeries(id)
    if (seriesId.value === id) seriesId.value = ''
    if (editingSeriesId.value === id) resetSeriesForm()
    await refresh()
  } catch (e) {
    error.value = e.message
  }
}

function filmMeta(film) {
  const bits = []
  if (film.series_name) bits.push(film.series_name)
  if (film.season != null) bits.push(`S${film.season}`)
  if (film.episode != null) bits.push(`E${film.episode}`)
  if (!film.series_id && film.sort_order) bits.push(`#${film.sort_order}`)
  return bits.join(' · ')
}

function submitLabel() {
  if (submitting.value) return editingFilm.value ? 'Saving…' : 'Uploading…'
  return editingFilm.value ? 'Save changes' : 'Upload'
}

function seriesSubmitLabel() {
  if (seriesSubmitting.value) return editingSeries.value ? 'Saving…' : 'Creating…'
  return editingSeries.value ? 'Save series' : 'Create series'
}
</script>

<template>
  <section class="admin">
    <header class="intro">
      <p class="eyebrow">Administration</p>
      <h1>Admin</h1>
      <p>
        Manage series and films in separate tabs. Posters are stored as
        <strong>WebP</strong>. Track video conversion in
        <RouterLink class="inline-link" to="/conversions">Conversions</RouterLink>.
      </p>
    </header>

    <nav class="tabs" aria-label="Admin sections">
      <button
        type="button"
        class="tab"
        :class="{ active: tab === 'films' }"
        @click="tab = 'films'"
      >
        Films
      </button>
      <button
        type="button"
        class="tab"
        :class="{ active: tab === 'series' }"
        @click="tab = 'series'"
      >
        Series
      </button>
    </nav>

    <p v-if="success" class="banner ok">{{ success }}</p>
    <p v-if="error" class="banner err">{{ error }}</p>

    <div v-show="tab === 'films'" class="panel">
      <form ref="filmFormEl" class="form" @submit="onSubmit">
        <h2>{{ editingFilm ? 'Edit film / episode' : 'Upload film / episode' }}</h2>
        <label>
          Name
          <input
            v-model="name"
            type="text"
            required
            maxlength="200"
            placeholder="Film or episode title"
          />
        </label>
        <label>
          Description
          <textarea
            v-model="description"
            rows="3"
            maxlength="2000"
            placeholder="Log entry / synopsis"
          />
        </label>
        <label>
          Series (optional)
          <select v-model="seriesId" @change="onSeriesChange">
            <option value="">Standalone — no series</option>
            <option v-for="s in seriesList" :key="s.id" :value="s.id">
              {{ s.name }} ({{ s.entry_count }})
            </option>
          </select>
        </label>
        <div v-if="inSeries" class="row3">
          <label>
            Season
            <input v-model="season" type="number" min="1" placeholder="e.g. 1" />
          </label>
          <label>
            Episode
            <input v-model="episode" type="number" min="1" placeholder="e.g. 12" />
          </label>
          <label>
            Sort / part
            <input v-model="sortOrder" type="number" placeholder="1 for Part One" />
          </label>
        </div>
        <p v-if="inSeries" class="hint">
          Movies in a franchise: leave season/episode empty and use sort (1, 2…). Anime/TV: set
          season + episode.
        </p>
        <div class="files">
          <label v-if="!editingFilm">
            Video file
            <input type="file" accept="video/*" required @change="onVideoChange" />
          </label>
          <label>
            {{ editingFilm ? 'New poster (optional)' : 'Poster image (optional)' }}
            <input type="file" accept="image/*" @change="onPosterChange" />
            <span class="field-note">{{
              editingFilm
                ? 'Leave empty to keep the current poster'
                : 'Leave empty to grab a preview frame from the video'
            }}</span>
          </label>
        </div>
        <div class="form-actions">
          <button class="submit" type="submit" :disabled="submitting">
            {{ submitLabel() }}
          </button>
          <button
            v-if="editingFilm"
            type="button"
            class="cancel"
            :disabled="submitting"
            @click="cancelEditFilm"
          >
            Cancel
          </button>
        </div>
      </form>

      <div class="inventory">
        <h2>All films</h2>
        <div v-if="loading" class="state">Loading…</div>
        <ul v-else-if="films.length" class="list">
          <li v-for="film in films" :key="film.id">
            <img :src="film.poster_url" :alt="film.name" />
            <div>
              <strong>{{ film.name }}</strong>
              <p>
                <span class="status-pill" :class="film.playback_status || 'ready'">{{
                  film.playback_status || 'ready'
                }}</span>
                {{ filmMeta(film) || film.description || '—' }}
              </p>
            </div>
            <div class="actions">
              <button type="button" class="edit" @click="startEditFilm(film)">Edit</button>
              <RouterLink :to="{ name: 'watch', params: { id: film.id } }">Watch</RouterLink>
              <button type="button" class="danger" @click="removeFilm(film.id, film.name)">
                Delete
              </button>
            </div>
          </li>
        </ul>
        <div v-else class="state">No films yet.</div>
      </div>
    </div>

    <div v-show="tab === 'series'" class="panel">
      <form ref="seriesFormEl" class="form" @submit="onCreateSeries">
        <h2>{{ editingSeries ? 'Edit series' : 'New series' }}</h2>
        <label>
          Series name
          <input
            v-model="seriesName"
            type="text"
            required
            maxlength="200"
            placeholder="e.g. Bleach"
          />
        </label>
        <label>
          Description
          <textarea
            v-model="seriesDescription"
            rows="2"
            maxlength="2000"
            placeholder="Optional series synopsis"
          />
        </label>
        <div class="row2">
          <label>
            Kind
            <select v-model="seriesKind">
              <option value="franchise">Franchise (movies)</option>
              <option value="anime">Anime</option>
              <option value="tv">TV show</option>
            </select>
          </label>
          <label>
            {{ editingSeries ? 'New cover (optional)' : 'Cover poster (optional)' }}
            <input type="file" accept="image/*" @change="onSeriesPosterChange" />
            <span class="field-note">{{
              editingSeries
                ? 'Leave empty to keep the current cover'
                : 'Converted to WebP on upload'
            }}</span>
          </label>
        </div>
        <div class="form-actions">
          <button class="submit" type="submit" :disabled="seriesSubmitting">
            {{ seriesSubmitLabel() }}
          </button>
          <button
            v-if="editingSeries"
            type="button"
            class="cancel"
            :disabled="seriesSubmitting"
            @click="cancelEditSeries"
          >
            Cancel
          </button>
        </div>
      </form>

      <div class="inventory">
        <h2>Series</h2>
        <div v-if="loading" class="state">Loading…</div>
        <ul v-else-if="seriesList.length" class="list">
          <li v-for="s in seriesList" :key="s.id">
            <img v-if="s.poster_url" :src="s.poster_url" :alt="s.name" />
            <div v-else class="thumb-fallback">▶</div>
            <div>
              <strong>{{ s.name }}</strong>
              <p>{{ s.kind }} · {{ s.entry_count }} entries</p>
            </div>
            <div class="actions">
              <button type="button" class="edit" @click="startEditSeries(s)">Edit</button>
              <RouterLink :to="{ name: 'series', params: { id: s.id } }">Open</RouterLink>
              <button type="button" class="danger" @click="removeSeries(s.id, s.name)">
                Delete
              </button>
            </div>
          </li>
        </ul>
        <div v-else class="state">No series yet.</div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.admin {
  padding: 0.5rem clamp(1rem, 4vw, 3rem) 3rem;
  animation: fadeRise 0.55s ease both;
}

.intro {
  max-width: 40rem;
  margin-bottom: 1.25rem;
}

.eyebrow {
  margin: 0;
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--brown);
}

.intro h1 {
  margin: 0.35rem 0 0.6rem;
  font-family: var(--font-display);
  font-size: clamp(2.2rem, 6vw, 3.4rem);
  color: var(--cream);
  text-shadow: 0 3px 0 color-mix(in srgb, var(--brown) 50%, transparent);
}

.intro p {
  margin: 0;
  color: var(--ink);
}

.inline-link {
  color: var(--accent);
  font-weight: 800;
  text-decoration: underline;
  text-underline-offset: 2px;
}

.tabs {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 1.25rem;
}

.tab {
  padding: 0.55rem 1.15rem;
  border: 2px solid color-mix(in srgb, var(--brown) 35%, transparent);
  border-radius: 999px;
  background: color-mix(in srgb, var(--cream) 85%, transparent);
  color: var(--brown);
  font-weight: 800;
  transition: background 0.2s ease, color 0.2s ease, border-color 0.2s ease;
}

.tab:hover {
  border-color: var(--accent);
}

.tab.active {
  background: var(--accent);
  border-color: var(--accent);
  color: var(--cream);
}

.status-pill {
  display: inline-block;
  margin-right: 0.35rem;
  padding: 0.1rem 0.45rem;
  border-radius: 999px;
  font-size: 0.7rem;
  font-weight: 800;
  text-transform: uppercase;
  letter-spacing: 0.03em;
  vertical-align: middle;
}

.status-pill.ready {
  background: color-mix(in srgb, var(--teal) 25%, transparent);
  color: var(--teal);
}

.status-pill.processing {
  background: color-mix(in srgb, var(--accent) 30%, transparent);
  color: var(--accent);
}

.status-pill.failed {
  background: #fde8e8;
  color: #8b1e1e;
}

.form {
  display: grid;
  gap: 1rem;
  max-width: 40rem;
  padding: 1.25rem;
  margin-bottom: 1.25rem;
  background: color-mix(in srgb, var(--cream) 92%, transparent);
  border: 2px solid color-mix(in srgb, var(--brown) 30%, transparent);
  border-radius: 1rem;
  color: var(--on-light);
}

.form h2 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 1.35rem;
  color: var(--brown);
}

label {
  display: grid;
  gap: 0.4rem;
  font-weight: 700;
  color: var(--brown);
}

.field-note {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--teal);
}

input,
textarea,
select {
  width: 100%;
  padding: 0.7rem 0.85rem;
  border: 2px solid color-mix(in srgb, var(--brown) 40%, transparent);
  border-radius: 0.65rem;
  background: #fff;
  color: var(--on-light);
}

.row2,
.row3,
.files {
  display: grid;
  gap: 1rem;
}

@media (min-width: 640px) {
  .row2,
  .files {
    grid-template-columns: 1fr 1fr;
  }

  .row3 {
    grid-template-columns: 1fr 1fr 1fr;
  }
}

.hint {
  margin: 0;
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--teal);
}

.form-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.65rem;
  align-items: center;
}

.submit {
  justify-self: start;
  padding: 0.75rem 1.4rem;
  border: none;
  border-radius: 999px;
  font-weight: 700;
  color: var(--cream);
  background: var(--accent);
  box-shadow: 0 4px 0 color-mix(in srgb, var(--brown) 55%, transparent);
  transition: transform 0.2s ease;
}

.submit:hover:not(:disabled) {
  transform: translateY(-2px);
}

.submit:disabled {
  opacity: 0.7;
  cursor: wait;
}

.cancel {
  padding: 0.75rem 1.2rem;
  border: 2px solid color-mix(in srgb, var(--brown) 40%, transparent);
  border-radius: 999px;
  font-weight: 700;
  color: var(--brown);
  background: transparent;
}

.banner {
  margin: 0 0 1rem;
  padding: 0.85rem 1rem;
  border-radius: 0.75rem;
  font-weight: 700;
  max-width: 40rem;
}

.banner.ok {
  background: color-mix(in srgb, var(--teal) 20%, var(--cream));
  color: var(--teal);
}

.banner.err {
  background: #fde8e8;
  color: #8b1e1e;
}

.inventory {
  margin-top: 2rem;
}

.inventory h2 {
  margin: 0 0 1rem;
  font-family: var(--font-display);
  color: var(--cream);
}

.list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.85rem;
}

.list li {
  display: grid;
  grid-template-columns: 64px 1fr auto;
  gap: 0.9rem;
  align-items: center;
  padding: 0.75rem;
  background: color-mix(in srgb, var(--cream) 90%, transparent);
  border: 2px solid color-mix(in srgb, var(--brown) 25%, transparent);
  border-radius: 0.85rem;
  color: var(--on-light);
}

.list img,
.thumb-fallback {
  width: 64px;
  height: 96px;
  object-fit: cover;
  border-radius: 0.4rem;
  border: 2px solid var(--brown);
}

.thumb-fallback {
  display: grid;
  place-items: center;
  background: linear-gradient(160deg, var(--accent-yellow), var(--teal));
  font-size: 1.5rem;
}

.list p {
  margin: 0.25rem 0 0;
  font-size: 0.9rem;
  color: color-mix(in srgb, var(--on-light) 70%, transparent);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.list strong {
  font-family: var(--font-display);
}

.actions {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.actions a,
.edit,
.danger {
  padding: 0.35rem 0.7rem;
  border-radius: 999px;
  font-weight: 700;
  text-align: center;
  font-size: 0.85rem;
}

.actions a {
  background: var(--teal);
  color: var(--cream);
}

.edit {
  background: color-mix(in srgb, var(--accent-yellow) 70%, #fff);
  color: var(--brown);
  border: 1px solid color-mix(in srgb, var(--brown) 35%, transparent);
}

.danger {
  background: transparent;
  color: #8b1e1e;
  border: 1px solid #8b1e1e;
}

.state {
  padding: 1.25rem;
  background: color-mix(in srgb, var(--cream) 88%, transparent);
  border-radius: 0.85rem;
  color: var(--on-light);
}

@media (max-width: 560px) {
  .list li {
    grid-template-columns: 56px 1fr;
  }

  .actions {
    grid-column: 1 / -1;
    flex-direction: row;
  }
}
</style>
