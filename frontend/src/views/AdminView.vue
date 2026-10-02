<script setup>
import { computed, nextTick, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import ConversionsPanel from '../components/ConversionsPanel.vue'
import {
  createAnimation,
  createSeries,
  deleteAnimation,
  deleteAnimationSubtitle,
  deleteSeries,
  addAnimationSubtitles,
  getAnimation,
  listAnimations,
  listSeries,
  updateAnimation,
  updateSeries,
} from '../api'
import { entryLabel, groupBySeason } from '../lib/seriesNav'

const route = useRoute()
const router = useRouter()
const section = computed(() => {
  if (route.name === 'admin-series') return 'series'
  if (route.name === 'admin-conversions') return 'conversions'
  if (route.name === 'admin-upload') return 'upload'
  return 'films'
})

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
const editingSubtitles = ref([])
const pendingSubtitles = ref([]) // { key, file, lang, label }
const addSubtitleLang = ref('it')
const addSubtitleInput = ref(null)
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

const inventoryQuery = ref('')
const inventorySeries = ref('')
const inventoryStatus = ref('')
const inventorySubs = ref('')

const inventoryFiltersActive = computed(
  () =>
    Boolean(
      inventoryQuery.value.trim() ||
        inventorySeries.value ||
        inventoryStatus.value ||
        inventorySubs.value,
    ),
)

const inventorySeriesOptions = computed(() => {
  const options = seriesList.value.map((s) => ({
    value: s.id,
    label: `${s.name} (${s.entry_count ?? 0})`,
  }))
  if (films.value.some((f) => !f.series_id)) {
    options.push({ value: 'standalone', label: 'Standalone' })
  }
  return options
})

const filteredFilms = computed(() => {
  const q = inventoryQuery.value.trim().toLowerCase()
  const series = inventorySeries.value
  const status = inventoryStatus.value
  const subs = inventorySubs.value

  return films.value.filter((film) => {
    if (q) {
      const hay = `${film.name || ''} ${film.series_name || ''} ${film.description || ''}`.toLowerCase()
      if (!hay.includes(q)) return false
    }
    if (series === 'standalone') {
      if (film.series_id) return false
    } else if (series && film.series_id !== series) {
      return false
    }
    if (status && (film.playback_status || 'ready') !== status) return false
    if (subs === 'has' && !film.subtitles?.length) return false
    if (subs === 'none' && film.subtitles?.length) return false
    return true
  })
})

function clearInventoryFilters() {
  inventoryQuery.value = ''
  inventorySeries.value = ''
  inventoryStatus.value = ''
  inventorySubs.value = ''
}

function sortInventoryFilms(list) {
  return [...list].sort((a, b) => {
    const sa = a.season == null ? Number.POSITIVE_INFINITY : a.season
    const sb = b.season == null ? Number.POSITIVE_INFINITY : b.season
    if (sa !== sb) return sa - sb
    const ea = a.episode == null ? Number.POSITIVE_INFINITY : a.episode
    const eb = b.episode == null ? Number.POSITIVE_INFINITY : b.episode
    if (ea !== eb) return ea - eb
    const oa = a.sort_order || 0
    const ob = b.sort_order || 0
    if (oa !== ob) return oa - ob
    return (a.name || '').localeCompare(b.name || '')
  })
}

/** Series → season → films for the admin inventory. */
const filmsGrouped = computed(() => {
  const map = new Map()
  for (const film of filteredFilms.value) {
    const key = film.series_id || 'standalone'
    if (!map.has(key)) {
      map.set(key, {
        key,
        name: film.series_name || 'Standalone',
        films: [],
      })
    }
    map.get(key).films.push(film)
  }

  const groups = [...map.values()].map((g) => {
    const seasons = groupBySeason(sortInventoryFilms(g.films))
    const showSeasons =
      seasons.length > 1 || (seasons.length === 1 && seasons[0].season != null)
    return {
      key: g.key,
      name: g.name,
      count: g.films.length,
      showSeasons,
      seasons,
    }
  })

  groups.sort((a, b) => {
    if (a.key === 'standalone') return 1
    if (b.key === 'standalone') return -1
    return a.name.localeCompare(b.name)
  })
  return groups
})

async function refresh({ silent = false } = {}) {
  if (!silent) loading.value = true
  if (!silent) error.value = ''
  try {
    const [a, s] = await Promise.all([listAnimations(), listSeries()])
    films.value = a
    seriesList.value = s
  } catch (e) {
    error.value = e.message
  } finally {
    if (!silent) loading.value = false
  }
}

/** Replace one film in the inventory without remounting the whole list. */
function upsertFilm(film) {
  if (!film?.id) return
  const i = films.value.findIndex((f) => f.id === film.id)
  if (i >= 0) films.value.splice(i, 1, film)
  else films.value = [film, ...films.value]
}

async function refreshSeriesQuietly() {
  try {
    seriesList.value = await listSeries()
  } catch {
    /* keep existing series list */
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
    subtitleFiles: [], // { file, lang, label }
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

function fileBaseName(filename) {
  return (filename || '').replace(/\.[^.]+$/, '').toLowerCase()
}

function isSubtitleFile(file) {
  const name = (file?.name || '').toLowerCase()
  return name.endsWith('.srt') || name.endsWith('.vtt')
}

const LANG_TAGS = {
  en: { code: 'en', label: 'English' },
  eng: { code: 'en', label: 'English' },
  english: { code: 'en', label: 'English' },
  it: { code: 'it', label: 'Italian' },
  ita: { code: 'it', label: 'Italian' },
  italian: { code: 'it', label: 'Italian' },
  es: { code: 'es', label: 'Spanish' },
  spa: { code: 'es', label: 'Spanish' },
  fr: { code: 'fr', label: 'French' },
  fre: { code: 'fr', label: 'French' },
  fra: { code: 'fr', label: 'French' },
  de: { code: 'de', label: 'German' },
  ger: { code: 'de', label: 'German' },
  deu: { code: 'de', label: 'German' },
  pt: { code: 'pt', label: 'Portuguese' },
  ja: { code: 'ja', label: 'Japanese' },
  jpn: { code: 'ja', label: 'Japanese' },
  zh: { code: 'zh', label: 'Chinese' },
  ko: { code: 'ko', label: 'Korean' },
  kor: { code: 'ko', label: 'Korean' },
  ru: { code: 'ru', label: 'Russian' },
  rus: { code: 'ru', label: 'Russian' },
}

const SUBTITLE_LANG_OPTIONS = [
  { code: 'it', label: 'Italian' },
  { code: 'en', label: 'English' },
  { code: 'es', label: 'Spanish' },
  { code: 'fr', label: 'French' },
  { code: 'de', label: 'German' },
  { code: 'pt', label: 'Portuguese' },
  { code: 'ja', label: 'Japanese' },
  { code: 'zh', label: 'Chinese' },
  { code: 'ko', label: 'Korean' },
  { code: 'ru', label: 'Russian' },
  { code: 'und', label: 'Other / unknown' },
]

function parseSubtitleFilename(filename) {
  const stem = fileBaseName(filename)
  const dot = stem.lastIndexOf('.')
  if (dot > 0) {
    const tag = stem.slice(dot + 1)
    const info = LANG_TAGS[tag]
    if (info) return { base: stem.slice(0, dot), lang: info.code, label: info.label }
  }
  const under = stem.lastIndexOf('_')
  if (under > 0) {
    const tag = stem.slice(under + 1)
    const info = LANG_TAGS[tag]
    if (info) return { base: stem.slice(0, under), lang: info.code, label: info.label }
  }
  return { base: stem, lang: 'und', label: 'Subtitles' }
}

function onBatchFiles(e) {
  const files = [...(e.target.files || [])]
  if (!files.length) return
  editingFilmId.value = null

  const videos = files.filter((f) => !isSubtitleFile(f))
  const subs = files.filter(isSubtitleFile)
  const rows = videos.map(newBatchRow)

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

  const pool = [...batchItems.value]
  let matched = 0
  for (const sub of subs) {
    const parsed = parseSubtitleFilename(sub.name)
    const row = pool.find(
      (r) =>
        (r.status === 'pending' || r.status === 'error') &&
        fileBaseName(r.file.name) === parsed.base,
    )
    if (row) {
      const existingIdx = row.subtitleFiles.findIndex((s) => s.lang === parsed.lang)
      const entry = { file: sub, lang: parsed.lang, label: parsed.label }
      if (existingIdx >= 0) row.subtitleFiles[existingIdx] = entry
      else row.subtitleFiles.push(entry)
      matched += 1
    }
  }

  if (batchFileInput.value) batchFileInput.value.value = ''
  success.value = ''
  error.value = ''
  if (!videos.length && subs.length && !matched) {
    error.value =
      'No matching videos for those subtitle files (use Movie.it.srt / Movie.en.srt).'
  } else if (subs.length && matched) {
    success.value = `Matched ${matched} subtitle file${matched === 1 ? '' : 's'} by name.`
  }
}

function onBatchSubtitleChange(row, e) {
  const file = e.target.files?.[0] || null
  e.target.value = ''
  if (!file) return
  if (!isSubtitleFile(file)) {
    error.value = 'Subtitle must be .srt or .vtt'
    return
  }
  const parsed = parseSubtitleFilename(file.name)
  const existingIdx = row.subtitleFiles.findIndex((s) => s.lang === parsed.lang)
  const entry = { file, lang: parsed.lang, label: parsed.label }
  if (existingIdx >= 0) row.subtitleFiles[existingIdx] = entry
  else row.subtitleFiles.push(entry)
}

function clearBatchSubtitle(row, lang) {
  row.subtitleFiles = row.subtitleFiles.filter((s) => s.lang !== lang)
}

function setBatchSubtitleLang(row, oldLang, newLang) {
  const entry = row.subtitleFiles.find((s) => s.lang === oldLang)
  if (!entry) return
  const info = SUBTITLE_LANG_OPTIONS.find((o) => o.code === newLang)
  entry.lang = newLang
  entry.label = info?.label || newLang.toUpperCase()
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

function subtitleLangLabel(code) {
  return SUBTITLE_LANG_OPTIONS.find((o) => o.code === code)?.label || code || 'Subtitles'
}

function onAddSubtitleFileChange(e) {
  const file = e.target.files?.[0] || null
  if (addSubtitleInput.value) addSubtitleInput.value.value = ''
  if (!file) return

  const parsed = parseSubtitleFilename(file.name)
  const lang = parsed.lang !== 'und' ? parsed.lang : addSubtitleLang.value || 'und'
  const label = subtitleLangLabel(lang)
  const existing = pendingSubtitles.value.findIndex((s) => s.lang === lang)
  const entry = {
    key: `${lang}-${file.name}-${Date.now()}`,
    file,
    lang,
    label,
  }
  if (existing >= 0) pendingSubtitles.value[existing] = entry
  else pendingSubtitles.value.push(entry)

  const used = new Set([
    ...editingSubtitles.value.map((t) => t.language),
    ...pendingSubtitles.value.map((s) => s.lang),
  ])
  const next = SUBTITLE_LANG_OPTIONS.find((o) => o.code !== 'und' && !used.has(o.code))
  addSubtitleLang.value = next?.code || 'und'
}

function removePendingSubtitle(key) {
  pendingSubtitles.value = pendingSubtitles.value.filter((s) => s.key !== key)
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
  editingSubtitles.value = []
  pendingSubtitles.value = []
  addSubtitleLang.value = 'it'
  if (addSubtitleInput.value) addSubtitleInput.value.value = ''
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

function setFilmFormEl(el) {
  filmFormEl.value = el || null
}

function startEditFilm(film) {
  if (route.name !== 'admin-films') router.push({ name: 'admin-films' })
  editingFilmId.value = film.id
  name.value = film.name || ''
  description.value = film.description || ''
  seriesId.value = film.series_id || ''
  season.value = film.season != null ? String(film.season) : ''
  episode.value = film.episode != null ? String(film.episode) : ''
  sortOrder.value = film.sort_order != null ? String(film.sort_order) : '0'
  videoFile.value = null
  posterFile.value = null
  editingSubtitles.value = [...(film.subtitles || [])]
  pendingSubtitles.value = []
  addSubtitleLang.value = 'it'
  if (addSubtitleInput.value) addSubtitleInput.value.value = ''
  success.value = ''
  error.value = ''
  nextTick(() => {
    document
      .querySelector('.film-item.editing')
      ?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
  })
}

function cancelEditFilm() {
  resetFilmForm(seriesId.value)
  success.value = ''
  error.value = ''
}

function startEditSeries(s) {
  if (route.name !== 'admin-series') router.push({ name: 'admin-series' })
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

function buildAnimationForm(fields, { video, poster, subtitles } = {}) {
  const form = new FormData()
  form.append('name', fields.name.trim())
  form.append('description', (fields.description || '').trim())
  form.append('series_id', fields.seriesId || '')
  form.append('season', fields.season !== '' && fields.season != null ? fields.season : '')
  form.append('episode', fields.episode !== '' && fields.episode != null ? fields.episode : '')
  form.append('sort_order', fields.sortOrder || '0')
  if (video) form.append('video', video)
  if (poster) form.append('poster', poster)
  for (const sub of subtitles || []) {
    form.append('subtitle', sub.file)
    form.append('subtitle_lang', sub.lang || 'und')
  }
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
    const filmId = editingFilmId.value
    let updated = await updateAnimation(filmId, form)
    if (pendingSubtitles.value.length) {
      const subForm = new FormData()
      for (const sub of pendingSubtitles.value) {
        subForm.append('subtitle', sub.file)
        subForm.append('subtitle_lang', sub.lang || 'und')
      }
      await addAnimationSubtitles(filmId, subForm)
      updated = await getAnimation(filmId)
    }
    upsertFilm(updated)
    await refreshSeriesQuietly()
    success.value = `"${name.value.trim()}" updated.`
    resetFilmForm(seriesId.value)
  } catch (err) {
    error.value = err.message
  } finally {
    submitting.value = false
  }
}

async function removeEditingSubtitle(track) {
  if (!editingFilmId.value || !track?.id) return
  if (!confirm(`Remove ${track.label || track.language} subtitles?`)) return
  error.value = ''
  try {
    const filmId = editingFilmId.value
    await deleteAnimationSubtitle(filmId, track.id)
    editingSubtitles.value = editingSubtitles.value.filter((t) => t.id !== track.id)
    const current = films.value.find((f) => f.id === filmId)
    if (current) {
      upsertFilm({
        ...current,
        subtitles: (current.subtitles || []).filter((t) => t.id !== track.id),
      })
    }
    success.value = `Removed ${track.label || track.language} subtitles.`
  } catch (err) {
    error.value = err.message
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
        { video: row.file, subtitles: row.subtitleFiles },
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
    films.value = films.value.filter((f) => f.id !== id)
    await refreshSeriesQuietly()
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

function filmRowMeta(film) {
  return entryLabel(film) || film.description || '—'
}

function subtitleLangs(film) {
  const tracks = film.subtitles || []
  if (!tracks.length) return ''
  return tracks
    .map((t) => {
      const code = String(t.language || '').trim()
      if (code && code !== 'und') return code.toUpperCase()
      return String(t.label || 'UND').toUpperCase()
    })
    .join(' · ')
}

function subtitleTitle(film) {
  const tracks = film.subtitles || []
  if (!tracks.length) return 'No subtitles'
  const detail = tracks.map((t) => t.label || t.language || 'und').join(', ')
  return `Subtitles: ${detail}`
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
    <nav class="tabs" aria-label="Admin sections">
      <RouterLink class="tab" :to="{ name: 'admin-films' }">Films</RouterLink>
      <RouterLink class="tab" :to="{ name: 'admin-upload' }">Upload</RouterLink>
      <RouterLink class="tab" :to="{ name: 'admin-series' }">Series</RouterLink>
      <RouterLink class="tab" :to="{ name: 'admin-conversions' }">Conversions</RouterLink>
    </nav>

    <p v-if="section !== 'conversions' && success" class="banner ok">{{ success }}</p>
    <p v-if="section !== 'conversions' && error" class="banner err">{{ error }}</p>

    <div v-if="section === 'films'" class="panel">
      <div class="inventory inventory-standalone">
        <h2>
          All films
          <span v-if="films.length" class="inventory-count">
            <template v-if="inventoryFiltersActive">
              {{ filteredFilms.length }} / {{ films.length }}
            </template>
            <template v-else>{{ films.length }}</template>
          </span>
        </h2>

        <div v-if="films.length" class="inventory-filters">
          <label class="inv-search">
            <span class="sr-only">Search films</span>
            <input
              v-model="inventoryQuery"
              type="search"
              placeholder="Search titles…"
              autocomplete="off"
            />
          </label>
          <label>
            <span class="sr-only">Series</span>
            <select v-model="inventorySeries">
              <option value="">All series</option>
              <option
                v-for="opt in inventorySeriesOptions"
                :key="opt.value"
                :value="opt.value"
              >
                {{ opt.label }}
              </option>
            </select>
          </label>
          <label>
            <span class="sr-only">Status</span>
            <select v-model="inventoryStatus">
              <option value="">All statuses</option>
              <option value="ready">Ready</option>
              <option value="processing">Processing</option>
              <option value="failed">Failed</option>
            </select>
          </label>
          <label>
            <span class="sr-only">Subtitles</span>
            <select v-model="inventorySubs">
              <option value="">Any subtitles</option>
              <option value="has">Has CC</option>
              <option value="none">No CC</option>
            </select>
          </label>
          <button
            v-if="inventoryFiltersActive"
            type="button"
            class="inv-clear"
            @click="clearInventoryFilters"
          >
            Clear
          </button>
        </div>

        <div v-if="loading" class="state">Loading…</div>
        <div v-else-if="filmsGrouped.length" class="film-groups">
          <details
            v-for="group in filmsGrouped"
            :key="group.key"
            class="film-group"
            open
          >
            <summary class="film-group-summary">
              <span class="film-group-name">{{ group.name }}</span>
              <span class="film-group-count">
                <template v-if="group.showSeasons"
                  >{{ group.seasons.length }} season{{
                    group.seasons.length === 1 ? '' : 's'
                  }}
                  · </template
                >{{ group.count }} episode{{ group.count === 1 ? '' : 's' }}
              </span>
            </summary>
            <div class="film-group-body">
              <component
                :is="group.showSeasons ? 'details' : 'div'"
                v-for="seasonGroup in group.seasons"
                :key="seasonGroup.key"
                class="season-group"
                v-bind="group.showSeasons ? { open: true } : {}"
              >
                <summary v-if="group.showSeasons" class="season-summary">
                  <span class="season-heading">{{ seasonGroup.label }}</span>
                  <span class="season-count">
                    {{ seasonGroup.entries.length }}
                    episode{{ seasonGroup.entries.length === 1 ? '' : 's' }}
                  </span>
                </summary>
                <ul class="list">
                  <li
                    v-for="film in seasonGroup.entries"
                    :key="film.id"
                    class="film-item"
                    :class="{ editing: editingFilmId === film.id }"
                  >
                    <div class="film-row">
                      <img :src="film.poster_url" :alt="film.name" />
                      <div>
                        <strong>{{ film.name }}</strong>
                        <p>
                          <span class="status-pill" :class="film.playback_status || 'ready'">{{
                            film.playback_status || 'ready'
                          }}</span>
                          <span
                            class="sub-badge"
                            :class="film.subtitles?.length ? 'has' : 'none'"
                            :title="subtitleTitle(film)"
                          >
                            <svg
                              class="sub-icon"
                              viewBox="0 0 24 24"
                              width="14"
                              height="14"
                              aria-hidden="true"
                            >
                              <rect
                                x="2"
                                y="4"
                                width="20"
                                height="16"
                                rx="2"
                                fill="none"
                                stroke="currentColor"
                                stroke-width="2"
                              />
                              <path
                                d="M5 15h4M11 15h8M5 11h8M15 11h4"
                                fill="none"
                                stroke="currentColor"
                                stroke-width="2"
                                stroke-linecap="round"
                              />
                            </svg>
                            <template v-if="film.subtitles?.length">
                              Subs
                              <span class="sub-langs">{{ subtitleLangs(film) }}</span>
                            </template>
                            <template v-else>No subs</template>
                          </span>
                          {{ filmRowMeta(film) }}
                        </p>
                      </div>
                      <div class="actions">
                        <button
                          type="button"
                          class="edit"
                          @click="
                            editingFilmId === film.id ? cancelEditFilm() : startEditFilm(film)
                          "
                        >
                          {{ editingFilmId === film.id ? 'Close' : 'Edit' }}
                        </button>
                        <RouterLink :to="{ name: 'watch', params: { id: film.id } }">Watch</RouterLink>
                        <button
                          type="button"
                          class="danger"
                          @click="removeFilm(film.id, film.name)"
                        >
                          Delete
                        </button>
                      </div>
                    </div>

                    <form
                      v-if="editingFilmId === film.id"
                      :ref="(el) => setFilmFormEl(el)"
                      class="form inline-edit"
                      @submit="onSubmit"
                    >
                      <h3 class="inline-edit-title">Edit film / episode</h3>
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
                        Movies in a franchise: leave season/episode empty and use sort (1, 2…).
                        Anime/TV: set season + episode.
                      </p>
                      <label>
                        New poster (optional)
                        <input type="file" accept="image/*" @change="onPosterChange" />
                        <span class="field-note">Leave empty to keep the current poster</span>
                      </label>

                      <div class="sub-edit">
                        <h3 class="sub-edit-title">Subtitles</h3>
                        <ul v-if="editingSubtitles.length" class="sub-track-list">
                          <li v-for="track in editingSubtitles" :key="track.id">
                            <span class="sub-track-label">{{ track.label || track.language }}</span>
                            <span class="sub-track-code">{{ track.language }}</span>
                            <button
                              type="button"
                              class="danger icon-btn"
                              :aria-label="`Remove ${track.label || track.language}`"
                              @click="removeEditingSubtitle(track)"
                            >
                              ✕
                            </button>
                          </li>
                        </ul>
                        <p v-else class="field-note">No subtitle tracks yet.</p>

                        <ul v-if="pendingSubtitles.length" class="sub-track-list pending">
                          <li v-for="sub in pendingSubtitles" :key="sub.key">
                            <span class="sub-track-label">
                              {{ sub.label }}
                              <span class="pending-file" :title="sub.file.name">{{
                                sub.file.name
                              }}</span>
                            </span>
                            <span class="sub-track-code">{{ sub.lang }}</span>
                            <button
                              type="button"
                              class="danger icon-btn"
                              :aria-label="`Remove pending ${sub.label}`"
                              @click="removePendingSubtitle(sub.key)"
                            >
                              ✕
                            </button>
                          </li>
                        </ul>

                        <div class="sub-add-row">
                          <label>
                            Language
                            <select v-model="addSubtitleLang">
                              <option
                                v-for="opt in SUBTITLE_LANG_OPTIONS"
                                :key="opt.code"
                                :value="opt.code"
                              >
                                {{ opt.label }}
                              </option>
                            </select>
                          </label>
                          <label>
                            Add .srt / .vtt
                            <input
                              ref="addSubtitleInput"
                              type="file"
                              accept=".srt,.vtt,text/vtt"
                              @change="onAddSubtitleFileChange"
                            />
                          </label>
                        </div>
                        <p class="field-note">
                          Pick a language and file to queue it. Repeat for more languages, then
                          save.
                        </p>
                      </div>

                      <div class="form-actions">
                        <button class="submit" type="submit" :disabled="submitting">
                          {{ submitting ? 'Saving…' : 'Save changes' }}
                        </button>
                        <button
                          type="button"
                          class="cancel"
                          :disabled="submitting"
                          @click="cancelEditFilm"
                        >
                          Cancel
                        </button>
                      </div>
                    </form>
                  </li>
                </ul>
              </component>
            </div>
          </details>
        </div>
        <div v-else-if="inventoryFiltersActive" class="state">
          No films match these filters.
          <button type="button" class="inv-clear inline" @click="clearInventoryFilters">
            Clear filters
          </button>
        </div>
        <div v-else class="state">No films yet.</div>
      </div>
    </div>

    <div v-else-if="section === 'upload'" class="panel">
      <div class="batch form-wide">
        <div class="batch-head">
          <h2>Upload films / episodes</h2>
          <p class="hint">
            Select videos and optional .srt/.vtt files. Name them like
            <code>Movie.it.srt</code> and <code>Movie.en.srt</code> to attach multiple languages.
          </p>
        </div>

        <div class="batch-toolbar">
          <label class="file-pick">
            Add videos / subtitles
            <input
              ref="batchFileInput"
              type="file"
              accept="video/*,.srt,.vtt,text/vtt"
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
                <th scope="col">Subtitles</th>
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
                <td class="sub-cell">
                  <div class="sub-multi">
                    <div
                      v-for="sub in row.subtitleFiles"
                      :key="sub.lang + sub.file.name"
                      class="sub-chip"
                    >
                      <select
                        :value="sub.lang"
                        :disabled="row.status === 'uploading' || row.status === 'done'"
                        :aria-label="`Language for ${sub.file.name}`"
                        @change="setBatchSubtitleLang(row, sub.lang, $event.target.value)"
                      >
                        <option
                          v-for="opt in SUBTITLE_LANG_OPTIONS"
                          :key="opt.code"
                          :value="opt.code"
                        >
                          {{ opt.label }}
                        </option>
                      </select>
                      <span class="sub-chip-name" :title="sub.file.name">{{ sub.file.name }}</span>
                      <button
                        type="button"
                        class="danger icon-btn"
                        :disabled="row.status === 'uploading' || row.status === 'done'"
                        :aria-label="`Clear ${sub.label} subtitle`"
                        @click="clearBatchSubtitle(row, sub.lang)"
                      >
                        ✕
                      </button>
                    </div>
                    <label class="sub-file">
                      <span class="sub-label">Add .srt</span>
                      <input
                        type="file"
                        accept=".srt,.vtt,text/vtt"
                        :disabled="row.status === 'uploading' || row.status === 'done'"
                        :aria-label="`Add subtitle for ${row.file.name}`"
                        @change="onBatchSubtitleChange(row, $event)"
                      />
                    </label>
                  </div>
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
    </div>

    <div v-else-if="section === 'series'" class="panel">
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

    <ConversionsPanel v-else />
  </section>
</template>

<style scoped>
.admin {
  padding: 0.5rem clamp(1rem, 4vw, 3rem) 3rem;
  animation: fadeRise 0.55s ease both;
}

.tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-bottom: 1.25rem;
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

.status-pill.cc {
  background: transparent;
  border: 1px solid color-mix(in srgb, var(--cream) 45%, transparent);
  color: var(--cream);
  letter-spacing: 0.06em;
}

.sub-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  margin-right: 0.4rem;
  padding: 0.12rem 0.45rem;
  border-radius: 0.25rem;
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.02em;
  vertical-align: middle;
  white-space: nowrap;
}

.sub-badge .sub-icon {
  flex-shrink: 0;
}

.sub-badge.has {
  border: 1px solid color-mix(in srgb, var(--accent) 55%, transparent);
  color: var(--accent);
  background: color-mix(in srgb, var(--accent) 12%, transparent);
}

.sub-badge.none {
  border: 1px solid color-mix(in srgb, var(--cream) 18%, transparent);
  color: color-mix(in srgb, var(--cream) 40%, transparent);
}

.sub-badge .sub-langs {
  font-weight: 800;
  opacity: 0.95;
}

.check-row {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  font-weight: 700;
  color: color-mix(in srgb, var(--cream) 85%, transparent);
}

.check-row input[type='checkbox'] {
  width: auto;
  margin: 0;
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

.sub-cell {
  max-width: 14rem;
  min-width: 10rem;
}

.sub-multi {
  display: grid;
  gap: 0.35rem;
}

.sub-chip {
  display: grid;
  grid-template-columns: 5.5rem 1fr auto;
  gap: 0.3rem;
  align-items: center;
}

.sub-chip select {
  padding: 0.3rem 0.35rem;
  font-size: 0.75rem;
}

.sub-chip-name {
  font-size: 0.72rem;
  font-weight: 600;
  color: color-mix(in srgb, var(--cream) 70%, transparent);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.sub-pick {
  display: flex;
  align-items: center;
  gap: 0.35rem;
}

.sub-file {
  position: relative;
  display: block;
  min-width: 0;
  cursor: pointer;
}

.sub-file input {
  position: absolute;
  inset: 0;
  opacity: 0;
  cursor: pointer;
}

.sub-label {
  display: block;
  padding: 0.45rem 0.55rem;
  border: 1px solid color-mix(in srgb, var(--cream) 28%, transparent);
  border-radius: 0.3rem;
  font-size: 0.78rem;
  font-weight: 700;
  color: color-mix(in srgb, var(--cream) 80%, transparent);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  text-align: center;
}

.sub-file:hover .sub-label {
  border-color: var(--accent);
  color: var(--accent);
}

.sub-edit {
  display: grid;
  gap: 0.65rem;
  padding: 0.85rem;
  border: 1px solid color-mix(in srgb, var(--cream) 22%, transparent);
  border-radius: 0.35rem;
}

.sub-edit-title {
  margin: 0;
  font-size: 0.95rem;
  color: var(--cream);
}

.sub-track-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.4rem;
}

.sub-track-list li {
  display: grid;
  grid-template-columns: 1fr auto auto;
  gap: 0.5rem;
  align-items: center;
  padding: 0.4rem 0.55rem;
  border: 1px solid color-mix(in srgb, var(--cream) 20%, transparent);
  border-radius: 0.3rem;
}

.sub-track-label {
  font-weight: 700;
  color: var(--cream);
  min-width: 0;
  display: grid;
  gap: 0.15rem;
}

.pending-file {
  display: block;
  font-size: 0.75rem;
  font-weight: 600;
  color: color-mix(in srgb, var(--cream) 55%, transparent);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.sub-track-list.pending li {
  border-color: color-mix(in srgb, var(--accent) 40%, transparent);
  background: color-mix(in srgb, var(--accent) 8%, transparent);
}

.sub-track-code {
  font-size: 0.75rem;
  font-weight: 700;
  color: var(--accent);
  text-transform: uppercase;
}

.sub-add-row {
  display: grid;
  grid-template-columns: 1fr 1.4fr;
  gap: 0.75rem;
}

@media (max-width: 640px) {
  .sub-add-row {
    grid-template-columns: 1fr;
  }
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

.inventory-standalone {
  margin-top: 0;
}

.inventory h2 {
  margin: 0 0 1rem;
  font-family: var(--font-display);
  color: var(--cream);
  display: flex;
  align-items: baseline;
  gap: 0.55rem;
}

.inventory-count {
  font-family: var(--font-body);
  font-size: 0.85rem;
  font-weight: 600;
  color: color-mix(in srgb, var(--cream) 55%, transparent);
}

.inventory-filters {
  display: grid;
  gap: 0.65rem;
  margin-bottom: 1rem;
  align-items: end;
}

@media (min-width: 720px) {
  .inventory-filters {
    grid-template-columns: minmax(10rem, 1.4fr) repeat(3, minmax(7rem, 1fr)) auto;
  }
}

.inventory-filters label {
  margin: 0;
  font-weight: 600;
}

.inventory-filters .inv-search {
  min-width: 0;
}

.inventory-filters input,
.inventory-filters select {
  padding: 0.55rem 0.7rem;
  font-weight: 600;
}

.inv-clear {
  padding: 0.55rem 0.85rem;
  border: 1px solid color-mix(in srgb, var(--cream) 35%, transparent);
  border-radius: 0.35rem;
  background: transparent;
  color: var(--cream);
  font-weight: 700;
  white-space: nowrap;
}

.inv-clear:hover {
  border-color: var(--accent);
  color: var(--accent);
}

.inv-clear.inline {
  display: inline-block;
  margin-left: 0.65rem;
  padding: 0.25rem 0.6rem;
  font-size: 0.85rem;
  vertical-align: baseline;
}

.film-groups {
  display: grid;
  gap: 0.85rem;
}

.film-group {
  border: 1px solid color-mix(in srgb, var(--cream) 22%, transparent);
  border-radius: 0.35rem;
  overflow: hidden;
}

.film-group-summary {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.75rem 1rem;
  cursor: pointer;
  list-style: none;
  background: color-mix(in srgb, var(--cream) 6%, transparent);
  color: var(--cream);
  user-select: none;
}

.film-group-summary::-webkit-details-marker {
  display: none;
}

.film-group-summary::before {
  content: '▸';
  flex-shrink: 0;
  font-size: 0.75rem;
  color: color-mix(in srgb, var(--cream) 55%, transparent);
  transition: transform 0.15s ease;
}

.film-group[open] > .film-group-summary::before {
  transform: rotate(90deg);
}

.film-group-name {
  font-family: var(--font-display);
  font-weight: 700;
  font-size: 1.05rem;
}

.film-group-count {
  margin-left: auto;
  font-size: 0.8rem;
  font-weight: 600;
  color: color-mix(in srgb, var(--cream) 55%, transparent);
}

.film-group-body {
  display: grid;
  gap: 0.65rem;
  padding: 0.75rem;
  border-top: 1px solid color-mix(in srgb, var(--cream) 14%, transparent);
}

.season-group {
  border: 1px solid color-mix(in srgb, var(--cream) 14%, transparent);
  border-radius: 0.3rem;
  overflow: hidden;
}

.season-group > .list {
  padding: 0.65rem;
}

details.season-group > .list {
  border-top: 1px solid color-mix(in srgb, var(--cream) 12%, transparent);
}

.season-summary {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  padding: 0.55rem 0.75rem;
  cursor: pointer;
  list-style: none;
  user-select: none;
  background: color-mix(in srgb, var(--cream) 4%, transparent);
}

.season-summary::-webkit-details-marker {
  display: none;
}

.season-summary::before {
  content: '▸';
  flex-shrink: 0;
  font-size: 0.7rem;
  color: color-mix(in srgb, var(--cream) 50%, transparent);
  transition: transform 0.15s ease;
}

.season-group[open] > .season-summary::before {
  transform: rotate(90deg);
}

.season-heading {
  margin: 0;
  font-family: var(--font-display);
  font-size: 0.85rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: color-mix(in srgb, var(--cream) 70%, transparent);
}

.season-count {
  margin-left: auto;
  font-size: 0.78rem;
  font-weight: 600;
  color: color-mix(in srgb, var(--cream) 50%, transparent);
}

.list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.85rem;
}

.list > li {
  padding: 0.75rem;
  background: transparent;
  border: 1px solid color-mix(in srgb, var(--cream) 22%, transparent);
  border-radius: 0.35rem;
  color: var(--cream);
}

.list > li:not(.film-item) {
  display: grid;
  grid-template-columns: 64px 1fr auto;
  gap: 0.9rem;
  align-items: center;
}

.film-item {
  display: grid;
  gap: 0.75rem;
}

.film-item.editing {
  border-color: color-mix(in srgb, var(--accent) 50%, transparent);
  background: color-mix(in srgb, var(--accent) 6%, transparent);
}

.film-row {
  display: grid;
  grid-template-columns: 64px 1fr auto;
  gap: 0.9rem;
  align-items: center;
}

.inline-edit {
  margin: 0;
  max-width: none;
  padding: 1rem;
  border: 1px solid color-mix(in srgb, var(--cream) 18%, transparent);
  border-radius: 0.3rem;
  background: color-mix(in srgb, var(--cream) 4%, transparent);
}

.inline-edit-title {
  margin: 0;
  font-family: var(--font-display);
  font-size: 1.05rem;
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
  .list > li:not(.film-item),
  .film-row {
    grid-template-columns: 56px 1fr;
  }

  .list > li:not(.film-item) .actions,
  .film-row .actions {
    grid-column: 1 / -1;
    flex-direction: row;
  }
}
</style>
