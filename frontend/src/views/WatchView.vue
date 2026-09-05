<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { getAnimation } from '../api'

const props = defineProps({
  id: { type: String, required: true },
})

const film = ref(null)
const loading = ref(true)
const error = ref('')
const playError = ref('')
const videoEl = ref(null)
const playing = ref(false)
const overlayVisible = ref(true)
const seekFlash = ref('') // 'back' | 'fwd' | ''
let pollTimer
let hideTimer
let flashTimer

const SEEK_SECONDS = 10

const playLabel = computed(() => (playing.value ? 'Pause' : 'Play'))

async function load() {
  loading.value = true
  error.value = ''
  playError.value = ''
  film.value = null
  playing.value = false
  try {
    film.value = await getAnimation(props.id)
    maybePoll()
    await nextTick()
    bindVideoEvents()
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
          await nextTick()
          bindVideoEvents()
        }
      } catch {
        /* ignore transient poll errors */
      }
    }, 4000)
  }
}

function bindVideoEvents() {
  const v = videoEl.value
  if (!v) return
  playing.value = !v.paused
  v.onplay = () => {
    playing.value = true
    scheduleHideOverlay()
  }
  v.onpause = () => {
    playing.value = false
    showOverlay(true)
  }
  v.onended = () => {
    playing.value = false
    showOverlay(true)
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

function showOverlay(sticky = false) {
  overlayVisible.value = true
  clearTimeout(hideTimer)
  if (!sticky && playing.value) scheduleHideOverlay()
}

function scheduleHideOverlay() {
  clearTimeout(hideTimer)
  hideTimer = setTimeout(() => {
    if (playing.value) overlayVisible.value = false
  }, 2200)
}

function flashSeek(dir) {
  seekFlash.value = dir
  clearTimeout(flashTimer)
  flashTimer = setTimeout(() => {
    seekFlash.value = ''
  }, 450)
}

async function togglePlay() {
  const v = videoEl.value
  if (!v) return
  showOverlay()
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
  flashSeek(delta < 0 ? 'back' : 'fwd')
  showOverlay()
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
  clearTimeout(hideTimer)
  clearTimeout(flashTimer)
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
      <div
        class="player-shell"
        @mousemove="showOverlay()"
        @mouseleave="playing && scheduleHideOverlay()"
      >
        <template v-if="film.playback_status === 'ready'">
          <video
            ref="videoEl"
            :key="film.stream_url + film.playback_status"
            controls
            playsinline
            preload="metadata"
            :poster="film.poster_url"
            :src="film.stream_url"
            @error="onVideoError"
          >
            Your browser does not support HTML5 video.
          </video>

          <div class="seek-flash" :class="{ show: seekFlash === 'back', back: true }" aria-hidden="true">
            −{{ SEEK_SECONDS }}s
          </div>
          <div class="seek-flash" :class="{ show: seekFlash === 'fwd', fwd: true }" aria-hidden="true">
            +{{ SEEK_SECONDS }}s
          </div>

          <div class="overlay" :class="{ visible: overlayVisible || !playing }">
            <button
              type="button"
              class="ctl"
              :aria-label="`Rewind ${SEEK_SECONDS} seconds`"
              @click.stop="seekBy(-SEEK_SECONDS)"
            >
              <span class="icon" aria-hidden="true">⟲</span>
              <span class="ctl-label">{{ SEEK_SECONDS }}</span>
            </button>
            <button
              type="button"
              class="ctl play"
              :aria-label="playLabel"
              @click.stop="togglePlay"
            >
              <span class="icon play-icon" aria-hidden="true">{{ playing ? '❚❚' : '▶' }}</span>
            </button>
            <button
              type="button"
              class="ctl"
              :aria-label="`Forward ${SEEK_SECONDS} seconds`"
              @click.stop="seekBy(SEEK_SECONDS)"
            >
              <span class="icon" aria-hidden="true">⟳</span>
              <span class="ctl-label">{{ SEEK_SECONDS }}</span>
            </button>
          </div>
        </template>
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
          On video: ←10s · play/pause · +10s · also
          <kbd>Space</kbd> <kbd>←</kbd><kbd>→</kbd> <kbd>M</kbd> <kbd>F</kbd>
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
  position: relative;
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

.waiting {
  display: grid;
  place-items: center;
  background-size: cover;
  background-position: center;
  color: var(--cream-sail);
  font-weight: 700;
  text-shadow: 0 2px 8px #000;
}

.overlay {
  position: absolute;
  inset: 0 0 3.2rem;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: clamp(0.75rem, 3vw, 1.75rem);
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.25s ease;
  background: radial-gradient(ellipse at center, rgba(0, 0, 0, 0.35), transparent 65%);
}

.overlay.visible {
  opacity: 1;
  pointer-events: auto;
}

.ctl {
  pointer-events: auto;
  display: grid;
  place-items: center;
  gap: 0.1rem;
  width: clamp(3.2rem, 8vw, 4.2rem);
  height: clamp(3.2rem, 8vw, 4.2rem);
  border: 2px solid rgba(255, 248, 231, 0.55);
  border-radius: 50%;
  background: rgba(42, 26, 16, 0.72);
  color: var(--cream-sail);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.35);
  transition: transform 0.15s ease, background 0.15s ease, border-color 0.15s ease;
}

.ctl:hover,
.ctl:focus-visible {
  transform: scale(1.06);
  background: rgba(232, 93, 4, 0.9);
  border-color: var(--sunny-yellow);
  outline: none;
}

.ctl.play {
  width: clamp(4rem, 10vw, 5.2rem);
  height: clamp(4rem, 10vw, 5.2rem);
  background: rgba(232, 93, 4, 0.88);
  border-color: var(--sunny-yellow);
}

.icon {
  font-size: clamp(1.1rem, 2.5vw, 1.45rem);
  line-height: 1;
}

.play-icon {
  font-size: clamp(1.25rem, 3vw, 1.7rem);
}

.ctl-label {
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.02em;
}

.seek-flash {
  position: absolute;
  top: 50%;
  transform: translateY(-50%) scale(0.9);
  padding: 0.55rem 0.9rem;
  border-radius: 999px;
  background: rgba(42, 26, 16, 0.75);
  color: var(--sunny-yellow);
  font-weight: 800;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.seek-flash.back {
  left: 12%;
}

.seek-flash.fwd {
  right: 12%;
}

.seek-flash.show {
  opacity: 1;
  transform: translateY(-50%) scale(1);
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

.player-shell:fullscreen,
.player-shell:-webkit-full-screen {
  border-radius: 0;
  border: none;
  max-height: 100vh;
}

.player-shell:fullscreen video,
.player-shell:-webkit-full-screen video {
  max-height: 100vh;
  height: 100%;
  object-fit: contain;
}
</style>
