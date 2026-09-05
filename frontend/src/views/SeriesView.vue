<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { getSeries } from '../api'

const props = defineProps({
  id: { type: String, required: true },
})

const router = useRouter()
const series = ref(null)
const loading = ref(true)
const error = ref('')

async function load() {
  loading.value = true
  error.value = ''
  series.value = null
  try {
    series.value = await getSeries(props.id)
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(() => props.id, load)

const seasons = computed(() => {
  const entries = series.value?.entries || []
  const map = new Map()
  for (const e of entries) {
    const key = e.season == null ? 'parts' : `season-${e.season}`
    if (!map.has(key)) {
      map.set(key, {
        key,
        label: e.season == null ? 'Parts & specials' : `Season ${e.season}`,
        season: e.season,
        entries: [],
      })
    }
    map.get(key).entries.push(e)
  }
  return [...map.values()].sort((a, b) => {
    if (a.season == null && b.season == null) return 0
    if (a.season == null) return 1
    if (b.season == null) return -1
    return a.season - b.season
  })
})

function entryLabel(e) {
  const bits = []
  if (e.season != null) bits.push(`S${e.season}`)
  if (e.episode != null) bits.push(`E${e.episode}`)
  if (!bits.length && e.sort_order) bits.push(`Part ${e.sort_order}`)
  return bits.length ? bits.join(' · ') : null
}

function openFilm(id) {
  router.push({ name: 'watch', params: { id } })
}
</script>

<template>
  <section class="series">
    <div v-if="loading" class="state">Unrolling the sea chart…</div>
    <div v-else-if="error" class="state error">{{ error }}</div>
    <template v-else-if="series">
      <header class="hero">
        <div class="poster">
          <img v-if="series.poster_url" :src="series.poster_url" :alt="series.name" />
          <div v-else class="poster-fallback" aria-hidden="true">☀</div>
        </div>
        <div class="copy">
          <p class="eyebrow">{{ series.kind }} · {{ series.entry_count }} entries</p>
          <h1>{{ series.name }}</h1>
          <p class="desc">{{ series.description || 'No log entry for this series yet.' }}</p>
          <RouterLink class="back" to="/">← Back to library</RouterLink>
        </div>
      </header>

      <div v-if="!series.entries?.length" class="state">
        No entries yet. Add films to this series in <RouterLink to="/admin">Galley-La</RouterLink>.
      </div>

      <div v-for="group in seasons" :key="group.key" class="season">
        <h2>{{ group.label }}</h2>
        <ul class="list">
          <li v-for="entry in group.entries" :key="entry.id">
            <button type="button" class="row" @click="openFilm(entry.id)">
              <img :src="entry.poster_url" :alt="entry.name" />
              <div class="meta">
                <span v-if="entryLabel(entry)" class="ep">{{ entryLabel(entry) }}</span>
                <strong>{{ entry.name }}</strong>
                <p>{{ entry.description || '—' }}</p>
              </div>
            </button>
          </li>
        </ul>
      </div>
    </template>
  </section>
</template>

<style scoped>
.series {
  padding: 0.5rem clamp(1rem, 4vw, 3rem) 3rem;
  animation: fadeRise 0.55s ease both;
}

.hero {
  display: grid;
  gap: 1.25rem;
  margin-bottom: 2rem;
  align-items: end;
}

@media (min-width: 720px) {
  .hero {
    grid-template-columns: 180px 1fr;
  }
}

.poster {
  aspect-ratio: 2 / 3;
  max-width: 180px;
  overflow: hidden;
  border-radius: 0.85rem;
  border: 3px solid var(--wood-brown);
  box-shadow: 0 10px 0 color-mix(in srgb, var(--wood-brown) 35%, transparent);
  background: var(--cream-sail);
}

.poster img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.poster-fallback {
  width: 100%;
  height: 100%;
  display: grid;
  place-items: center;
  font-size: 3rem;
  background: linear-gradient(160deg, var(--sunny-yellow), var(--sea-teal));
}

.eyebrow {
  margin: 0;
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--wood-brown);
}

.copy h1 {
  margin: 0.35rem 0 0.75rem;
  font-family: var(--font-display);
  font-size: clamp(2rem, 5vw, 3.2rem);
  color: var(--cream-sail);
  text-shadow: 0 3px 0 color-mix(in srgb, var(--wood-brown) 50%, transparent);
}

.desc {
  margin: 0 0 1rem;
  max-width: 40rem;
  color: var(--ink);
  line-height: 1.5;
}

.back {
  font-weight: 700;
  color: var(--ship-orange);
}

.season {
  margin-bottom: 2rem;
}

.season h2 {
  margin: 0 0 0.85rem;
  font-family: var(--font-display);
  color: var(--cream-sail);
}

.list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.75rem;
}

.row {
  width: 100%;
  display: grid;
  grid-template-columns: 56px 1fr;
  gap: 0.85rem;
  align-items: center;
  padding: 0.65rem;
  border: 2px solid color-mix(in srgb, var(--wood-brown) 25%, transparent);
  border-radius: 0.85rem;
  background: color-mix(in srgb, var(--cream-sail) 90%, transparent);
  text-align: left;
  color: inherit;
  transition: transform 0.2s ease, border-color 0.2s ease;
}

.row:hover {
  transform: translateX(4px);
  border-color: var(--ship-orange);
}

.row img {
  width: 56px;
  height: 84px;
  object-fit: cover;
  border-radius: 0.35rem;
  border: 2px solid var(--wood-brown);
}

.ep {
  display: inline-block;
  margin-bottom: 0.2rem;
  font-size: 0.75rem;
  font-weight: 700;
  color: var(--sea-teal);
}

.meta strong {
  display: block;
  font-family: var(--font-display);
  color: var(--wood-brown);
}

.meta p {
  margin: 0.25rem 0 0;
  font-size: 0.9rem;
  color: color-mix(in srgb, var(--ink) 75%, transparent);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.state {
  padding: 1.5rem;
  background: color-mix(in srgb, var(--cream-sail) 88%, transparent);
  border-radius: 1rem;
}

.state.error {
  color: #8b1e1e;
}
</style>
