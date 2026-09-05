<script setup>
import { onMounted, ref, watch } from 'vue'
import { getAnimation } from '../api'

const props = defineProps({
  id: { type: String, required: true },
})

const film = ref(null)
const loading = ref(true)
const error = ref('')

async function load() {
  loading.value = true
  error.value = ''
  film.value = null
  try {
    film.value = await getAnimation(props.id)
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(() => props.id, load)
</script>

<template>
  <section class="watch">
    <div v-if="loading" class="state">Opening the den den projector…</div>
    <div v-else-if="error" class="state error">{{ error }}</div>
    <template v-else-if="film">
      <div class="player-shell">
        <video
          :key="film.id"
          controls
          playsinline
          preload="metadata"
          :poster="film.poster_url"
          :src="film.stream_url"
        >
          Your browser does not support HTML5 video.
        </video>
      </div>
      <div class="details">
        <p class="eyebrow">Now sailing</p>
        <h1>{{ film.name }}</h1>
        <p v-if="film.series_name" class="series-line">
          <RouterLink
            v-if="film.series_id"
            class="series-link"
            :to="{ name: 'series', params: { id: film.series_id } }"
          >
            {{ film.series_name }}
          </RouterLink>
          <span v-if="film.season != null || film.episode != null">
            ·
            <template v-if="film.season != null">S{{ film.season }}</template>
            <template v-if="film.episode != null">E{{ film.episode }}</template>
          </span>
        </p>
        <p class="desc">{{ film.description || 'No description logged.' }}</p>
        <RouterLink
          v-if="film.series_id"
          class="back"
          :to="{ name: 'series', params: { id: film.series_id } }"
        >
          ← Back to {{ film.series_name }}
        </RouterLink>
        <RouterLink v-else class="back" to="/">← Back to the Sunny library</RouterLink>
      </div>
    </template>
  </section>
</template>

<style scoped>
.watch {
  padding: 0.5rem clamp(1rem, 4vw, 3rem) 3rem;
  animation: fadeRise 0.55s ease both;
}

.player-shell {
  border-radius: 1rem;
  overflow: hidden;
  border: 3px solid var(--wood-brown);
  background: #111;
  box-shadow: 0 16px 40px var(--shadow);
}

video {
  display: block;
  width: 100%;
  max-height: min(70vh, 720px);
  background: #000;
}

.details {
  margin-top: 1.5rem;
  max-width: 48rem;
  padding: 1.25rem 1.4rem;
  background: color-mix(in srgb, var(--cream-sail) 90%, transparent);
  border: 2px solid color-mix(in srgb, var(--wood-brown) 28%, transparent);
  border-radius: 1rem;
}

.eyebrow {
  margin: 0;
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--sea-teal);
}

.details h1 {
  margin: 0.35rem 0 0.75rem;
  font-family: var(--font-display);
  font-size: clamp(1.8rem, 4vw, 2.6rem);
  color: var(--wood-brown);
}

.desc {
  margin: 0 0 1rem;
  line-height: 1.55;
}

.series-line {
  margin: 0 0 0.75rem;
  font-weight: 700;
  color: var(--sea-teal);
}

.series-link {
  color: var(--sea-teal);
  text-decoration: underline;
  text-underline-offset: 2px;
}

.back {
  font-weight: 700;
  color: var(--ship-orange);
}

.state {
  padding: 2rem;
  background: color-mix(in srgb, var(--cream-sail) 88%, transparent);
  border-radius: 1rem;
}

.state.error {
  color: #8b1e1e;
}
</style>
