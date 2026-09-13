<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { getAnimation, getSeries } from '../api'
import {
  entryLabel,
  groupBySeason,
  isPlayable,
  neighbors,
  shortEntryLabel,
} from '../lib/seriesNav'

const props = defineProps({
  id: { type: String, required: true },
})

const router = useRouter()
const film = ref(null)
const series = ref(null)
const loading = ref(true)
const error = ref('')
const playError = ref('')
const videoEl = ref(null)
const playing = ref(false)
const overlayVisible = ref(true)
const seekFlash = ref('') // 'back' | 'fwd' | ''
const episodesOpen = ref(false)
const upNextVisible = ref(false)
const upNextSeconds = ref(0)
let pollTimer
let hideTimer
let flashTimer
let upNextTimer

const SEEK_SECONDS = 10
const UP_NEXT_COUNTDOWN = 8

const playLabel = computed(() => (playing.value ? 'Pause' : 'Play'))

const seriesNav = computed(() => {
  if (!film.value?.series_id || !series.value?.entries?.length) return null
  return neighbors(series.value.entries, film.value.id)
})

const prevEntry = computed(() => seriesNav.value?.prev || null)
const nextEntry = computed(() => seriesNav.value?.next || null)
const nextPlayable = computed(() => {
  const list = seriesNav.value?.list || []
  const i = seriesNav.value?.index ?? -1
  if (i < 0) return null
  for (let j = i + 1; j < list.length; j++) {
    if (isPlayable(list[j])) return list[j]
  }
  return null
})

const seasonGroups = computed(() => groupBySeason(series.value?.entries || []))

const episodePosition = computed(() => {
  const nav = seriesNav.value
  if (!nav || nav.index < 0) return null
  return `${nav.index + 1} / ${nav.list.length}`
})

async function load() {
  loading.value = true
  error.value = ''
  playError.value = ''
  film.value = null
  series.value = null
  playing.value = false
  cancelUpNext()
  try {
    film.value = await getAnimation(props.id)
    if (film.value.series_id) {
      try {
        series.value = await getSeries(film.value.series_id)
      } catch {
        series.value = null
      }
    }
    maybePoll()
    await nextTick()
    bindVideoEvents()
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

function goToEntry(entry) {
  if (!entry) return
  cancelUpNext()
  router.push({ name: 'watch', params: { id: entry.id } })
}

function cancelUpNext() {
  upNextVisible.value = false
  upNextSeconds.value = 0
  clearInterval(upNextTimer)
}

function startUpNextCountdown() {
  const next = nextPlayable.value
  if (!next) return
  upNextVisible.value = true
  upNextSeconds.value = UP_NEXT_COUNTDOWN
  clearInterval(upNextTimer)
  upNextTimer = setInterval(() => {
    upNextSeconds.value -= 1
    if (upNextSeconds.value <= 0) {
      clearInterval(upNextTimer)
      goToEntry(next)
    }
  }, 1000)
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
    startUpNextCountdown()
  }
}

function onVideoError() {
  const status = film.value?.playback_status
  if (status === 'processing') {
    playError.value = 'Still converting this file for Firefox/Chrome… hang tight.'
    return
  }
  playError.value =
    'This browser cannot play the file (often MKV or HEVC). Prefer H.264 + AAC in an MP4 container — Homeflix will auto-convert unsupported uploads.'
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
    case 'n':
    case 'N':
      if (nextEntry.value) {
        e.preventDefault()
        goToEntry(nextEntry.value)
      }
      break
    case 'p':
    case 'P':
      if (prevEntry.value) {
        e.preventDefault()
        goToEntry(prevEntry.value)
      }
      break
    case 'e':
    case 'E':
      if (series.value?.entries?.length) {
        e.preventDefault()
        episodesOpen.value = !episodesOpen.value
      }
      break
    case 'Escape':
      if (upNextVisible.value) {
        e.preventDefault()
        cancelUpNext()
      }
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
  cancelUpNext()
  window.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <section class="watch">
    <div v-if="loading" class="state">Loading player…</div>
    <div v-else-if="error" class="state error">{{ error }}</div>
    <template v-else-if="film">
      <div v-if="film.playback_status === 'processing'" class="banner">
        Converting to browser-friendly MP4 (H.264). Large films can take a while — this page will
        unlock when ready. Track all jobs in
        <RouterLink to="/conversions">Conversions</RouterLink>.
      </div>
      <div v-else-if="film.playback_status === 'failed'" class="banner err">
        Conversion failed. Retry from
        <RouterLink to="/conversions">Conversions</RouterLink>, or re-upload an H.264 MP4.
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

          <div v-if="upNextVisible && nextPlayable" class="up-next" role="dialog" aria-label="Up next">
            <p class="up-next-label">Up next in {{ upNextSeconds }}s</p>
            <strong>{{ shortEntryLabel(nextPlayable) }} · {{ nextPlayable.name }}</strong>
            <div class="up-next-actions">
              <button type="button" class="up-next-play" @click="goToEntry(nextPlayable)">
                Play now
              </button>
              <button type="button" class="up-next-cancel" @click="cancelUpNext">Cancel</button>
            </div>
          </div>
        </template>
        <div v-else class="waiting" :style="{ backgroundImage: `url(${film.poster_url})` }">
          <p v-if="film.playback_status === 'processing'">Preparing video…</p>
          <p v-else>Playback not ready.</p>
        </div>
      </div>
      <p v-if="playError" class="banner err">{{ playError }}</p>
      <div class="details">
        <p class="eyebrow">Now playing</p>
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
          <span v-if="episodePosition" class="ep-pos"> · {{ episodePosition }}</span>
        </p>
        <p class="desc">{{ film.description || 'No description logged.' }}</p>

        <nav v-if="seriesNav && seriesNav.list.length > 1" class="series-nav" aria-label="Series navigation">
          <button
            type="button"
            class="nav-btn"
            :disabled="!prevEntry"
            :title="prevEntry ? prevEntry.name : undefined"
            @click="goToEntry(prevEntry)"
          >
            <span aria-hidden="true">←</span>
            <span class="nav-copy">
              <span class="nav-kind">Previous</span>
              <span v-if="prevEntry" class="nav-title">
                {{ shortEntryLabel(prevEntry) }} · {{ prevEntry.name }}
              </span>
            </span>
          </button>
          <button
            type="button"
            class="nav-btn next"
            :disabled="!nextEntry"
            :title="nextEntry ? nextEntry.name : undefined"
            @click="goToEntry(nextEntry)"
          >
            <span class="nav-copy">
              <span class="nav-kind">Next</span>
              <span v-if="nextEntry" class="nav-title">
                {{ shortEntryLabel(nextEntry) }} · {{ nextEntry.name }}
              </span>
            </span>
            <span aria-hidden="true">→</span>
          </button>
        </nav>

        <details
          v-if="seasonGroups.length"
          class="episodes"
          :open="episodesOpen"
          @toggle="episodesOpen = $event.target.open"
        >
          <summary>Episodes <kbd>E</kbd></summary>
          <div v-for="group in seasonGroups" :key="group.key" class="ep-group">
            <h3>{{ group.label }}</h3>
            <ul>
              <li v-for="entry in group.entries" :key="entry.id">
                <button
                  type="button"
                  class="ep-row"
                  :class="{ current: entry.id === film.id, disabled: !isPlayable(entry) }"
                  :disabled="entry.id === film.id"
                  @click="goToEntry(entry)"
                >
                  <span v-if="entryLabel(entry)" class="ep-tag">{{ entryLabel(entry) }}</span>
                  <span class="ep-name">{{ entry.name }}</span>
                  <span v-if="entry.id === film.id" class="ep-now">Now</span>
                  <span v-else-if="!isPlayable(entry)" class="ep-status">{{ entry.playback_status }}</span>
                </button>
              </li>
            </ul>
          </div>
        </details>

        <p v-if="film.playback_status === 'ready'" class="shortcuts">
          On video: ←10s · play/pause · +10s · also
          <kbd>Space</kbd> <kbd>←</kbd><kbd>→</kbd> <kbd>M</kbd> <kbd>F</kbd>
          <template v-if="seriesNav && seriesNav.list.length > 1">
            · series <kbd>P</kbd> prev · <kbd>N</kbd> next · <kbd>E</kbd> episodes
          </template>
        </p>
        <RouterLink
          v-if="film.series_id"
          class="back"
          :to="{ name: 'series', params: { id: film.series_id } }"
        >
          ← Back to {{ film.series_name }}
        </RouterLink>
        <RouterLink v-else class="back" to="/">← Back to library</RouterLink>
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
  background: color-mix(in srgb, var(--teal) 18%, var(--cream));
  color: var(--teal);
}

.banner.err {
  background: #fde8e8;
  color: #8b1e1e;
}

.player-shell {
  position: relative;
  border-radius: 1rem;
  overflow: hidden;
  border: 3px solid var(--brown);
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
  color: var(--cream);
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
  color: var(--cream);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.35);
  transition: transform 0.15s ease, background 0.15s ease, border-color 0.15s ease;
}

.ctl:hover,
.ctl:focus-visible {
  transform: scale(1.06);
  background: rgba(232, 93, 4, 0.9);
  border-color: var(--accent-yellow);
  outline: none;
}

.ctl.play {
  width: clamp(4rem, 10vw, 5.2rem);
  height: clamp(4rem, 10vw, 5.2rem);
  background: rgba(232, 93, 4, 0.88);
  border-color: var(--accent-yellow);
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
  color: var(--accent-yellow);
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
  background: color-mix(in srgb, var(--cream) 90%, transparent);
  border: 2px solid color-mix(in srgb, var(--brown) 28%, transparent);
  border-radius: 1rem;
  color: var(--on-light);
}

.eyebrow {
  margin: 0;
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--teal);
}

.details h1 {
  margin: 0.35rem 0 0.75rem;
  font-family: var(--font-display);
  font-size: clamp(1.8rem, 4vw, 2.6rem);
  color: var(--brown);
}

.desc {
  margin: 0 0 1rem;
  line-height: 1.55;
}

.shortcuts {
  margin: 0 0 1rem;
  font-size: 0.9rem;
  font-weight: 700;
  color: color-mix(in srgb, var(--on-light) 70%, transparent);
}

.shortcuts kbd {
  display: inline-block;
  margin: 0 0.1rem;
  padding: 0.12rem 0.4rem;
  border: 1px solid color-mix(in srgb, var(--brown) 35%, transparent);
  border-bottom-width: 2px;
  border-radius: 0.35rem;
  background: #fff;
  font: inherit;
  font-size: 0.8rem;
  color: var(--brown);
}

.series-line {
  margin: 0 0 0.75rem;
  font-weight: 700;
  color: var(--teal);
}

.ep-pos {
  font-weight: 600;
  color: color-mix(in srgb, var(--on-light) 55%, transparent);
}

.series-link {
  color: var(--teal);
  text-decoration: underline;
  text-underline-offset: 2px;
}

.series-nav {
  display: grid;
  gap: 0.65rem;
  margin: 0 0 1.1rem;
}

@media (min-width: 640px) {
  .series-nav {
    grid-template-columns: 1fr 1fr;
  }
}

.nav-btn {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  min-height: 3.1rem;
  padding: 0.55rem 0.85rem;
  border: 2px solid color-mix(in srgb, var(--brown) 28%, transparent);
  border-radius: 0.75rem;
  background: #fff;
  color: var(--brown);
  text-align: left;
  transition: border-color 0.15s ease, transform 0.15s ease, background 0.15s ease;
}

.nav-btn.next {
  text-align: right;
  flex-direction: row-reverse;
}

.nav-btn:hover:not(:disabled),
.nav-btn:focus-visible:not(:disabled) {
  border-color: var(--accent);
  background: color-mix(in srgb, var(--accent-yellow) 22%, #fff);
  outline: none;
  transform: translateY(-1px);
}

.nav-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.nav-copy {
  display: grid;
  gap: 0.1rem;
  min-width: 0;
  flex: 1;
}

.nav-kind {
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--teal);
}

.nav-title {
  font-family: var(--font-display);
  font-weight: 700;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.episodes {
  margin: 0 0 1.1rem;
  border: 2px solid color-mix(in srgb, var(--brown) 22%, transparent);
  border-radius: 0.75rem;
  background: #fff;
  overflow: hidden;
}

.episodes summary {
  cursor: pointer;
  padding: 0.75rem 1rem;
  font-weight: 800;
  color: var(--brown);
  list-style: none;
}

.episodes summary::-webkit-details-marker {
  display: none;
}

.episodes summary::before {
  content: '▸ ';
  color: var(--accent);
}

.episodes[open] summary::before {
  content: '▾ ';
}

.episodes summary kbd {
  margin-left: 0.35rem;
  padding: 0.08rem 0.35rem;
  border: 1px solid color-mix(in srgb, var(--brown) 30%, transparent);
  border-radius: 0.3rem;
  font: inherit;
  font-size: 0.75rem;
  font-weight: 700;
}

.ep-group {
  padding: 0 0.75rem 0.85rem;
}

.ep-group h3 {
  margin: 0.35rem 0 0.45rem;
  font-size: 0.8rem;
  font-weight: 800;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--teal);
}

.ep-group ul {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.35rem;
}

.ep-row {
  width: 100%;
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.4rem 0.65rem;
  padding: 0.55rem 0.65rem;
  border: 1px solid transparent;
  border-radius: 0.5rem;
  background: color-mix(in srgb, var(--cream) 70%, #fff);
  color: var(--on-light);
  text-align: left;
}

.ep-row:hover:not(:disabled),
.ep-row:focus-visible:not(:disabled) {
  border-color: var(--accent);
  outline: none;
}

.ep-row.current {
  border-color: var(--teal);
  background: color-mix(in srgb, var(--teal) 12%, #fff);
}

.ep-row.disabled:not(.current) {
  opacity: 0.55;
}

.ep-tag {
  font-size: 0.75rem;
  font-weight: 800;
  color: var(--teal);
  flex-shrink: 0;
}

.ep-name {
  font-family: var(--font-display);
  font-weight: 700;
  flex: 1;
  min-width: 0;
}

.ep-now,
.ep-status {
  font-size: 0.72rem;
  font-weight: 800;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.ep-now {
  color: var(--accent);
}

.ep-status {
  color: color-mix(in srgb, var(--on-light) 55%, transparent);
}

.up-next {
  position: absolute;
  right: 1rem;
  bottom: 4.2rem;
  z-index: 3;
  max-width: min(22rem, calc(100% - 2rem));
  padding: 1rem 1.1rem;
  border-radius: 0.85rem;
  border: 2px solid var(--accent-yellow);
  background: rgba(42, 26, 16, 0.92);
  color: var(--cream);
  box-shadow: 0 12px 28px rgba(0, 0, 0, 0.4);
  animation: fadeRise 0.35s ease both;
}

.up-next-label {
  margin: 0 0 0.25rem;
  font-size: 0.75rem;
  font-weight: 800;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--accent-yellow);
}

.up-next strong {
  display: block;
  margin-bottom: 0.75rem;
  font-family: var(--font-display);
  font-size: 1.05rem;
  line-height: 1.3;
}

.up-next-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.up-next-play,
.up-next-cancel {
  padding: 0.45rem 0.85rem;
  border-radius: 0.5rem;
  font-weight: 800;
}

.up-next-play {
  border: none;
  background: var(--accent);
  color: var(--cream);
}

.up-next-cancel {
  border: 1px solid rgba(255, 248, 231, 0.45);
  background: transparent;
  color: var(--cream);
}

.back {
  font-weight: 700;
  color: var(--accent);
}

.state {
  padding: 2rem;
  background: color-mix(in srgb, var(--cream) 88%, transparent);
  border-radius: 1rem;
  color: var(--on-light);
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

.player-shell:fullscreen .up-next,
.player-shell:-webkit-full-screen .up-next {
  bottom: 5rem;
}
</style>
