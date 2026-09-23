<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { deleteAnimation, listAnimations, transcodeAnimation } from '../api'

const films = ref([])
const loading = ref(true)
const error = ref('')
const busyId = ref('')
let pollTimer

const counts = computed(() => {
  const c = { ready: 0, processing: 0, failed: 0, other: 0 }
  for (const f of films.value) {
    const s = f.playback_status || 'ready'
    if (s in c) c[s]++
    else c.other++
  }
  return c
})

const sorted = computed(() => {
  const rank = { processing: 0, failed: 1, ready: 2 }
  return [...films.value].sort((a, b) => {
    const ra = rank[a.playback_status] ?? 3
    const rb = rank[b.playback_status] ?? 3
    if (ra !== rb) return ra - rb
    return new Date(b.created_at) - new Date(a.created_at)
  })
})

async function refresh({ quiet = false } = {}) {
  if (!quiet) loading.value = true
  try {
    films.value = await listAnimations()
    error.value = ''
  } catch (e) {
    if (!quiet) error.value = e.message
  } finally {
    loading.value = false
  }
}

function formatLabel(film) {
  const ct = film.content_type || ''
  if (ct.includes('matroska') || ct.includes('x-matroska')) return 'MKV'
  if (ct.includes('mp4')) return 'MP4'
  if (ct.includes('webm')) return 'WebM'
  return ct || 'unknown'
}

function statusLabel(status) {
  switch (status) {
    case 'processing':
      return 'Converting'
    case 'failed':
      return 'Failed'
    case 'ready':
      return 'Ready'
    default:
      return status || 'Unknown'
  }
}

function statusHint(film) {
  switch (film.playback_status) {
    case 'processing':
      return 'ffmpeg is building an H.264/AAC MP4. Large films can take a long time — this list refreshes every few seconds.'
    case 'failed':
      return 'Conversion failed. Retry, or re-upload an H.264 MP4.'
    case 'ready':
      return formatLabel(film) === 'MP4' ? 'Browser-ready.' : 'Marked ready.'
    default:
      return ''
  }
}

async function retry(film) {
  busyId.value = film.id
  error.value = ''
  try {
    await transcodeAnimation(film.id)
    await refresh({ quiet: true })
  } catch (e) {
    error.value = e.message
  } finally {
    busyId.value = ''
  }
}

async function remove(film) {
  if (!confirm(`Delete "${film.name}"?`)) return
  error.value = ''
  try {
    await deleteAnimation(film.id)
    await refresh({ quiet: true })
  } catch (e) {
    error.value = e.message
  }
}

onMounted(() => {
  refresh()
  pollTimer = setInterval(() => refresh({ quiet: true }), 4000)
})
onUnmounted(() => clearInterval(pollTimer))
</script>

<template>
  <section class="conversions">
    <header class="intro">
      <p class="eyebrow">Transcoding</p>
      <h1>Conversions</h1>
      <p>
        Live queue for browser playback. Unsupported uploads (MKV, HEVC, …) become H.264 MP4 here.
      </p>
    </header>

    <div class="stats">
      <div class="stat processing">
        <strong>{{ counts.processing }}</strong>
        <span>Converting</span>
      </div>
      <div class="stat ready">
        <strong>{{ counts.ready }}</strong>
        <span>Ready</span>
      </div>
      <div class="stat failed">
        <strong>{{ counts.failed }}</strong>
        <span>Failed</span>
      </div>
    </div>

    <p v-if="error" class="banner err">{{ error }}</p>
    <div v-if="loading && !films.length" class="state">Loading…</div>

    <ul v-else-if="sorted.length" class="queue">
      <li v-for="film in sorted" :key="film.id" :class="film.playback_status">
        <img :src="film.poster_url" :alt="film.name" />
        <div class="meta">
          <div class="title-row">
            <strong>{{ film.name }}</strong>
            <span class="badge" :class="film.playback_status">{{
              statusLabel(film.playback_status)
            }}</span>
          </div>
          <p class="sub">
            <template v-if="film.series_name">{{ film.series_name }} · </template>
            {{ formatLabel(film) }}
            <template v-if="film.playback_status === 'processing'"> → MP4</template>
          </p>
          <p class="hint">{{ statusHint(film) }}</p>
          <div v-if="film.playback_status === 'processing'" class="bar" aria-hidden="true">
            <span class="bar-fill" />
          </div>
        </div>
        <div class="actions">
          <RouterLink
            v-if="film.playback_status === 'ready'"
            :to="{ name: 'watch', params: { id: film.id } }"
          >
            Watch
          </RouterLink>
          <RouterLink
            v-else
            class="ghost"
            :to="{ name: 'watch', params: { id: film.id } }"
          >
            Open
          </RouterLink>
          <button
            v-if="film.playback_status === 'failed' || film.playback_status === 'ready'"
            type="button"
            class="retry"
            :disabled="busyId === film.id"
            @click="retry(film)"
          >
            {{ busyId === film.id ? 'Queuing…' : 'Retry convert' }}
          </button>
          <button
            v-if="film.playback_status === 'processing'"
            type="button"
            class="retry"
            :disabled="busyId === film.id"
            @click="retry(film)"
          >
            {{ busyId === film.id ? 'Queuing…' : 'Re-queue' }}
          </button>
          <button type="button" class="danger" @click="remove(film)">Delete</button>
        </div>
      </li>
    </ul>
    <div v-else class="state">No films yet. Upload from Admin.</div>

    <p class="foot">
      <RouterLink to="/admin">← Back to Admin</RouterLink>
      · Auto-refreshes every 4s
    </p>
  </section>
</template>

<style scoped>
.conversions {
  padding: 0.5rem clamp(1rem, 4vw, 3rem) 3rem;
  animation: fadeRise 0.55s ease both;
}

.intro {
  max-width: 42rem;
  margin-bottom: 1.5rem;
}

.eyebrow {
  margin: 0;
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--brown);
}

.intro h1 {
  margin: 0.35rem 0 0.6rem;
  font-family: var(--font-display);
  font-size: clamp(2.2rem, 6vw, 3.4rem);
  color: var(--cream);
}

.intro p {
  margin: 0;
  color: var(--ink);
}

.stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.75rem;
  max-width: 36rem;
  margin-bottom: 1.5rem;
}

.stat {
  padding: 0.9rem 1rem;
  border-radius: 0.35rem;
  background: transparent;
  border: 1px solid color-mix(in srgb, var(--cream) 22%, transparent);
  color: var(--cream);
}

.stat strong {
  display: block;
  font-family: var(--font-display);
  font-size: 1.8rem;
  line-height: 1;
  color: var(--cream);
}

.stat span {
  font-size: 0.85rem;
  font-weight: 700;
  color: color-mix(in srgb, var(--cream) 70%, transparent);
}

.stat.processing strong {
  color: var(--accent);
}

.stat.ready strong {
  color: var(--accent);
}

.stat.failed strong {
  color: #fde8e8;
}

.banner.err {
  margin: 0 0 1rem;
  padding: 0.85rem 1rem;
  border-radius: 0.35rem;
  border: 1px solid #8b1e1e;
  font-weight: 700;
  background: transparent;
  color: #fde8e8;
  max-width: 40rem;
}

.queue {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.85rem;
}

.queue li {
  display: grid;
  grid-template-columns: 72px 1fr auto;
  gap: 1rem;
  align-items: center;
  padding: 0.85rem;
  background: transparent;
  border: 1px solid color-mix(in srgb, var(--cream) 22%, transparent);
  border-radius: 0.35rem;
  color: var(--cream);
}

.queue li.processing {
  border-color: var(--accent);
}

.queue li.failed {
  border-color: #8b1e1e;
}

.queue img {
  width: 72px;
  height: 108px;
  object-fit: cover;
  border-radius: 0.25rem;
  border: 1px solid color-mix(in srgb, var(--cream) 25%, transparent);
}

.title-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  align-items: center;
}

.title-row strong {
  font-family: var(--font-display);
  font-size: 1.15rem;
  color: var(--cream);
}

.badge {
  padding: 0.2rem 0.45rem;
  border-radius: 0.25rem;
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.badge.processing {
  background: var(--accent);
  color: var(--cream);
}

.badge.ready {
  background: transparent;
  border: 1px solid var(--accent);
  color: var(--accent);
}

.badge.failed {
  background: transparent;
  border: 1px solid #8b1e1e;
  color: #fde8e8;
}

.sub,
.hint {
  margin: 0.3rem 0 0;
  font-size: 0.9rem;
  color: color-mix(in srgb, var(--cream) 70%, transparent);
}

.hint {
  font-weight: 600;
}

.bar {
  margin-top: 0.65rem;
  height: 0.45rem;
  border-radius: 0.2rem;
  background: color-mix(in srgb, var(--cream) 15%, transparent);
  overflow: hidden;
}

.bar-fill {
  display: block;
  height: 100%;
  width: 40%;
  border-radius: inherit;
  background: var(--accent);
  animation: slide 1.4s ease-in-out infinite;
}

@keyframes slide {
  0% {
    transform: translateX(-120%);
  }
  100% {
    transform: translateX(320%);
  }
}

.actions {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  min-width: 7.5rem;
}

.actions a,
.retry,
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

.actions a.ghost {
  background: transparent;
  color: var(--accent);
  border: 1px solid var(--accent);
}

.retry {
  border: 1px solid var(--accent);
  background: transparent;
  color: var(--accent);
}

.retry:disabled {
  opacity: 0.6;
  cursor: wait;
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

.foot {
  margin-top: 1.5rem;
  font-weight: 700;
  color: var(--cream);
}

.foot a {
  color: var(--accent-yellow);
}

@media (max-width: 700px) {
  .queue li {
    grid-template-columns: 56px 1fr;
  }

  .actions {
    grid-column: 1 / -1;
    flex-direction: row;
    flex-wrap: wrap;
    min-width: 0;
  }
}
</style>
