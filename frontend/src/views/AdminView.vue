<script setup>
import { computed, onMounted, ref } from 'vue'
import {
  createAnimation,
  createSeries,
  deleteAnimation,
  deleteSeries,
  listAnimations,
  listSeries,
} from '../api'

const films = ref([])
const seriesList = ref([])
const loading = ref(true)
const submitting = ref(false)
const seriesSubmitting = ref(false)
const error = ref('')
const success = ref('')

const name = ref('')
const description = ref('')
const seriesId = ref('')
const season = ref('')
const episode = ref('')
const sortOrder = ref('0')
const videoFile = ref(null)
const posterFile = ref(null)

const seriesName = ref('')
const seriesDescription = ref('')
const seriesKind = ref('franchise')
const seriesPosterFile = ref(null)

const inSeries = computed(() => Boolean(seriesId.value))

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
    const created = await createSeries(form)
    success.value = `Series "${created.name}" is charted.`
    seriesName.value = ''
    seriesDescription.value = ''
    seriesKind.value = 'franchise'
    seriesPosterFile.value = null
    e.target.reset()
    await refresh()
    seriesId.value = created.id
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

  if (!name.value.trim() || !videoFile.value || !posterFile.value) {
    error.value = 'Name, video, and poster are required.'
    return
  }

  const form = new FormData()
  form.append('name', name.value.trim())
  form.append('description', description.value.trim())
  form.append('video', videoFile.value)
  form.append('poster', posterFile.value)
  if (seriesId.value) form.append('series_id', seriesId.value)
  if (season.value !== '') form.append('season', season.value)
  if (episode.value !== '') form.append('episode', episode.value)
  form.append('sort_order', sortOrder.value || '0')

  submitting.value = true
  try {
    await createAnimation(form)
    success.value = `"${name.value.trim()}" is aboard the Sunny.`
    const keepSeries = seriesId.value
    name.value = ''
    description.value = ''
    season.value = ''
    episode.value = ''
    sortOrder.value = '0'
    videoFile.value = null
    posterFile.value = null
    e.target.reset()
    seriesId.value = keepSeries
    await refresh()
  } catch (err) {
    error.value = err.message
  } finally {
    submitting.value = false
  }
}

async function removeFilm(id, filmName) {
  if (!confirm(`Throw "${filmName}" overboard?`)) return
  error.value = ''
  try {
    await deleteAnimation(id)
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
</script>

<template>
  <section class="admin">
    <header class="intro">
      <p class="eyebrow">Shipwright desk</p>
      <h1>Galley-La Dock</h1>
      <p>
        Chart a series (Dune, Bleach, …), then stow films with season / episode / part order.
        <strong>Firefox needs H.264 + AAC in MP4</strong> — MKV/HEVC uploads are auto-converted in the
        background.
      </p>
    </header>

    <form class="form" @submit="onCreateSeries">
      <h2>New series</h2>
      <label>
        Series name
        <input v-model="seriesName" type="text" required maxlength="200" placeholder="e.g. Bleach" />
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
          Cover poster (optional)
          <input type="file" accept="image/*" @change="onSeriesPosterChange" />
        </label>
      </div>
      <button class="submit" type="submit" :disabled="seriesSubmitting">
        {{ seriesSubmitting ? 'Charting…' : 'Create series' }}
      </button>
    </form>

    <form class="form" @submit="onSubmit">
      <h2>Upload film / episode</h2>
      <label>
        Name
        <input v-model="name" type="text" required maxlength="200" placeholder="Film or episode title" />
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
        <select v-model="seriesId">
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
        Movies in a franchise: leave season/episode empty and use sort (1, 2…). Anime/TV: set season
        + episode.
      </p>
      <div class="files">
        <label>
          Video file
          <input type="file" accept="video/*" required @change="onVideoChange" />
        </label>
        <label>
          Poster image
          <input type="file" accept="image/*" required @change="onPosterChange" />
        </label>
      </div>
      <button class="submit" type="submit" :disabled="submitting">
        {{ submitting ? 'Loading cargo…' : 'Stow aboard' }}
      </button>
    </form>

    <p v-if="success" class="banner ok">{{ success }}</p>
    <p v-if="error" class="banner err">{{ error }}</p>

    <div class="inventory">
      <h2>Series</h2>
      <div v-if="loading" class="state">Counting barrels…</div>
      <ul v-else-if="seriesList.length" class="list">
        <li v-for="s in seriesList" :key="s.id">
          <img v-if="s.poster_url" :src="s.poster_url" :alt="s.name" />
          <div v-else class="thumb-fallback">☀</div>
          <div>
            <strong>{{ s.name }}</strong>
            <p>{{ s.kind }} · {{ s.entry_count }} entries</p>
          </div>
          <div class="actions">
            <RouterLink :to="{ name: 'series', params: { id: s.id } }">Open</RouterLink>
            <button type="button" class="danger" @click="removeSeries(s.id, s.name)">Delete</button>
          </div>
        </li>
      </ul>
      <div v-else class="state">No series charted yet.</div>
    </div>

    <div class="inventory">
      <h2>All films</h2>
      <ul v-if="!loading && films.length" class="list">
        <li v-for="film in films" :key="film.id">
          <img :src="film.poster_url" :alt="film.name" />
          <div>
            <strong>{{ film.name }}</strong>
            <p>{{ filmMeta(film) || film.description || '—' }}</p>
          </div>
          <div class="actions">
            <RouterLink :to="{ name: 'watch', params: { id: film.id } }">Watch</RouterLink>
            <button type="button" class="danger" @click="removeFilm(film.id, film.name)">
              Delete
            </button>
          </div>
        </li>
      </ul>
      <div v-else-if="!loading" class="state">Hold is empty.</div>
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
  margin-bottom: 1.5rem;
}

.eyebrow {
  margin: 0;
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--wood-brown);
}

.intro h1 {
  margin: 0.35rem 0 0.6rem;
  font-family: var(--font-display);
  font-size: clamp(2.2rem, 6vw, 3.4rem);
  color: var(--cream-sail);
  text-shadow: 0 3px 0 color-mix(in srgb, var(--wood-brown) 50%, transparent);
}

.intro p {
  margin: 0;
  color: var(--ink);
}

.form {
  display: grid;
  gap: 1rem;
  max-width: 40rem;
  padding: 1.25rem;
  margin-bottom: 1.25rem;
  background: color-mix(in srgb, var(--cream-sail) 92%, transparent);
  border: 2px solid color-mix(in srgb, var(--wood-brown) 30%, transparent);
  border-radius: 1rem;
}

.form h2 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 1.35rem;
  color: var(--wood-brown);
}

label {
  display: grid;
  gap: 0.4rem;
  font-weight: 700;
  color: var(--wood-brown);
}

input,
textarea,
select {
  width: 100%;
  padding: 0.7rem 0.85rem;
  border: 2px solid color-mix(in srgb, var(--wood-brown) 40%, transparent);
  border-radius: 0.65rem;
  background: #fff;
  color: var(--ink);
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
  color: var(--sea-teal);
}

.submit {
  justify-self: start;
  padding: 0.75rem 1.4rem;
  border: none;
  border-radius: 999px;
  font-weight: 700;
  color: var(--cream-sail);
  background: var(--ship-orange);
  box-shadow: 0 4px 0 color-mix(in srgb, var(--wood-brown) 55%, transparent);
  transition: transform 0.2s ease;
}

.submit:hover:not(:disabled) {
  transform: translateY(-2px);
}

.submit:disabled {
  opacity: 0.7;
  cursor: wait;
}

.banner {
  margin: 0 0 1rem;
  padding: 0.85rem 1rem;
  border-radius: 0.75rem;
  font-weight: 700;
  max-width: 40rem;
}

.banner.ok {
  background: color-mix(in srgb, var(--sea-teal) 20%, var(--cream-sail));
  color: var(--sea-teal);
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
  color: var(--cream-sail);
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
  background: color-mix(in srgb, var(--cream-sail) 90%, transparent);
  border: 2px solid color-mix(in srgb, var(--wood-brown) 25%, transparent);
  border-radius: 0.85rem;
}

.list img,
.thumb-fallback {
  width: 64px;
  height: 96px;
  object-fit: cover;
  border-radius: 0.4rem;
  border: 2px solid var(--wood-brown);
}

.thumb-fallback {
  display: grid;
  place-items: center;
  background: linear-gradient(160deg, var(--sunny-yellow), var(--sea-teal));
  font-size: 1.5rem;
}

.list p {
  margin: 0.25rem 0 0;
  font-size: 0.9rem;
  color: color-mix(in srgb, var(--ink) 75%, transparent);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.actions {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.actions a,
.danger {
  padding: 0.35rem 0.7rem;
  border-radius: 999px;
  font-weight: 700;
  text-align: center;
  font-size: 0.85rem;
}

.actions a {
  background: var(--sea-teal);
  color: var(--cream-sail);
}

.danger {
  background: transparent;
  color: #8b1e1e;
  border: 1px solid #8b1e1e;
}

.state {
  padding: 1.25rem;
  background: color-mix(in srgb, var(--cream-sail) 88%, transparent);
  border-radius: 0.85rem;
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
