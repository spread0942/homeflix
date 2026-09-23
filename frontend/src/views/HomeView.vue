<script setup>
import { onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { listContinueWatching, listLibrary } from '../api'
import { entryLabel } from '../lib/seriesNav'

const router = useRouter()
const query = ref('')
const items = ref([])
const continueItems = ref([])
const loading = ref(true)
const error = ref('')
let debounceTimer

async function loadContinue() {
  try {
    continueItems.value = await listContinueWatching()
  } catch {
    continueItems.value = []
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    items.value = await listLibrary(query.value)
  } catch (e) {
    error.value = e.message
    items.value = []
  } finally {
    loading.value = false
  }
}

onMounted(loadContinue)

watch(
  query,
  () => {
    clearTimeout(debounceTimer)
    debounceTimer = setTimeout(load, 280)
  },
  { immediate: true },
)

function openItem(item) {
  if (item.type === 'series') {
    router.push({ name: 'series', params: { id: item.id } })
  } else {
    router.push({ name: 'watch', params: { id: item.id } })
  }
}

function openContinue(item) {
  if (item.series_complete && item.series_id) {
    router.push({ name: 'series', params: { id: item.series_id } })
    return
  }
  router.push({ name: 'watch', params: { id: item.animation_id } })
}

function continueTitle(item) {
  return item.series_name || item.name
}

function continueSubtitle(item) {
  if (item.series_complete) return 'Series complete'
  const label = item.entry_label || entryLabel(item)
  if (item.series_name && label) return label
  if (item.series_name) return item.name
  return label || 'Continue watching'
}

function progressPercent(item) {
  if (item.series_complete) return 100
  if (!item.duration_seconds) return 0
  return Math.min(100, Math.max(0, (item.position_seconds / item.duration_seconds) * 100))
}

function subtitle(item) {
  if (item.type === 'series') {
    const n = item.entry_count || 0
    const kind = item.kind === 'anime' ? 'anime' : item.kind === 'tv' ? 'show' : 'franchise'
    return `${n} ${n === 1 ? 'entry' : 'entries'} · ${kind}`
  }
  return item.description || 'Standalone film'
}

function statusBadge(item) {
  if (item.completed) return { text: 'Completed', className: 'done' }
  if (item.type === 'film' && item.viewed) return { text: 'Viewed', className: 'viewed' }
  if (item.type === 'series') return { text: 'Series', className: '' }
  return null
}
</script>

<template>
  <section class="home">
    <div class="hero">
      <p class="eyebrow">Personal streaming</p>
      <h1>Homeflix</h1>
      <p class="tagline">Your private film library — search and watch at home.</p>
      <label class="search">
        <span class="sr-only">Search by name or description</span>
        <input
          v-model="query"
          type="search"
          placeholder="Search series, films, or descriptions…"
          autocomplete="off"
        />
      </label>
    </div>

    <section v-if="continueItems.length && !query" class="continue" aria-label="Continue watching">
      <h2>Continue watching</h2>
      <ul class="continue-row">
        <li v-for="item in continueItems" :key="`${item.series_id || 'film'}-${item.animation_id}`">
          <button class="continue-card" type="button" @click="openContinue(item)">
            <div class="poster-wrap">
              <img
                v-if="item.poster_url"
                :src="item.poster_url"
                :alt="continueTitle(item)"
                loading="lazy"
              />
              <div v-else class="poster-fallback" aria-hidden="true">▶</div>
              <span v-if="item.series_complete" class="badge done">Complete</span>
              <span v-else-if="item.series_id" class="badge">Series</span>
              <div
                v-if="!item.series_complete"
                class="progress-track"
                aria-hidden="true"
              >
                <div class="progress-fill" :style="{ width: `${progressPercent(item)}%` }" />
              </div>
            </div>
            <div class="meta">
              <h3>{{ continueTitle(item) }}</h3>
              <p>{{ continueSubtitle(item) }}</p>
            </div>
          </button>
        </li>
      </ul>
    </section>

    <div class="library">
      <div v-if="loading" class="state">Loading library…</div>
      <div v-else-if="error" class="state error">{{ error }}</div>
      <div v-else-if="!items.length" class="state">
        No films yet. Head to <RouterLink to="/admin">Admin</RouterLink> to upload.
      </div>
      <ul v-else class="grid">
        <li v-for="item in items" :key="`${item.type}-${item.id}`">
          <button
            class="card"
            :class="{ completed: item.completed }"
            type="button"
            @click="openItem(item)"
          >
            <div class="poster-wrap">
              <img
                v-if="item.poster_url"
                :src="item.poster_url"
                :alt="item.name"
                loading="lazy"
              />
              <div v-else class="poster-fallback" aria-hidden="true">▶</div>
              <span
                v-if="statusBadge(item)"
                class="badge"
                :class="statusBadge(item).className"
              >
                {{ statusBadge(item).text }}
              </span>
            </div>
            <div class="meta">
              <h2>{{ item.name }}</h2>
              <p>{{ subtitle(item) }}</p>
            </div>
          </button>
        </li>
      </ul>
    </div>
  </section>
</template>

<style scoped>
.home {
  padding: 0 clamp(1rem, 4vw, 3rem) 3rem;
}

.hero {
  max-width: 42rem;
  padding: 1rem 0 2.5rem;
  animation: fadeRise 0.7s ease both;
}

.eyebrow {
  margin: 0 0 0.5rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  font-size: 0.78rem;
  color: var(--brown);
}

.hero h1 {
  margin: 0;
  font-family: var(--font-display);
  font-size: clamp(3rem, 10vw, 5.5rem);
  line-height: 0.95;
  color: var(--cream);
  text-shadow: 0 4px 0 color-mix(in srgb, var(--brown) 55%, transparent);
}

.tagline {
  margin: 1rem 0 1.5rem;
  font-size: 1.15rem;
  max-width: 34rem;
  color: var(--ink);
}

.search {
  display: block;
}

.search input {
  width: min(100%, 34rem);
  padding: 0.9rem 1.15rem;
  border: 2px solid var(--brown);
  border-radius: 999px;
  background: var(--foam);
  color: var(--on-light);
  outline: none;
  box-shadow: 0 8px 24px var(--shadow);
  transition: box-shadow 0.25s ease, border-color 0.25s ease, transform 0.25s ease;
}

.search input:focus {
  border-color: var(--accent);
  transform: translateY(-2px);
  box-shadow: 0 12px 28px var(--shadow);
}

.continue {
  margin: 0 0 2rem;
  animation: fadeRise 0.75s ease 0.06s both;
}

.continue h2 {
  margin: 0 0 0.85rem;
  font-family: var(--font-display);
  font-size: 1.5rem;
  color: var(--cream);
}

.continue-row {
  list-style: none;
  margin: 0;
  padding: 0 0 0.35rem;
  display: flex;
  gap: 1rem;
  overflow-x: auto;
  scroll-snap-type: x mandatory;
}

.continue-row > li {
  flex: 0 0 min(160px, 42vw);
  scroll-snap-align: start;
}

.continue-card {
  width: 100%;
  padding: 0;
  border: none;
  background: transparent;
  text-align: left;
  color: inherit;
  transition: transform 0.25s ease;
}

.continue-card:hover,
.continue-card:focus-visible {
  transform: translateY(-4px);
  outline: none;
}

.library {
  animation: fadeRise 0.8s ease 0.12s both;
}

.state {
  padding: 2rem;
  background: color-mix(in srgb, var(--cream) 88%, transparent);
  border: 2px solid color-mix(in srgb, var(--brown) 30%, transparent);
  border-radius: 1rem;
  color: var(--on-light);
}

.state.error {
  color: #8b1e1e;
}

.grid {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 1.25rem;
}

.card {
  width: 100%;
  padding: 0;
  border: none;
  background: transparent;
  text-align: left;
  color: inherit;
  transition: transform 0.25s ease;
}

.card:hover,
.card:focus-visible {
  transform: translateY(-6px);
  outline: none;
}

.poster-wrap {
  position: relative;
  aspect-ratio: 2 / 3;
  overflow: hidden;
  border-radius: 0.85rem;
  border: 3px solid var(--brown);
  background: var(--cream);
  box-shadow: 0 10px 0 color-mix(in srgb, var(--brown) 35%, transparent);
}

.poster-wrap img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform 0.35s ease;
}

.poster-fallback {
  width: 100%;
  height: 100%;
  display: grid;
  place-items: center;
  font-size: 3rem;
  background: linear-gradient(160deg, var(--accent-yellow), var(--teal));
  color: var(--cream);
}

.card:hover .poster-wrap img,
.continue-card:hover .poster-wrap img {
  transform: scale(1.05);
}

.badge {
  position: absolute;
  top: 0.5rem;
  left: 0.5rem;
  padding: 0.2rem 0.55rem;
  border-radius: 999px;
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  background: var(--accent);
  color: var(--cream);
}

.badge.done {
  background: var(--teal);
}

.badge.viewed {
  background: color-mix(in srgb, var(--brown) 85%, #000);
}

.card.completed .poster-wrap {
  box-shadow: 0 10px 0 color-mix(in srgb, var(--teal) 40%, transparent);
}

.card.completed .poster-wrap::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(
    to top,
    color-mix(in srgb, var(--teal) 28%, transparent),
    transparent 42%
  );
  pointer-events: none;
}

.progress-track {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 4px;
  background: rgba(0, 0, 0, 0.45);
}

.progress-fill {
  height: 100%;
  background: var(--accent-yellow);
}

.meta {
  padding: 0.75rem 0.15rem 0;
}

.meta h2,
.meta h3 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 1.15rem;
  color: var(--cream);
}

.meta h3 {
  font-size: 1.05rem;
}

.meta p {
  margin: 0.35rem 0 0;
  font-size: 0.9rem;
  color: color-mix(in srgb, var(--cream) 85%, var(--ink));
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
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
</style>
