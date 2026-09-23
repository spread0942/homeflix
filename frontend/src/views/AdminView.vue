<script setup>
import { computed, nextTick, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import ConversionsPanel from '../components/ConversionsPanel.vue'
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

const route = useRoute()
const section = computed(() =>
  route.name === 'admin-conversions' ? 'conversions' : 'upload',
)

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

const batchItems = ref([])
const batchSeriesId = ref('')
const batchSeason = ref('')
const batchFind = ref('')
const batchReplace = ref('')
const batchUseRegex = ref(false)
const batchFileInput = ref(null)
/** Entries uploaded this session but not yet in `films` (for episode defaults). */
const sessionExtras = ref([])
let batchSeq = 0

const editingSeriesId = ref(null)
const seriesName = ref('')
const seriesDescription = ref('')
const seriesKind = ref('franchise')
const seriesPosterFile = ref(null)
const seriesFormEl = ref(null)

const inSeries = computed(() => Boolean(seriesId.value))
const editingFilm = computed(() => Boolean(editingFilmId.value))
const editingSeries = computed(() => Boolean(editingSeriesId.value))
const pendingBatchCount = computed(
  () => batchItems.value.filter((r) => r.status === 'pending' || r.status === 'error').length,
)
const batchBusy = computed(() => batchItems.value.some((r) => r.status === 'uploading'))

const batchReplacePreview = computed(() => {
  const find = batchFind.value
  if (!find || !batchItems.value.length) {
    return { error: null, items: [] }
  }
  const tool = makeTitleReplacer(find, batchReplace.value, batchUseRegex.value)
  if (tool.error) return { error: tool.error, items: [] }
  const items = []
  for (const row of batchItems.value) {
    if (row.status === 'uploading' || row.status === 'done') continue
    const after = tool.apply(row.name)
    if (after === row.name) continue
    items.push({ key: row.key, before: row.name, after })
  }
  return { error: null, items }
})

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

function formatSize(bytes) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 ** 2) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 ** 3) return `${(bytes / 1024 ** 2).toFixed(1)} MB`
  return `${(bytes / 1024 ** 3).toFixed(2)} GB`
}

function normalizeEntry(partial) {
  const seasonVal = partial.season
  const episodeVal = partial.episode
  return {
    season: seasonVal === '' || seasonVal == null ? null : Number(seasonVal),
    episode: episodeVal === '' || episodeVal == null ? null : Number(episodeVal),
    sort_order: Number(partial.sort_order ?? partial.sortOrder ?? 0) || 0,
  }
}

/** Compute season / episode / sort from library + optional pending extras. */
function computeSeriesDefaults(id, extraEntries = []) {
  if (!id) {
    return { season: '', episode: '', sortOrder: '0' }
  }
  const entries = [
    ...films.value.filter((f) => f.series_id === id).map(normalizeEntry),
    ...sessionExtras.value.filter((f) => f.series_id === id).map(normalizeEntry),
    ...extraEntries.map(normalizeEntry),
  ]
  const sortOrderVal = String(entries.length + 1)

  const seasons = entries.map((f) => f.season).filter((s) => s != null && !Number.isNaN(s))
  if (seasons.length) {
    const lastSeason = Math.max(...seasons)
    const epsInSeason = entries
      .filter((f) => f.season === lastSeason)
      .map((f) => f.episode)
      .filter((e) => e != null && !Number.isNaN(e))
    return {
      season: String(lastSeason),
      episode: String(epsInSeason.length ? Math.max(...epsInSeason) + 1 : 1),
      sortOrder: sortOrderVal,
    }
  }

  const kind = seriesList.value.find((s) => s.id === id)?.kind
  if (kind === 'anime' || kind === 'tv') {
    return { season: '1', episode: '1', sortOrder: sortOrderVal }
  }
  return { season: '', episode: '', sortOrder: sortOrderVal }
}

function applySeriesDefaults(id) {
  const d = computeSeriesDefaults(id)
  season.value = d.season
  episode.value = d.episode
  sortOrder.value = d.sortOrder
}

function onSeriesChange(e) {
  if (editingFilmId.value) return
  applySeriesDefaults(e.target.value)
}

function extrasBeforeBatchIndex(index, seriesKey) {
  return batchItems.value
    .slice(0, index)
    .filter((r) => r.seriesId === seriesKey && r.status !== 'done')
    .map((r) => ({ season: r.season, episode: r.episode, sortOrder: r.sortOrder }))
}

function applyDefaultsToBatchRow(row, index) {
  const d = computeSeriesDefaults(row.seriesId, extrasBeforeBatchIndex(index, row.seriesId))
  row.season = d.season
  row.episode = d.episode
  row.sortOrder = d.sortOrder
}

function onBatchSeriesChange(row, index, e) {
  if (row.status === 'uploading' || row.status === 'done') return
  row.seriesId = e?.target?.value ?? row.seriesId
  applyDefaultsToBatchRow(row, index)
}

/** Next episode/sort for a forced season, using library + pending extras. */
function computeNextInSeason(id, seasonNum, extraEntries = []) {
  const entries = [
    ...films.value.filter((f) => f.series_id === id).map(normalizeEntry),
    ...sessionExtras.value.filter((f) => f.series_id === id).map(normalizeEntry),
    ...extraEntries.map(normalizeEntry),
  ]
  const sortOrder = String(entries.length + 1)
  const epsInSeason = entries
    .filter((f) => f.season === seasonNum)
    .map((f) => f.episode)
    .filter((e) => e != null && !Number.isNaN(e))
  return {
    season: String(seasonNum),
    episode: String(epsInSeason.length ? Math.max(...epsInSeason) + 1 : 1),
    sortOrder,
  }
}

function defaultsForBatchRow(id, extras) {
  if (!id) {
    return { season: '', episode: '', sortOrder: '0' }
  }
  const seasonOverride = batchSeason.value
  if (seasonOverride !== '' && seasonOverride != null) {
    return computeNextInSeason(id, Number(seasonOverride), extras)
  }
  return computeSeriesDefaults(id, extras)
}

function applyBatchSeriesToAll() {
  const id = batchSeriesId.value
  const extras = []
  for (const row of batchItems.value) {
    if (row.status === 'uploading' || row.status === 'done') continue
    row.seriesId = id
    const d = defaultsForBatchRow(id, extras)
    row.season = d.season
    row.episode = d.episode
    row.sortOrder = d.sortOrder
    if (id) extras.push({ season: d.season, episode: d.episode, sortOrder: d.sortOrder })
  }
}

function applyBatchFindReplace() {
  const find = batchFind.value
  if (!find) {
    error.value = 'Enter text to find before replacing.'
    return
  }
  const tool = makeTitleReplacer(find, batchReplace.value, batchUseRegex.value)
  if (tool.error) {
    error.value = `Invalid regex: ${tool.error}`
    return
  }
  let changed = 0
  for (const row of batchItems.value) {
    if (row.status === 'uploading' || row.status === 'done') continue
    const next = tool.apply(row.name)
    if (next === row.name) continue
    row.name = next
    changed += 1
  }
  error.value = ''
  success.value = changed
    ? `Updated ${changed} title${changed === 1 ? '' : 's'}.`
    : 'No titles matched.'
}

function makeTitleReplacer(find, replace, useRegex) {
  if (useRegex) {
    try {
      const re = new RegExp(find, 'g')
      return {
        error: null,
        apply(text) {
          re.lastIndex = 0
          return text.replace(re, replace)
        },
      }
    } catch (e) {
      return { error: e.message, apply: (t) => t }
    }
  }
  return {
    error: null,
    apply(text) {
      return text.split(find).join(replace)
    },
  }
}

function newBatchRow(file) {
  batchSeq += 1
  return {
    key: `batch-${batchSeq}`,
    file,
    name: file.name.replace(/\.[^.]+$/, '') || file.name,
    description: '',
    seriesId: '',
    season: '',
    episode: '',
    sortOrder: '0',
    status: 'pending',
    error: '',
  }
}

function onBatchFiles(e) {
  const files = [...(e.target.files || [])]
  if (!files.length) return
  editingFilmId.value = null
  const rows = files.map(newBatchRow)

  if (batchSeriesId.value) {
    const extras = batchItems.value
      .filter((r) => r.seriesId === batchSeriesId.value && r.status !== 'done')
      .map((r) => ({ season: r.season, episode: r.episode, sortOrder: r.sortOrder }))
    for (const row of rows) {
      row.seriesId = batchSeriesId.value
      const d = defaultsForBatchRow(batchSeriesId.value, extras)
      row.season = d.season
      row.episode = d.episode
      row.sortOrder = d.sortOrder
      extras.push({ season: d.season, episode: d.episode, sortOrder: d.sortOrder })
    }
  }

  batchItems.value.push(...rows)
  if (batchFileInput.value) batchFileInput.value.value = ''
  success.value = ''
  error.value = ''
}

function removeBatchRow(key) {
  batchItems.value = batchItems.value.filter((r) => r.key !== key)
}

function clearBatch() {
  if (batchBusy.value) return
  batchItems.value = []
  if (batchFileInput.value) batchFileInput.value.value = ''
}

function onPosterChange(e) {
  posterFile.value = e.target.files?.[0] || null
}

function onSeriesPosterChange(e) {
  seriesPosterFile.value = e.target.files?.[0] || null
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
      batchSeriesId.value = created.id
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

function buildAnimationForm(fields, { video, poster } = {}) {
  const form = new FormData()
  form.append('name', fields.name.trim())
  form.append('description', (fields.description || '').trim())
  form.append('series_id', fields.seriesId || '')
  form.append('season', fields.season !== '' && fields.season != null ? fields.season : '')
  form.append('episode', fields.episode !== '' && fields.episode != null ? fields.episode : '')
  form.append('sort_order', fields.sortOrder || '0')
  if (video) form.append('video', video)
  if (poster) form.append('poster', poster)
  return form
}

async function onSubmit(e) {
  e.preventDefault()
  success.value = ''
  error.value = ''

  if (!name.value.trim()) {
    error.value = 'Name is required.'
    return
  }
  if (!editingFilm.value) {
    error.value = 'Use the batch uploader to add new films.'
    return
  }

  const form = buildAnimationForm(
    {
      name: name.value,
      description: description.value,
      seriesId: seriesId.value,
      season: season.value,
      episode: episode.value,
      sortOrder: sortOrder.value,
    },
    { poster: posterFile.value },
  )

  submitting.value = true
  try {
    await updateAnimation(editingFilmId.value, form)
    success.value = `"${name.value.trim()}" updated.`
    const keepSeries = seriesId.value
    resetFilmForm(keepSeries)
    await refresh()
    applySeriesDefaults(keepSeries)
  } catch (err) {
    error.value = err.message
  } finally {
    submitting.value = false
  }
}

async function uploadBatch() {
  const queue = batchItems.value.filter((r) => r.status === 'pending' || r.status === 'error')
  if (!queue.length) {
    error.value = 'Add one or more video files first.'
    return
  }
  if (queue.some((r) => !r.name.trim())) {
    error.value = 'Every row needs a title.'
    return
  }

  submitting.value = true
  success.value = ''
  error.value = ''
  let ok = 0
  let fail = 0

  for (const row of queue) {
    row.status = 'uploading'
    row.error = ''
    try {
      const form = buildAnimationForm(
        {
          name: row.name,
          description: row.description,
          seriesId: row.seriesId,
          season: row.season,
          episode: row.episode,
          sortOrder: row.sortOrder,
        },
        { video: row.file },
      )
      await createAnimation(form)
      row.status = 'done'
      ok += 1
      if (row.seriesId) {
        sessionExtras.value.push({
          series_id: row.seriesId,
          season: row.season === '' ? null : Number(row.season),
          episode: row.episode === '' ? null : Number(row.episode),
          sort_order: Number(row.sortOrder) || 0,
        })
      }
    } catch (err) {
      row.status = 'error'
      row.error = err.message
      fail += 1
    }
  }

  await refresh()
  sessionExtras.value = []
  batchItems.value = batchItems.value.filter((r) => r.status !== 'done')
  if (ok && !fail) success.value = `Uploaded ${ok} film${ok === 1 ? '' : 's'}.`
  else if (ok && fail) success.value = `Uploaded ${ok}, ${fail} failed.`
  else if (fail) error.value = `All ${fail} upload${fail === 1 ? '' : 's'} failed.`
  submitting.value = false
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

async function removeSeries(id, seriesLabel) {
  if (!confirm(`Delete series "${seriesLabel}" and all its entries?`)) return
  error.value = ''
  try {
    await deleteSeries(id)
    if (seriesId.value === id) seriesId.value = ''
    if (batchSeriesId.value === id) {
      batchSeriesId.value = ''
      batchSeason.value = ''
    }
    for (const row of batchItems.value) {
      if (row.seriesId === id) {
        row.seriesId = ''
        row.season = ''
        row.episode = ''
        row.sortOrder = '0'
      }
    }
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

function rowStatusLabel(row) {
  if (row.status === 'uploading') return 'Uploading…'
  if (row.status === 'done') return 'Done'
  if (row.status === 'error') return row.error || 'Failed'
  return 'Ready'
}
</script>

<template>
  <section class="admin">
    <header class="intro">
      <p class="eyebrow">Administration</p>
      <h1>Admin</h1>
      <p>
        Upload and manage series or films (posters stored as <strong>WebP</strong>). Check conversion
        status and retry failed jobs in the Conversions tab.
      </p>
    </header>

    <nav class="tabs" aria-label="Admin sections">
      <RouterLink class="tab" :to="{ name: 'admin-upload' }">Upload</RouterLink>
      <RouterLink class="tab" :to="{ name: 'admin-conversions' }">Conversions</RouterLink>
    </nav>

    <template v-if="section === 'upload'">
      <nav class="tabs subtabs" aria-label="Upload sections">
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
      <!-- Edit existing film -->
      <form v-if="editingFilm" ref="filmFormEl" class="form" @submit="onSubmit">
        <h2>Edit film / episode</h2>
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
        <label>
          New poster (optional)
          <input type="file" accept="image/*" @change="onPosterChange" />
          <span class="field-note">Leave empty to keep the current poster</span>
        </label>
        <div class="form-actions">
          <button class="submit" type="submit" :disabled="submitting">
            {{ submitting ? 'Saving…' : 'Save changes' }}
          </button>
          <button type="button" class="cancel" :disabled="submitting" @click="cancelEditFilm">
            Cancel
          </button>
        </div>
      </form>

      <!-- Bulk upload -->
      <div v-else class="batch form-wide">
        <div class="batch-head">
          <h2>Upload films / episodes</h2>
          <p class="hint">
            Select several videos, edit titles and series in the table, then upload all.
          </p>
        </div>

        <div class="batch-toolbar">
          <label class="file-pick">
            Add videos
            <input
              ref="batchFileInput"
              type="file"
              accept="video/*"
              multiple
              :disabled="batchBusy"
              @change="onBatchFiles"
            />
          </label>
          <label class="batch-series">
            Series for new / all rows
            <select v-model="batchSeriesId" :disabled="batchBusy">
              <option value="">Standalone — no series</option>
              <option v-for="s in seriesList" :key="s.id" :value="s.id">
                {{ s.name }} ({{ s.entry_count }})
              </option>
            </select>
          </label>
          <label v-if="batchSeriesId" class="batch-season">
            Season for all
            <input
              v-model="batchSeason"
              type="number"
              min="1"
              placeholder="auto"
              :disabled="batchBusy"
            />
          </label>
          <button
            type="button"
            class="cancel"
            :disabled="batchBusy || !batchItems.length"
            @click="applyBatchSeriesToAll"
          >
            Apply to all
          </button>
        </div>

        <div class="batch-replace">
          <label>
            Find in titles
            <input
              v-model="batchFind"
              type="text"
              :placeholder="batchUseRegex ? 'e.g. ^S\\d+E\\d+\\s*' : 'text to find'"
              :disabled="batchBusy || !batchItems.length"
              @keydown.enter.prevent="applyBatchFindReplace"
            />
          </label>
          <label>
            Replace with
            <input
              v-model="batchReplace"
              type="text"
              :placeholder="batchUseRegex ? 'replacement ($1, $2… ok)' : 'replacement (can be empty)'"
              :disabled="batchBusy || !batchItems.length"
              @keydown.enter.prevent="applyBatchFindReplace"
            />
          </label>
          <label class="regex-toggle">
            <input v-model="batchUseRegex" type="checkbox" :disabled="batchBusy || !batchItems.length" />
            Regex
          </label>
          <button
            type="button"
            class="cancel"
            :disabled="batchBusy || !batchItems.length || !batchFind || !!batchReplacePreview.error"
            @click="applyBatchFindReplace"
          >
            Replace in all
          </button>
        </div>

        <div v-if="batchFind && batchItems.length" class="replace-preview">
          <p v-if="batchReplacePreview.error" class="preview-error">
            Invalid regex: {{ batchReplacePreview.error }}
          </p>
          <template v-else>
            <p class="preview-summary">
              {{
                batchReplacePreview.items.length
                  ? `${batchReplacePreview.items.length} title${batchReplacePreview.items.length === 1 ? '' : 's'} would change`
                  : 'No titles would change'
              }}
            </p>
            <ul v-if="batchReplacePreview.items.length" class="preview-list">
              <li v-for="item in batchReplacePreview.items" :key="item.key">
                <span class="preview-before" :title="item.before">{{ item.before }}</span>
                <span class="preview-arrow" aria-hidden="true">→</span>
                <span class="preview-after" :title="item.after">{{ item.after || '(empty)' }}</span>
              </li>
            </ul>
          </template>
        </div>

        <div v-if="batchItems.length" class="batch-table-wrap">
          <table class="batch-table">
            <thead>
              <tr>
                <th scope="col">File</th>
                <th scope="col">Title</th>
                <th scope="col">Series</th>
                <th scope="col">Season</th>
                <th scope="col">Episode</th>
                <th scope="col">Sort</th>
                <th scope="col">Status</th>
                <th scope="col"><span class="sr-only">Remove</span></th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="(row, index) in batchItems"
                :key="row.key"
                :class="[`is-${row.status}`]"
              >
                <td class="file-cell">
                  <span class="file-name" :title="row.file.name">{{ row.file.name }}</span>
                  <span class="file-size">{{ formatSize(row.file.size) }}</span>
                </td>
                <td>
                  <input
                    v-model="row.name"
                    type="text"
                    maxlength="200"
                    :disabled="row.status === 'uploading' || row.status === 'done'"
                    aria-label="Title"
                  />
                </td>
                <td>
                  <select
                    v-model="row.seriesId"
                    :disabled="row.status === 'uploading' || row.status === 'done'"
                    aria-label="Series"
                    @change="onBatchSeriesChange(row, index, $event)"
                  >
                    <option value="">—</option>
                    <option v-for="s in seriesList" :key="s.id" :value="s.id">
                      {{ s.name }}
                    </option>
                  </select>
                </td>
                <td class="num">
                  <input
                    v-model="row.season"
                    type="number"
                    min="1"
                    :disabled="row.status === 'uploading' || row.status === 'done' || !row.seriesId"
                    aria-label="Season"
                  />
                </td>
                <td class="num">
                  <input
                    v-model="row.episode"
                    type="number"
                    min="1"
                    :disabled="row.status === 'uploading' || row.status === 'done' || !row.seriesId"
                    aria-label="Episode"
                  />
                </td>
                <td class="num">
                  <input
                    v-model="row.sortOrder"
                    type="number"
                    :disabled="row.status === 'uploading' || row.status === 'done' || !row.seriesId"
                    aria-label="Sort"
                  />
                </td>
                <td class="status-cell">
                  <span class="status-pill" :class="row.status">{{ rowStatusLabel(row) }}</span>
                </td>
                <td>
                  <button
                    type="button"
                    class="danger icon-btn"
                    :disabled="row.status === 'uploading' || batchBusy"
                    :aria-label="`Remove ${row.file.name}`"
                    @click="removeBatchRow(row.key)"
                  >
                    ✕
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else class="batch-empty">No files selected yet.</p>

        <div class="form-actions">
          <button
            type="button"
            class="submit"
            :disabled="submitting || !pendingBatchCount"
            @click="uploadBatch"
          >
            {{
              submitting
                ? 'Uploading…'
                : pendingBatchCount
                  ? `Upload ${pendingBatchCount} file${pendingBatchCount === 1 ? '' : 's'}`
                  : 'Upload'
            }}
          </button>
          <button
            type="button"
            class="cancel"
            :disabled="submitting || !batchItems.length"
            @click="clearBatch"
          >
            Clear queue
          </button>
        </div>
      </div>

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
            {{
              seriesSubmitting
                ? editingSeries
                  ? 'Saving…'
                  : 'Creating…'
                : editingSeries
                  ? 'Save series'
                  : 'Create series'
            }}
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
    </template>

    <ConversionsPanel v-else />
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
  color: var(--accent);
}

.intro h1 {
  margin: 0.35rem 0 0.6rem;
  font-family: var(--font-display);
  font-size: clamp(2.2rem, 6vw, 3.4rem);
  color: var(--cream);
}

.intro p {
  margin: 0;
  color: var(--ink);
}

.inline-link {
  color: var(--accent);
  font-weight: 700;
  text-decoration: underline;
  text-underline-offset: 2px;
}

.tabs {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 1.25rem;
}

.tabs.subtabs {
  margin-top: -0.35rem;
}

.tab {
  padding: 0.55rem 1.15rem;
  border: 1px solid color-mix(in srgb, var(--cream) 30%, transparent);
  border-radius: 0.35rem;
  background: transparent;
  color: var(--cream);
  font-weight: 700;
  text-decoration: none;
  transition: background 0.15s ease, color 0.15s ease, border-color 0.15s ease;
}

.tab:hover {
  border-color: var(--accent);
  color: var(--accent);
}

.tab.active,
.tab.router-link-active {
  background: transparent;
  border-color: var(--accent);
  color: var(--accent);
}

.status-pill {
  display: inline-block;
  margin-right: 0.35rem;
  padding: 0.1rem 0.45rem;
  border-radius: 0.25rem;
  font-size: 0.7rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.03em;
  vertical-align: middle;
}

.status-pill.ready,
.status-pill.pending,
.status-pill.done {
  background: transparent;
  border: 1px solid color-mix(in srgb, var(--accent) 55%, transparent);
  color: var(--accent);
}

.status-pill.processing,
.status-pill.uploading {
  background: color-mix(in srgb, var(--accent) 20%, transparent);
  color: var(--accent);
}

.status-pill.failed,
.status-pill.error {
  background: transparent;
  border: 1px solid #8b1e1e;
  color: #fde8e8;
}

.form,
.form-wide {
  display: grid;
  gap: 1rem;
  padding: 1.25rem;
  margin-bottom: 1.25rem;
  background: transparent;
  border: 1px solid color-mix(in srgb, var(--cream) 22%, transparent);
  border-radius: 0.35rem;
  color: var(--cream);
}

.form {
  max-width: 40rem;
}

.form-wide {
  max-width: min(100%, 72rem);
}

.form h2,
.batch-head h2 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 1.35rem;
  color: var(--cream);
}

label {
  display: grid;
  gap: 0.4rem;
  font-weight: 700;
  color: color-mix(in srgb, var(--cream) 85%, transparent);
}

.field-note {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--accent);
}

input,
textarea,
select {
  width: 100%;
  padding: 0.7rem 0.85rem;
  border: 1px solid color-mix(in srgb, var(--cream) 30%, transparent);
  border-radius: 0.35rem;
  background: transparent;
  color: var(--cream);
}

input::placeholder,
textarea::placeholder {
  color: color-mix(in srgb, var(--cream) 45%, transparent);
}

input[type='file'] {
  color: color-mix(in srgb, var(--cream) 75%, transparent);
}

input[type='file']::file-selector-button {
  margin-right: 0.75rem;
  padding: 0.4rem 0.75rem;
  border: 1px solid color-mix(in srgb, var(--cream) 35%, transparent);
  border-radius: 0.35rem;
  background: transparent;
  color: var(--cream);
  font: inherit;
  font-weight: 700;
  cursor: pointer;
}

input[type='file']::file-selector-button:hover {
  border-color: var(--accent);
  color: var(--accent);
}

option {
  background: var(--bg);
  color: var(--cream);
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
  color: color-mix(in srgb, var(--cream) 72%, transparent);
}

.hint code {
  font-size: 0.85em;
  font-weight: 700;
  color: var(--accent);
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
  border-radius: 0.35rem;
  font-weight: 700;
  color: var(--cream);
  background: var(--accent);
  transition: background 0.15s ease;
}

.submit:hover:not(:disabled) {
  background: color-mix(in srgb, var(--accent) 85%, #000);
}

.submit:disabled {
  opacity: 0.7;
  cursor: not-allowed;
}

.cancel {
  padding: 0.75rem 1.2rem;
  border: 1px solid color-mix(in srgb, var(--cream) 35%, transparent);
  border-radius: 0.35rem;
  font-weight: 700;
  color: var(--cream);
  background: transparent;
}

.cancel:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.banner {
  margin: 0 0 1rem;
  padding: 0.85rem 1rem;
  border-radius: 0.35rem;
  border: 1px solid transparent;
  font-weight: 700;
  max-width: 40rem;
}

.banner.ok {
  background: transparent;
  border-color: color-mix(in srgb, var(--accent) 45%, transparent);
  color: var(--accent);
}

.banner.err {
  background: transparent;
  border-color: #8b1e1e;
  color: #8b1e1e;
}

.batch-toolbar {
  display: grid;
  gap: 0.85rem;
  align-items: end;
}

@media (min-width: 720px) {
  .batch-toolbar {
    grid-template-columns: minmax(10rem, 14rem) minmax(12rem, 1fr) auto;
  }

  .batch-toolbar:has(.batch-season) {
    grid-template-columns: minmax(10rem, 14rem) minmax(10rem, 1fr) 6.5rem auto;
  }
}

.batch-season input {
  max-width: 6.5rem;
}

.batch-replace {
  display: grid;
  gap: 0.85rem;
  align-items: end;
}

@media (min-width: 720px) {
  .batch-replace {
    grid-template-columns: 1fr 1fr auto auto;
  }
}

.regex-toggle {
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 0.45rem;
  padding-bottom: 0.55rem;
  font-weight: 700;
  color: var(--cream);
  white-space: nowrap;
}

.regex-toggle input {
  width: auto;
  margin: 0;
  accent-color: var(--accent);
}

.replace-preview {
  padding: 0.85rem 1rem;
  border-radius: 0.35rem;
  background: transparent;
  border: 1px dashed color-mix(in srgb, var(--cream) 30%, transparent);
}

.preview-summary {
  margin: 0;
  font-size: 0.85rem;
  font-weight: 700;
  color: var(--cream);
}

.preview-error {
  margin: 0;
  font-size: 0.85rem;
  font-weight: 700;
  color: #8b1e1e;
}

.preview-list {
  list-style: none;
  margin: 0.65rem 0 0;
  padding: 0;
  display: grid;
  gap: 0.4rem;
  max-height: 12rem;
  overflow-y: auto;
}

.preview-list li {
  display: grid;
  gap: 0.15rem 0.5rem;
  padding: 0.35rem 0;
  border-bottom: 1px solid color-mix(in srgb, var(--cream) 12%, transparent);
  font-size: 0.82rem;
  font-weight: 600;
}

.preview-list li:last-child {
  border-bottom: none;
}

.preview-before {
  color: color-mix(in srgb, var(--cream) 55%, transparent);
  text-decoration: line-through;
  overflow-wrap: anywhere;
  word-break: break-word;
}

.preview-arrow {
  display: none;
}

.preview-after {
  color: var(--cream);
  overflow-wrap: anywhere;
  word-break: break-word;
}

.preview-after::before {
  content: '→ ';
  color: var(--accent);
  font-weight: 700;
}

.batch-empty {
  margin: 0;
  padding: 1rem;
  border-radius: 0.35rem;
  border: 1px solid color-mix(in srgb, var(--cream) 22%, transparent);
  background: transparent;
  color: color-mix(in srgb, var(--cream) 75%, transparent);
  font-weight: 600;
}

.batch-table-wrap {
  overflow-x: auto;
  border-radius: 0.35rem;
  border: 1px solid color-mix(in srgb, var(--cream) 22%, transparent);
  background: transparent;
}

.batch-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.9rem;
  color: var(--cream);
}

.batch-table th {
  text-align: left;
  padding: 0.65rem 0.55rem;
  background: transparent;
  color: var(--accent);
  font-weight: 700;
  white-space: nowrap;
  border-bottom: 1px solid color-mix(in srgb, var(--cream) 20%, transparent);
}

.batch-table td {
  padding: 0.45rem 0.4rem;
  vertical-align: middle;
  border-bottom: 1px solid color-mix(in srgb, var(--cream) 12%, transparent);
}

.batch-table tr.is-uploading {
  background: color-mix(in srgb, var(--accent) 12%, transparent);
}

.batch-table tr.is-error {
  background: color-mix(in srgb, #8b1e1e 18%, transparent);
}

.batch-table input,
.batch-table select {
  padding: 0.45rem 0.5rem;
  font-size: 0.85rem;
  border-radius: 0.45rem;
}

.batch-table .num {
  width: 4.5rem;
}

.batch-table .num input {
  min-width: 3.5rem;
}

.file-cell {
  max-width: 11rem;
}

.file-name {
  display: block;
  font-weight: 700;
  color: var(--cream);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.file-size {
  display: block;
  font-size: 0.75rem;
  font-weight: 600;
  color: color-mix(in srgb, var(--cream) 55%, transparent);
}

.status-cell .status-pill {
  margin: 0;
  max-width: 8rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.icon-btn {
  padding: 0.35rem 0.55rem;
  min-width: 2rem;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  border: 0;
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
  background: transparent;
  border: 1px solid color-mix(in srgb, var(--cream) 22%, transparent);
  border-radius: 0.35rem;
  color: var(--cream);
}

.list img,
.thumb-fallback {
  width: 64px;
  height: 96px;
  object-fit: cover;
  border-radius: 0.25rem;
  border: 1px solid color-mix(in srgb, var(--cream) 25%, transparent);
}

.thumb-fallback {
  display: grid;
  place-items: center;
  background: color-mix(in srgb, var(--accent) 55%, #121212);
  font-size: 1.5rem;
}

.list p {
  margin: 0.25rem 0 0;
  font-size: 0.9rem;
  color: color-mix(in srgb, var(--cream) 70%, transparent);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.list strong {
  font-family: var(--font-display);
  color: var(--cream);
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
  border-radius: 0.35rem;
  font-weight: 700;
  text-align: center;
  font-size: 0.85rem;
}

.actions a {
  background: var(--accent);
  color: var(--cream);
}

.edit {
  background: transparent;
  color: var(--cream);
  border: 1px solid color-mix(in srgb, var(--cream) 35%, transparent);
}

.danger {
  background: transparent;
  color: #8b1e1e;
  border: 1px solid #8b1e1e;
}

.state {
  padding: 1.25rem;
  background: transparent;
  border: 1px solid color-mix(in srgb, var(--cream) 25%, transparent);
  border-radius: 0.35rem;
  color: var(--cream);
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
