<script setup>
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { getAnimation } from '../api'

const props = defineProps({
  id: { type: String, required: true },
})

const film = ref(null)
const loading = ref(true)
const error = ref('')
const playError = ref('')
const videoEl = ref(null)
let pollTimer

const SEEK_SECONDS = 10

async function load() {
  loading.value = true
  error.value = ''
  playError.value = ''
  film.value = null
  try {
    film.value = await getAnimation(props.id)
    maybePoll()
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

function maybePoll() {
  clearInterval(pollTimer)
  if (film.value?.playback_status === 'processing') {
    pollTimer = setInterval(async () => {
      try {
        film.value = await getAnimation(props.id)
        if (film.value.playback_status !== 'processing') {
          clearInterval(pollTimer)
          playError.value = ''
        }
      } catch {
        /* ignore transient poll errors */
      }
    }, 4000)
  }
}

function onVideoError() {
  const status = film.value?.playback_status
  if (status === 'processing') {
    playError.value = 'Still converting this file for Firefox/Chrome… hang tight.'
    return
  }
  playError.value =
    'This browser cannot play the file (often MKV or HEVC). Prefer H.264 + AAC in an MP4 container — the Sunny will auto-convert unsupported uploads.'
}

function isTypingTarget(el) {
  if (!el || !(el instanceof Element)) return false
  const tag = el.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || el.isContentEditable
}

async function togglePlay() {
  const v = videoEl.value
  if (!v) return
  if (v.paused) {
    try {
      await v.play()
    } catch {
      /* autoplay policies / missing source */
    }
  } else {
    v.pause()
  }
}

function seekBy(delta) {
  const v = videoEl.value
  if (!v || !Number.isFinite(v.duration)) return
  v.currentTime = Math.min(Math.max(0, v.currentTime + delta), v.duration)
}

function bumpVolume(delta) {
  const v = videoEl.value
  if (!v) return
  v.volume = Math.min(1, Math.max(0, v.volume + delta))
}

async function toggleFullscreen() {
  const shell = videoEl.value?.closest('.player-shell')
  if (!shell) return
  if (document.fullscreenElement) {
    await document.exitFullscreen().catch(() => {})
  } else {
    await shell.requestFullscreen?.().catch(() => {})
  }
}

function onKeydown(e) {
  if (e.defaultPrevented || e.metaKey || e.ctrlKey || e.altKey) return
  if (isTypingTarget(e.target)) return
  if (!videoEl.value || film.value?.playback_status !== 'ready') return

  switch (e.key) {
    case ' ':
    case 'k':
    case 'K':
      e.preventDefault()
      togglePlay()
      break
    case 'ArrowLeft':
    case 'j':
    case 'J':
      e.preventDefault()
      seekBy(-SEEK_SECONDS)
      break
    case 'ArrowRight':
    case 'l':
    case 'L':
      e.preventDefault()
      seekBy(SEEK_SECONDS)
      break
    case 'ArrowUp':
      e.preventDefault()
      bumpVolume(0.1)
      break
    case 'ArrowDown':
      e.preventDefault()
      bumpVolume(-0.1)
      break
    case 'm':
    case 'M':
      e.preventDefault()
      videoEl.value.muted = !videoEl.value.muted
      break
    case 'f':
    case 'F':
      e.preventDefault()
      toggleFullscreen()
      break
    default:
      break
  }
}

onMounted(() => {
  load()
  window.addEventListener('keydown', onKeydown)
})
watch(() => props.id, load)
onUnmounted(() => {
  clearInterval(pollTimer)
  window.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <section class="watch">
    <div v-if="loading" class="state">Opening the den den projector…</div>
    <div v-else-if="error" class="state error">{{ error }}</div>
    <template v-else-if="film">
      <div v-if="film.playback_status === 'processing'" class="banner">
        Converting to browser-friendly MP4 (H.264). Large films can take a while — this page will
        unlock when ready. Track all jobs in
        <RouterLink to="/conversions">Den Den Workshop</RouterLink>.
      </div>
      <div v-else-if="film.playback_status === 'failed'" class="banner err">
        Conversion failed. Retry from
        <RouterLink to="/conversions">Den Den Workshop</RouterLink>, or re-upload an H.264 MP4.
      </div>
      <div class="player-shell">
        <video
          v-if="film.playback_status === 'ready'"
          ref="videoEl"
          :key="film.stream_url + film.playback_status"
          controls
          playsinline
          preload="metadata"
          :poster="film.poster_url"
          :src="film.stream_url"
          @error="onVideoError"
          @click="togglePlay"
        >
          Your browser does not support HTML5 video.
        </video>
        <div v-else class="waiting" :style="{ backgroundImage: `url(${film.poster_url})` }">
          <p v-if="film.playback_status === 'processing'">Preparing the den den projector…</p>
          <p v-else>Playback not ready.</p>
        </div>
      </div>
      <p v-if="playError" class="banner err">{{ playError }}</p>
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
        <p v-if="film.playback_status === 'ready'" class="shortcuts">
          <kbd>Space</kbd> play/pause · <kbd>←</kbd><kbd>→</kbd> ±10s · <kbd>↑</kbd><kbd>↓</kbd>
          volume · <kbd>M</kbd> mute · <kbd>F</kbd> fullscreen
        </p>
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

.banner {
  margin-bottom: 1rem;
  padding: 0.85rem 1rem;
  border-radius: 0.75rem;
  font-weight: 700;
  background: color-mix(in srgb, var(--sea-teal) 18%, var(--cream-sail));
  color: var(--sea-teal);
}

.banner.err {
  background: #fde8e8;
  color: #8b1e1e;
}

.player-shell {
  border-radius: 1rem;
  overflow: hidden;
  border: 3px solid var(--wood-brown);
  background: #111;
  box-shadow: 0 16px 40px var(--shadow);
}

video,
.waiting {
  display: block;
  width: 100%;
  max-height: min(70vh, 720px);
  min-height: 240px;
  background: #000;
}

video {
  cursor: pointer;
}

.waiting {
  display: grid;
  place-items: center;
  background-size: cover;
  background-position: center;
  color: var(--cream-sail);
  font-weight: 700;
  text-shadow: 0 2px 8px #000;
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

.shortcuts {
  margin: 0 0 1rem;
  font-size: 0.9rem;
  font-weight: 700;
  color: color-mix(in srgb, var(--ink) 70%, transparent);
}

.shortcuts kbd {
  display: inline-block;
  margin: 0 0.1rem;
  padding: 0.12rem 0.4rem;
  border: 1px solid color-mix(in srgb, var(--wood-brown) 35%, transparent);
  border-bottom-width: 2px;
  border-radius: 0.35rem;
  background: #fff;
  font: inherit;
  font-size: 0.8rem;
  color: var(--wood-brown);
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
