<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { getSeries } from '../api'
import { entryLabel, groupBySeason, isPlayable } from '../lib/seriesNav'

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

const seasons = computed(() => groupBySeason(series.value?.entries || []))

const selectedSeasonKey = ref('')

watch(
  seasons,
  (groups) => {
    if (!groups.length) {
      selectedSeasonKey.value = ''
      return
    }
    if (!groups.some((g) => g.key === selectedSeasonKey.value)) {
      selectedSeasonKey.value = groups[0].key
    }
  },
  { immediate: true },
)

const activeSeason = computed(
  () => seasons.value.find((g) => g.key === selectedSeasonKey.value) || seasons.value[0] || null,
)

const firstPlayable = computed(() => {
  const inSeason = activeSeason.value?.entries || []
  return inSeason.find(isPlayable) || inSeason[0] || null
})

function openFilm(id) {
  router.push({ name: 'watch', params: { id } })
}

function playSeries() {
  if (firstPlayable.value) openFilm(firstPlayable.value.id)
}
</script>

<template>
  <section class="series">
    <div v-if="loading" class="state">Loading series…</div>
    <div v-else-if="error" class="state error">{{ error }}</div>
    <template v-else-if="series">
      <header class="hero">
        <div class="poster">
          <img v-if="series.poster_url" :src="series.poster_url" :alt="series.name" />
          <div v-else class="poster-fallback" aria-hidden="true">▶</div>
        </div>
        <div class="copy">
          <p class="eyebrow">{{ series.kind }} · {{ series.entry_count }} entries</p>
          <h1>{{ series.name }}</h1>
          <p class="desc">{{ series.description || 'No description yet.' }}</p>
          <div class="hero-actions">
            <button
              v-if="firstPlayable"
              type="button"
              class="play-cta"
              @click="playSeries"
            >
              ▶ Play {{ entryLabel(firstPlayable) || firstPlayable.name }}
            </button>
            <RouterLink class="back" to="/">← Back to library</RouterLink>
          </div>
        </div>
      </header>

      <div v-if="!series.entries?.length" class="state">
        No entries yet. Add films to this series in <RouterLink to="/admin">Admin</RouterLink>.
      </div>

      <template v-else-if="activeSeason">
        <div class="season-bar">
          <label v-if="seasons.length > 1" class="season-pick">
            Season
            <select v-model="selectedSeasonKey" aria-label="Select season">
              <option v-for="group in seasons" :key="group.key" :value="group.key">
                {{ group.label }} ({{ group.entries.length }})
              </option>
            </select>
          </label>
          <h2 v-else>{{ activeSeason.label }}</h2>
        </div>
        <ul class="list">
          <li v-for="entry in activeSeason.entries" :key="entry.id">
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
      </template>
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
  border: 3px solid var(--brown);
  box-shadow: 0 10px 0 color-mix(in srgb, var(--brown) 35%, transparent);
  background: var(--cream);
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
  background: linear-gradient(160deg, var(--accent-yellow), var(--teal));
}

.eyebrow {
  margin: 0;
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--brown);
}

.copy h1 {
  margin: 0.35rem 0 0.75rem;
  font-family: var(--font-display);
  font-size: clamp(2rem, 5vw, 3.2rem);
  color: var(--cream);
  text-shadow: 0 3px 0 color-mix(in srgb, var(--brown) 50%, transparent);
}

.desc {
  margin: 0 0 1rem;
  max-width: 40rem;
  color: var(--ink);
  line-height: 1.5;
}

.hero-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.85rem 1.25rem;
}

.play-cta {
  padding: 0.65rem 1.15rem;
  border: none;
  border-radius: 999px;
  background: var(--accent);
  color: var(--cream);
  font-weight: 800;
  box-shadow: 0 4px 0 color-mix(in srgb, var(--accent-yellow) 55%, transparent);
  transition: transform 0.15s ease, background 0.15s ease;
}

.play-cta:hover,
.play-cta:focus-visible {
  transform: translateY(-1px);
  background: color-mix(in srgb, var(--accent) 85%, #000);
  outline: none;
}

.back {
  font-weight: 700;
  color: var(--accent);
}

.season {
  margin-bottom: 2rem;
}

.season h2,
.season-bar h2 {
  margin: 0 0 0.85rem;
  font-family: var(--font-display);
  color: var(--cream);
}

.season-bar {
  margin-bottom: 0.85rem;
}

.season-pick {
  display: grid;
  gap: 0.4rem;
  max-width: 18rem;
  font-weight: 700;
  color: var(--cream);
}

.season-pick select {
  width: 100%;
  padding: 0.65rem 0.85rem;
  border: 2px solid color-mix(in srgb, var(--brown) 40%, transparent);
  border-radius: 0.65rem;
  background: color-mix(in srgb, var(--cream) 92%, transparent);
  color: var(--on-light);
  font-weight: 700;
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
  border: 2px solid color-mix(in srgb, var(--brown) 25%, transparent);
  border-radius: 0.85rem;
  background: color-mix(in srgb, var(--cream) 90%, transparent);
  text-align: left;
  color: var(--on-light);
  transition: transform 0.2s ease, border-color 0.2s ease;
}

.row:hover {
  transform: translateX(4px);
  border-color: var(--accent);
}

.row img {
  width: 56px;
  height: 84px;
  object-fit: cover;
  border-radius: 0.35rem;
  border: 2px solid var(--brown);
}

.ep {
  display: inline-block;
  margin-bottom: 0.2rem;
  font-size: 0.75rem;
  font-weight: 700;
  color: var(--teal);
}

.meta strong {
  display: block;
  font-family: var(--font-display);
  color: var(--brown);
}

.meta p {
  margin: 0.25rem 0 0;
  font-size: 0.9rem;
  color: color-mix(in srgb, var(--on-light) 70%, transparent);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.state {
  padding: 1.5rem;
  background: color-mix(in srgb, var(--cream) 88%, transparent);
  border-radius: 1rem;
  color: var(--on-light);
}

.state.error {
  color: #8b1e1e;
}
</style>
