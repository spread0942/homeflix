<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { getAnimation, getProgress, getSeries, putProgress } from '../api'
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
const seriesCompleteVisible = ref(false)
const currentTime = ref(0)
const duration = ref(0)
const resumePosition = ref(0)
const progressHint = ref(null) // { position_seconds, duration_seconds, completed }
const isPlayStation = /PlayStation/i.test(
  typeof navigator !== 'undefined' ? navigator.userAgent : '',
)
let pollTimer
let hideTimer
let flashTimer
let upNextTimer
let lastProgressSave = 0
let resumeApplied = false
let lastGoodPosition = 0
let lastGoodDuration = 0
let progressFilmId = ''

const SEEK_SECONDS = 10
const SEEK_JUMPS = [30, 60]
const backJumps = [60, 30]
const UP_NEXT_COUNTDOWN = 8
const PROGRESS_SAVE_MS = 10000
const RESUME_MIN_SECONDS = 5

const playLabel = computed(() => (playing.value ? 'Pause' : 'Play'))
const progressPercent = computed(() => {
  if (!duration.value) return 0
  return Math.min(100, Math.max(0, (currentTime.value / duration.value) * 100))
})

function formatTime(seconds) {
  if (!Number.isFinite(seconds) || seconds < 0) return '0:00'
  const s = Math.floor(seconds)
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const r = s % 60
  if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(r).padStart(2, '0')}`
  return `${m}:${String(r).padStart(2, '0')}`
}

function formatJump(seconds) {
  if (seconds >= 60 && seconds % 60 === 0) return `${seconds / 60}m`
  return `${seconds}s`
}

function mediaDuration() {
  const v = videoEl.value
  if (!v) return 0
  if (Number.isFinite(v.duration) && v.duration > 0) return v.duration
  try {
    if (v.seekable && v.seekable.length > 0) {
      const end = v.seekable.end(v.seekable.length - 1)
      if (Number.isFinite(end) && end > 0) return end
    }
  } catch {
    /* ignore */
  }
  return duration.value || lastGoodDuration || 0
}

function readPlaybackClock() {
  const v = videoEl.value
  const livePos = v && Number.isFinite(v.currentTime) ? v.currentTime : 0
  const liveDur = mediaDuration()
  // Prefer live clock when it looks real; otherwise keep last good values
  // so unmount/pagehide cannot wipe progress with a reset video element.
  const pos =
    livePos >= RESUME_MIN_SECONDS
      ? livePos
      : Math.max(livePos, lastGoodPosition)
  const dur = liveDur > 0 ? liveDur : lastGoodDuration
  if (livePos >= RESUME_MIN_SECONDS) lastGoodPosition = livePos
  if (liveDur > 0) lastGoodDuration = liveDur
  return { pos, dur }
}

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

const selectedSeasonKey = ref('')

watch(
  [seasonGroups, film],
  ([groups, current]) => {
    if (!groups.length) {
      selectedSeasonKey.value = ''
      return
    }
    if (current) {
      const key = current.season == null ? 'parts' : `season-${current.season}`
      if (groups.some((g) => g.key === key)) {
        selectedSeasonKey.value = key
        return
      }
    }
    if (!groups.some((g) => g.key === selectedSeasonKey.value)) {
      selectedSeasonKey.value = groups[0].key
    }
  },
  { immediate: true },
)

const activeSeason = computed(
  () =>
    seasonGroups.value.find((g) => g.key === selectedSeasonKey.value) ||
    seasonGroups.value[0] ||
    null,
)

const episodePosition = computed(() => {
  const nav = seriesNav.value
  if (!nav || nav.index < 0) return null
  return `${nav.index + 1} / ${nav.list.length}`
})

async function load() {
  if (progressFilmId) {
    await saveProgress({ force: true, filmId: progressFilmId })
  }
  loading.value = true
  error.value = ''
  playError.value = ''
  film.value = null
  series.value = null
  playing.value = false
  currentTime.value = 0
  duration.value = 0
  resumePosition.value = 0
  progressHint.value = null
  resumeApplied = false
  lastProgressSave = 0
  lastGoodPosition = 0
  lastGoodDuration = 0
  progressFilmId = ''
  cancelUpNext()
  seriesCompleteVisible.value = false
  try {
    film.value = await getAnimation(props.id)
    progressFilmId = film.value.id
    if (film.value.series_id) {
      try {
        series.value = await getSeries(film.value.series_id)
      } catch {
        series.value = null
      }
    }
    try {
      const p = await getProgress(props.id)
      progressHint.value = p
      if (!p.completed && p.position_seconds >= RESUME_MIN_SECONDS) {
        resumePosition.value = p.position_seconds
        lastGoodPosition = p.position_seconds
        lastGoodDuration = p.duration_seconds || 0
      }
    } catch {
      /* no saved progress */
    }
    maybePoll()
    await nextTick()
    syncVideoState()
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

function goToEntry(entry) {
  if (!entry) return
  cancelUpNext()
  seriesCompleteVisible.value = false
  router.push({ name: 'watch', params: { id: entry.id } })
}

function cancelUpNext() {
  upNextVisible.value = false
  upNextSeconds.value = 0
  clearInterval(upNextTimer)
}

function startUpNextCountdown() {
  const next = nextPlayable.value
  if (!next) {
    if (film.value?.series_id) {
      seriesCompleteVisible.value = true
    }
    return
  }
  seriesCompleteVisible.value = false
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

function dismissSeriesComplete() {
  seriesCompleteVisible.value = false
}

async function saveProgress({ completed = false, force = false, filmId = '' } = {}) {
  const id = filmId || film.value?.id || progressFilmId
  if (!id) return
  if (film.value && film.value.id === id && film.value.playback_status !== 'ready') return

  const { pos, dur } = readPlaybackClock()
  // Never persist a near-zero scrub — it wipes resume points on leave/remount.
  if (!completed && pos < RESUME_MIN_SECONDS) return

  const now = Date.now()
  if (!force && !completed && now - lastProgressSave < PROGRESS_SAVE_MS) return
  lastProgressSave = now
  try {
    const saved = await putProgress(id, {
      position_seconds: completed ? Math.max(dur || pos, pos) : pos,
      duration_seconds: dur,
      completed,
    })
    if (!filmId || filmId === film.value?.id) {
      progressHint.value = saved
    }
  } catch {
    /* ignore transient save errors */
  }
}

function applyResumeSeek() {
  if (resumeApplied) return
  const v = videoEl.value
  const pos = resumePosition.value
  if (!v || !(pos >= RESUME_MIN_SECONDS)) return
  const dur = Number.isFinite(v.duration) ? v.duration : 0
  // Only skip resume when the saved point is past the end (already finished).
  if (dur > 0 && pos >= dur - 1) return
  try {
    v.currentTime = pos
    currentTime.value = pos
    lastGoodPosition = pos
    resumeApplied = true
  } catch {
    /* seek may fail until more data is buffered */
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
          syncVideoState()
        }
      } catch {
        /* ignore transient poll errors */
      }
    }, 4000)
  }
}

function syncVideoState() {
  const v = videoEl.value
  if (!v) return
  playing.value = !v.paused
  currentTime.value = v.currentTime || 0
  duration.value = Number.isFinite(v.duration) ? v.duration : 0
  if (duration.value > 0) lastGoodDuration = duration.value
  if (currentTime.value >= RESUME_MIN_SECONDS) lastGoodPosition = currentTime.value
  if (v.readyState >= 1) applyResumeSeek()
}

function onVideoPlay() {
  playing.value = true
  if (!isPlayStation) scheduleHideOverlay()
}

function onVideoPause() {
  playing.value = false
  showOverlay(true)
  saveProgress({ force: true })
}

function onVideoEnded() {
  playing.value = false
  showOverlay(true)
  saveProgress({ completed: true, force: true })
  startUpNextCountdown()
}

function onVideoTimeUpdate(e) {
  const v = e?.target || videoEl.value
  if (!v) return
  currentTime.value = v.currentTime || 0
  if (currentTime.value >= RESUME_MIN_SECONDS) {
    lastGoodPosition = currentTime.value
  }
  const d = mediaDuration()
  if (d > 0) lastGoodDuration = d
  saveProgress()
}

function onVideoDurationChange(e) {
  const v = e?.target || videoEl.value
  if (!v) return
  duration.value = Number.isFinite(v.duration) ? v.duration : 0
  if (duration.value > 0) lastGoodDuration = duration.value
}

function onVideoLoadedMetadata(e) {
  const v = e?.target || videoEl.value
  if (!v) return
  duration.value = Number.isFinite(v.duration) ? v.duration : 0
  if (duration.value > 0) lastGoodDuration = duration.value
  applyResumeSeek()
}

function onVideoSeeked(e) {
  const v = e?.target || videoEl.value
  if (!v) return
  currentTime.value = v.currentTime || 0
  if (currentTime.value >= RESUME_MIN_SECONDS) {
    lastGoodPosition = currentTime.value
  }
  const d = mediaDuration()
  if (d > 0) lastGoodDuration = d
  // Scrubbing while paused never fires a useful timeupdate cadence — force save.
  saveProgress({ force: true })
}

function onVisibilityFlush() {
  if (document.visibilityState === 'hidden') {
    saveProgress({ force: true })
  }
}

function onPageHide() {
  const id = film.value?.id || progressFilmId
  if (!id) return
  const { pos, dur } = readPlaybackClock()
  if (pos < RESUME_MIN_SECONDS) return
  const body = JSON.stringify({
    position_seconds: pos,
    duration_seconds: dur,
    completed: false,
  })
  try {
    fetch(`/api/progress/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body,
      keepalive: true,
    })
  } catch {
    /* ignore */
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
  if (!v) return
  const total = mediaDuration()
  const now = Number.isFinite(v.currentTime) ? v.currentTime : 0
  let next = total > 0 ? Math.min(Math.max(0, now + delta), total) : Math.max(0, now + delta)
  if (isPlayStation) next = clampToSeekable(next)
  try {
    v.currentTime = next
    currentTime.value = v.currentTime || next
  } catch {
    /* PS4 may reject seeks until enough data is buffered */
  }
  flashSeek(delta < 0 ? 'back' : 'fwd')
  showOverlay()
}

function seekToRatio(ratio) {
  const v = videoEl.value
  const total = mediaDuration()
  if (!v || total <= 0) return
  let next = Math.min(Math.max(0, total * ratio), total)
  if (isPlayStation) next = clampToSeekable(next)
  try {
    v.currentTime = next
    currentTime.value = v.currentTime || next
  } catch {
    /* ignore */
  }
  showOverlay()
}

function clampToSeekable(time) {
  const v = videoEl.value
  if (!v || !v.seekable || v.seekable.length === 0) return time
  try {
    for (let i = 0; i < v.seekable.length; i++) {
      const start = v.seekable.start(i)
      const end = v.seekable.end(i)
      if (time >= start && time <= end) return time
    }
    // Prefer the nearest seekable edge (usually the end of what is buffered).
    let nearest = v.seekable.end(0)
    let bestDist = Math.abs(time - nearest)
    for (let i = 0; i < v.seekable.length; i++) {
      const start = v.seekable.start(i)
      const end = v.seekable.end(i)
      const candidates = [start, end]
      for (let c = 0; c < candidates.length; c++) {
        const dist = Math.abs(time - candidates[c])
        if (dist < bestDist) {
          bestDist = dist
          nearest = candidates[c]
        }
      }
    }
    return nearest
  } catch {
    return time
  }
}

function seekFromClientX(el, clientX) {
  const total = mediaDuration()
  if (total <= 0 || !el) return
  const rect = el.getBoundingClientRect()
  if (!rect.width) return
  seekToRatio((clientX - rect.left) / rect.width)
}

function onProgressClick(e) {
  seekFromClientX(e.currentTarget, e.clientX)
}

function onProgressPointerDown(e) {
  if (isPlayStation || e.button !== 0) return
  const el = e.currentTarget
  e.preventDefault()
  el.setPointerCapture?.(e.pointerId)
  seekFromClientX(el, e.clientX)
  const onMove = (ev) => seekFromClientX(el, ev.clientX)
  const onUp = () => {
    el.removeEventListener('pointermove', onMove)
    el.removeEventListener('pointerup', onUp)
    el.removeEventListener('pointercancel', onUp)
  }
  el.addEventListener('pointermove', onMove)
  el.addEventListener('pointerup', onUp)
  el.addEventListener('pointercancel', onUp)
}

function onFilmBarKeydown(e) {
  if (e.defaultPrevented) return
  if (film.value?.playback_status !== 'ready') return
  switch (e.key) {
    case 'ArrowLeft':
      e.preventDefault()
      e.stopPropagation()
      seekBy(-(e.shiftKey ? 30 : SEEK_SECONDS))
      break
    case 'ArrowRight':
      e.preventDefault()
      e.stopPropagation()
      seekBy(e.shiftKey ? 30 : SEEK_SECONDS)
      break
    case ' ':
    case 'Enter':
      if (e.target && e.target.closest && e.target.closest('button, a, input, select, textarea')) {
        return
      }
      e.preventDefault()
      togglePlay()
      break
    default:
      break
  }
}

function bumpVolume(delta) {
  const v = videoEl.value
  if (!v) return
  v.volume = Math.min(1, Math.max(0, v.volume + delta))
}

function getFullscreenElement() {
  return (
    document.fullscreenElement ||
    document.webkitFullscreenElement ||
    document.msFullscreenElement ||
    null
  )
}

async function toggleFullscreen() {
  const v = videoEl.value
  if (!v) return
  try {
    if (getFullscreenElement()) {
      const exit =
        document.exitFullscreen ||
        document.webkitExitFullscreen ||
        document.msExitFullscreen
      if (exit) await exit.call(document)
      return
    }
    if (v.webkitEnterFullscreen) {
      v.webkitEnterFullscreen()
      return
    }
    const request = v.requestFullscreen || v.webkitRequestFullscreen || v.msRequestFullscreen
    if (request) await request.call(v)
  } catch (err) {
    console.warn('Fullscreen failed', err)
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
      } else if (seriesCompleteVisible.value) {
        e.preventDefault()
        dismissSeriesComplete()
      }
      break
    default:
      break
  }
}

onMounted(() => {
  load()
  window.addEventListener('keydown', onKeydown)
  document.addEventListener('visibilitychange', onVisibilityFlush)
  window.addEventListener('pagehide', onPageHide)
})
watch(() => props.id, load)
onUnmounted(() => {
  saveProgress({ force: true })
  clearInterval(pollTimer)
  clearTimeout(hideTimer)
  clearTimeout(flashTimer)
  cancelUpNext()
  window.removeEventListener('keydown', onKeydown)
  document.removeEventListener('visibilitychange', onVisibilityFlush)
  window.removeEventListener('pagehide', onPageHide)
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
        <RouterLink to="/admin/conversions">Conversions</RouterLink>.
      </div>
      <div v-else-if="film.playback_status === 'failed'" class="banner err">
        Conversion failed. Retry from
        <RouterLink to="/admin/conversions">Conversions</RouterLink>, or re-upload an H.264 MP4.
      </div>
      <div
        class="player-shell"
        :class="{ console: isPlayStation }"
        @mousemove="!isPlayStation && showOverlay()"
        @mouseleave="!isPlayStation && playing && scheduleHideOverlay()"
      >
        <template v-if="film.playback_status === 'ready'">
          <div class="video-frame">
            <video
              ref="videoEl"
              :key="film.stream_url + film.playback_status"
              controls
              playsinline
              preload="metadata"
              :poster="film.poster_url"
              :src="film.stream_url"
              @play="onVideoPlay"
              @pause="onVideoPause"
              @ended="onVideoEnded"
              @timeupdate="onVideoTimeUpdate"
              @durationchange="onVideoDurationChange"
              @loadedmetadata="onVideoLoadedMetadata"
              @seeked="onVideoSeeked"
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

            <div
              v-if="!isPlayStation"
              class="overlay"
              :class="{ visible: overlayVisible || !playing }"
            >
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

            <div
              v-else-if="seriesCompleteVisible"
              class="up-next series-complete"
              role="dialog"
              aria-label="Series complete"
            >
              <p class="up-next-label">Series complete</p>
              <strong>{{ film.series_name || 'This series' }}</strong>
              <p class="series-complete-copy">You’ve finished the last episode.</p>
              <div class="up-next-actions">
                <RouterLink class="up-next-play" to="/">Back to library</RouterLink>
                <button type="button" class="up-next-cancel" @click="dismissSeriesComplete">
                  Stay here
                </button>
              </div>
            </div>
          </div>

          <div
            v-if="isPlayStation"
            class="film-bar"
            role="group"
            aria-label="Playback controls"
            tabindex="0"
            @keydown="onFilmBarKeydown"
          >
            <div class="film-bar-row primary">
              <button
                v-if="prevEntry"
                type="button"
                class="film-btn episode"
                :aria-label="`Previous episode: ${prevEntry.name}`"
                @click="goToEntry(prevEntry)"
              >
                ← Ep
              </button>
              <button type="button" class="film-btn play" :aria-label="playLabel" @click="togglePlay">
                {{ playing ? '❚❚' : '▶' }}
              </button>
              <button
                v-if="nextEntry"
                type="button"
                class="film-btn episode"
                :aria-label="`Next episode: ${nextEntry.name}`"
                @click="goToEntry(nextEntry)"
              >
                Ep →
              </button>
              <span class="film-time">
                {{ formatTime(currentTime) }} / {{ formatTime(duration || mediaDuration()) }}
              </span>
            </div>

            <div
              class="film-progress"
              role="slider"
              tabindex="0"
              :aria-valuemin="0"
              :aria-valuemax="Math.floor(duration || mediaDuration() || 0)"
              :aria-valuenow="Math.floor(currentTime)"
              :aria-valuetext="`${formatTime(currentTime)} of ${formatTime(duration || mediaDuration())}`"
              aria-label="Seek"
              @click="onProgressClick"
              @pointerdown="onProgressPointerDown"
              @keydown="onFilmBarKeydown"
            >
              <div class="film-progress-fill" :style="{ width: progressPercent + '%' }" />
            </div>

            <div class="film-bar-row jumps">
              <button
                v-for="jump in backJumps"
                :key="'back-' + jump"
                type="button"
                class="film-btn"
                :aria-label="`Rewind ${formatJump(jump)}`"
                @click="seekBy(-jump)"
              >
                −{{ formatJump(jump) }}
              </button>
              <button
                type="button"
                class="film-btn"
                :aria-label="`Rewind ${SEEK_SECONDS} seconds`"
                @click="seekBy(-SEEK_SECONDS)"
              >
                −{{ SEEK_SECONDS }}s
              </button>
              <button
                type="button"
                class="film-btn"
                :aria-label="`Forward ${SEEK_SECONDS} seconds`"
                @click="seekBy(SEEK_SECONDS)"
              >
                +{{ SEEK_SECONDS }}s
              </button>
              <button
                v-for="jump in SEEK_JUMPS"
                :key="'fwd-' + jump"
                type="button"
                class="film-btn"
                :aria-label="`Forward ${formatJump(jump)}`"
                @click="seekBy(jump)"
              >
                +{{ formatJump(jump) }}
              </button>
            </div>

            <div class="film-bar-row marks">
              <button type="button" class="film-btn mark" @click="seekToRatio(0)">Start</button>
              <button type="button" class="film-btn mark" @click="seekToRatio(0.25)">25%</button>
              <button type="button" class="film-btn mark" @click="seekToRatio(0.5)">50%</button>
              <button type="button" class="film-btn mark" @click="seekToRatio(0.75)">75%</button>
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
          <div v-if="activeSeason" class="ep-group">
            <label v-if="seasonGroups.length > 1" class="season-pick">
              Season
              <select v-model="selectedSeasonKey" aria-label="Select season">
                <option v-for="group in seasonGroups" :key="group.key" :value="group.key">
                  {{ group.label }} ({{ group.entries.length }})
                </option>
              </select>
            </label>
            <h3 v-else>{{ activeSeason.label }}</h3>
            <ul>
              <li v-for="entry in activeSeason.entries" :key="entry.id">
                <button
                  type="button"
                  class="ep-row"
                  :class="{
                    current: entry.id === film.id,
                    disabled: !isPlayable(entry),
                    viewed: entry.viewed && entry.id !== film.id,
                  }"
                  :disabled="entry.id === film.id"
                  @click="goToEntry(entry)"
                >
                  <span v-if="entryLabel(entry)" class="ep-tag">{{ entryLabel(entry) }}</span>
                  <span class="ep-name">{{ entry.name }}</span>
                  <span v-if="entry.id === film.id" class="ep-now">
                    Now
                    <template
                      v-if="
                        progressHint &&
                        !progressHint.completed &&
                        progressHint.position_seconds >= RESUME_MIN_SECONDS
                      "
                    >
                      · {{ formatTime(progressHint.position_seconds) }}
                    </template>
                  </span>
                  <span v-else-if="entry.viewed" class="ep-viewed">Viewed</span>
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
  border-radius: 0.35rem;
  border: 1px solid color-mix(in srgb, var(--accent) 45%, transparent);
  font-weight: 700;
  background: transparent;
  color: var(--accent);
}

.banner.err {
  border-color: #8b1e1e;
  background: transparent;
  color: #fde8e8;
}

.player-shell {
  position: relative;
  border-radius: 0.35rem;
  border: 1px solid var(--brown);
  background: #111;
  overflow: visible;
}

.video-frame {
  position: relative;
  overflow: hidden;
  border-radius: 0.3rem 0.3rem 0 0;
  background: #000;
}

.player-shell:not(.console) .video-frame {
  border-radius: 0.3rem;
}

.player-shell.console .video-frame {
  border-radius: 0.3rem 0.3rem 0 0;
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
  border-radius: 0.3rem;
}

.film-bar {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
  padding: 0.75rem 0.85rem 0.9rem;
  background: #121212;
  border-top: 1px solid var(--brown);
  border-radius: 0 0 0.3rem 0.3rem;
}

.film-bar:focus {
  outline: 3px solid var(--accent-yellow);
  outline-offset: 2px;
}

.film-bar-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.45rem;
}

.film-bar-row.jumps,
.film-bar-row.marks {
  justify-content: center;
}

.film-btn {
  flex: 0 0 auto;
  min-width: 3rem;
  min-height: 2.75rem;
  padding: 0.45rem 0.7rem;
  border: 1px solid var(--brown);
  border-radius: 0.35rem;
  background: var(--accent);
  color: var(--cream);
  font-weight: 700;
  font-size: 0.9rem;
}

.film-btn.play {
  min-width: 3.6rem;
  background: var(--accent);
}

.film-btn.episode {
  background: transparent;
  color: var(--cream);
}

.film-btn.mark {
  background: transparent;
  border-color: color-mix(in srgb, var(--cream) 35%, transparent);
  font-size: 0.85rem;
}

.film-btn:focus {
  outline: 3px solid var(--accent-yellow);
  outline-offset: 2px;
}

.film-progress {
  position: relative;
  width: 100%;
  height: 1.85rem;
  border-radius: 0.25rem;
  background: #2a2a2a;
  border: 1px solid color-mix(in srgb, var(--cream) 25%, transparent);
  overflow: hidden;
  cursor: pointer;
  touch-action: none;
  user-select: none;
}

.film-progress:focus {
  outline: 3px solid var(--accent-yellow);
  outline-offset: 2px;
}

.film-progress-fill {
  height: 100%;
  background: var(--accent);
  border-radius: inherit;
  width: 0;
}

.film-time {
  margin-left: auto;
  flex: 0 0 auto;
  min-width: 6.5rem;
  font-weight: 800;
  font-variant-numeric: tabular-nums;
  color: var(--cream);
  font-size: 0.95rem;
  text-align: right;
}

.player-shell.console .film-btn {
  min-width: 3.6rem;
  min-height: 3.2rem;
  font-size: 1rem;
}

.player-shell.console .film-progress {
  height: 2rem;
}

.player-shell.console .film-time {
  font-size: 1.05rem;
  min-width: 7.5rem;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

.overlay {
  position: absolute;
  inset: 0 0 0;
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
}

.ctl {
  pointer-events: auto;
  display: grid;
  place-items: center;
  gap: 0.1rem;
  width: clamp(3.2rem, 8vw, 4.2rem);
  height: clamp(3.2rem, 8vw, 4.2rem);
  border: 1px solid rgba(255, 255, 255, 0.45);
  border-radius: 0.35rem;
  background: rgba(0, 0, 0, 0.55);
  color: var(--cream);
  transition: background 0.15s ease, border-color 0.15s ease;
}

.ctl:hover,
.ctl:focus-visible {
  background: color-mix(in srgb, var(--accent) 85%, #000);
  border-color: var(--accent);
  outline: none;
}

.ctl.play {
  width: clamp(4rem, 10vw, 5.2rem);
  height: clamp(4rem, 10vw, 5.2rem);
  background: color-mix(in srgb, var(--accent) 88%, #000);
  border-color: var(--accent);
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
  border-radius: 0.35rem;
  background: rgba(0, 0, 0, 0.7);
  color: var(--accent);
  font-weight: 700;
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
  padding: 0;
  background: transparent;
  border: none;
  color: var(--ink);
}

.eyebrow {
  margin: 0;
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--accent);
}

.details h1 {
  margin: 0.35rem 0 0.75rem;
  font-family: var(--font-display);
  font-size: clamp(1.8rem, 4vw, 2.6rem);
  color: var(--cream);
}

.desc {
  margin: 0 0 1rem;
  line-height: 1.55;
  color: color-mix(in srgb, var(--cream) 80%, transparent);
}

.shortcuts {
  margin: 0 0 1rem;
  font-size: 0.9rem;
  font-weight: 600;
  color: color-mix(in srgb, var(--cream) 65%, transparent);
}

.shortcuts kbd {
  display: inline-block;
  margin: 0 0.1rem;
  padding: 0.12rem 0.4rem;
  border: 1px solid color-mix(in srgb, var(--cream) 30%, transparent);
  border-radius: 0.25rem;
  background: transparent;
  font: inherit;
  font-size: 0.8rem;
  color: var(--cream);
}

.series-line {
  margin: 0 0 0.75rem;
  font-weight: 700;
  color: color-mix(in srgb, var(--cream) 85%, transparent);
}

.ep-pos {
  font-weight: 600;
  color: color-mix(in srgb, var(--cream) 55%, transparent);
}

.series-link {
  color: var(--accent);
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
  border: 1px solid color-mix(in srgb, var(--cream) 28%, transparent);
  border-radius: 0.35rem;
  background: transparent;
  color: var(--cream);
  text-align: left;
  transition: border-color 0.15s ease, color 0.15s ease;
}

.nav-btn.next {
  text-align: right;
  flex-direction: row-reverse;
}

.nav-btn:hover:not(:disabled),
.nav-btn:focus-visible:not(:disabled) {
  border-color: var(--accent);
  color: var(--accent);
  outline: none;
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
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--accent);
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
  border: 1px solid color-mix(in srgb, var(--cream) 22%, transparent);
  border-radius: 0.35rem;
  background: transparent;
  overflow: hidden;
}

.episodes summary {
  cursor: pointer;
  padding: 0.75rem 1rem;
  font-weight: 700;
  color: var(--cream);
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
  border: 1px solid color-mix(in srgb, var(--cream) 30%, transparent);
  border-radius: 0.25rem;
  font: inherit;
  font-size: 0.75rem;
  font-weight: 700;
}

.ep-group {
  padding: 0 0.75rem 0.85rem;
}

.season-pick {
  display: grid;
  gap: 0.35rem;
  margin: 0.35rem 0 0.65rem;
  max-width: 18rem;
  font-size: 0.8rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--accent);
}

.season-pick select {
  width: 100%;
  padding: 0.55rem 0.75rem;
  border: 1px solid color-mix(in srgb, var(--cream) 28%, transparent);
  border-radius: 0.35rem;
  background: transparent;
  color: var(--cream);
  font: inherit;
  font-size: 0.9rem;
  font-weight: 700;
  text-transform: none;
  letter-spacing: normal;
}

.season-pick select option {
  background: var(--bg);
  color: var(--cream);
}

.ep-group h3 {
  margin: 0.35rem 0 0.45rem;
  font-size: 0.8rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--accent);
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
  border-radius: 0.35rem;
  background: transparent;
  color: var(--cream);
  text-align: left;
}

.ep-row:hover:not(:disabled),
.ep-row:focus-visible:not(:disabled) {
  border-color: var(--accent);
  outline: none;
}

.ep-row.current {
  border-color: var(--accent);
  background: color-mix(in srgb, var(--accent) 12%, transparent);
}

.ep-row.disabled:not(.current) {
  opacity: 0.55;
}

.ep-tag {
  font-size: 0.75rem;
  font-weight: 700;
  color: var(--accent);
  flex-shrink: 0;
}

.ep-name {
  font-family: var(--font-display);
  font-weight: 700;
  flex: 1;
  min-width: 0;
}

.ep-now,
.ep-status,
.ep-viewed {
  font-size: 0.72rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.ep-now {
  color: var(--accent);
}

.ep-viewed {
  color: color-mix(in srgb, var(--cream) 70%, transparent);
}

.ep-status {
  color: color-mix(in srgb, var(--cream) 55%, transparent);
}

.up-next {
  position: absolute;
  right: 1rem;
  bottom: 4.2rem;
  z-index: 3;
  max-width: min(22rem, calc(100% - 2rem));
  padding: 1rem 1.1rem;
  border-radius: 0.35rem;
  border: 1px solid var(--accent);
  background: rgba(10, 16, 13, 0.94);
  color: var(--cream);
  animation: fadeRise 0.35s ease both;
}

.up-next-label {
  margin: 0 0 0.25rem;
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--accent);
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
  border-radius: 0.35rem;
  font-weight: 700;
}

a.up-next-play {
  display: inline-flex;
  align-items: center;
  text-decoration: none;
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

.series-complete-copy {
  margin: 0 0 0.75rem;
  font-size: 0.9rem;
  color: rgba(255, 248, 231, 0.8);
}

.back {
  font-weight: 700;
  color: var(--accent);
}

.state {
  padding: 2rem;
  background: transparent;
  border: 1px solid color-mix(in srgb, var(--cream) 25%, transparent);
  border-radius: 0.35rem;
  color: var(--cream);
}

.state.error {
  color: #fde8e8;
  border-color: #8b1e1e;
}

.player-shell:fullscreen,
.player-shell:-webkit-full-screen {
  width: 100%;
  height: 100%;
  max-height: none;
  border-radius: 0;
  border: none;
  display: flex;
  flex-direction: column;
  background: #000;
  box-shadow: none;
}

.player-shell:fullscreen .video-frame,
.player-shell:-webkit-full-screen .video-frame {
  flex: 1 1 auto;
  min-height: 0;
  border-radius: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #000;
}

.player-shell:fullscreen video,
.player-shell:-webkit-full-screen video {
  width: 100%;
  height: 100%;
  max-height: none;
  min-height: 0;
  object-fit: contain;
}

.player-shell:fullscreen .film-bar,
.player-shell:-webkit-full-screen .film-bar {
  flex: 0 0 auto;
  border-radius: 0;
}

.player-shell:fullscreen .up-next,
.player-shell:-webkit-full-screen .up-next {
  bottom: 5rem;
}
</style>
