<script setup>
import { ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { listLibrary } from '../api'

const router = useRouter()
const query = ref('')
const items = ref([])
const loading = ref(true)
const error = ref('')
let debounceTimer

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

function subtitle(item) {
  if (item.type === 'series') {
    const n = item.entry_count || 0
    const kind = item.kind === 'anime' ? 'anime' : item.kind === 'tv' ? 'show' : 'franchise'
    return `${n} ${n === 1 ? 'entry' : 'entries'} · ${kind}`
  }
  return item.description || 'Standalone film'
}
</script>

<template>
  <section class="home">
    <div class="hero">
      <p class="eyebrow">Pirate King Media Deck</p>
      <h1>Thousand Sunny</h1>
      <p class="tagline">Your private Grand Line of films — chart a Log Pose and set sail.</p>
      <label class="search">
        <span class="sr-only">Search by name or description</span>
        <input
          v-model="query"
          type="search"
          placeholder="Log Pose: search series, films, or descriptions…"
          autocomplete="off"
        />
      </label>
    </div>

    <div class="library">
      <div v-if="loading" class="state">Scanning the sea charts…</div>
      <div v-else-if="error" class="state error">{{ error }}</div>
      <div v-else-if="!items.length" class="state">
        No films aboard yet. Head to <RouterLink to="/admin">Galley-La</RouterLink> to load cargo.
      </div>
      <ul v-else class="grid">
        <li v-for="item in items" :key="`${item.type}-${item.id}`">
          <button class="card" type="button" @click="openItem(item)">
            <div class="poster-wrap">
              <img
                v-if="item.poster_url"
                :src="item.poster_url"
                :alt="item.name"
                loading="lazy"
              />
              <div v-else class="poster-fallback" aria-hidden="true">☀</div>
              <span v-if="item.type === 'series'" class="badge">Series</span>
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
  color: var(--wood-brown);
}

.hero h1 {
  margin: 0;
  font-family: var(--font-display);
  font-size: clamp(3rem, 10vw, 5.5rem);
  line-height: 0.95;
  color: var(--cream-sail);
  text-shadow: 0 4px 0 color-mix(in srgb, var(--wood-brown) 55%, transparent);
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
  border: 2px solid var(--wood-brown);
  border-radius: 999px;
  background: var(--foam);
  color: var(--ink);
  outline: none;
  box-shadow: 0 8px 24px var(--shadow);
  transition: box-shadow 0.25s ease, border-color 0.25s ease, transform 0.25s ease;
}

.search input:focus {
  border-color: var(--ship-orange);
  transform: translateY(-2px);
  box-shadow: 0 12px 28px var(--shadow);
}

.library {
  animation: fadeRise 0.8s ease 0.12s both;
}

.state {
  padding: 2rem;
  background: color-mix(in srgb, var(--cream-sail) 88%, transparent);
  border: 2px solid color-mix(in srgb, var(--wood-brown) 30%, transparent);
  border-radius: 1rem;
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
  border: 3px solid var(--wood-brown);
  background: var(--cream-sail);
  box-shadow: 0 10px 0 color-mix(in srgb, var(--wood-brown) 35%, transparent);
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
  background: linear-gradient(160deg, var(--sunny-yellow), var(--sea-teal));
  color: var(--wood-brown);
}

.card:hover .poster-wrap img {
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
  background: var(--ship-orange);
  color: var(--cream-sail);
}

.meta {
  padding: 0.75rem 0.15rem 0;
}

.meta h2 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 1.15rem;
  color: var(--cream-sail);
}

.meta p {
  margin: 0.35rem 0 0;
  font-size: 0.9rem;
  color: color-mix(in srgb, var(--cream-sail) 85%, var(--ink));
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
