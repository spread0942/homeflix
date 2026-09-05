<script setup>
import { onMounted, ref } from 'vue'
import { createAnimation, deleteAnimation, listAnimations } from '../api'

const films = ref([])
const loading = ref(true)
const submitting = ref(false)
const error = ref('')
const success = ref('')

const name = ref('')
const description = ref('')
const videoFile = ref(null)
const posterFile = ref(null)

async function refresh() {
  loading.value = true
  error.value = ''
  try {
    films.value = await listAnimations()
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

onMounted(refresh)

function onVideoChange(e) {
  videoFile.value = e.target.files?.[0] || null
}

function onPosterChange(e) {
  posterFile.value = e.target.files?.[0] || null
}

async function onSubmit(e) {
  e.preventDefault()
  success.value = ''
  error.value = ''

  if (!name.value.trim() || !videoFile.value || !posterFile.value) {
    error.value = 'Name, video, and poster are required.'
    return
  }

  const form = new FormData()
  form.append('name', name.value.trim())
  form.append('description', description.value.trim())
  form.append('video', videoFile.value)
  form.append('poster', posterFile.value)

  submitting.value = true
  try {
    await createAnimation(form)
    success.value = `"${name.value.trim()}" is aboard the Sunny.`
    name.value = ''
    description.value = ''
    videoFile.value = null
    posterFile.value = null
    e.target.reset()
    await refresh()
  } catch (err) {
    error.value = err.message
  } finally {
    submitting.value = false
  }
}

async function remove(id, filmName) {
  if (!confirm(`Throw "${filmName}" overboard?`)) return
  error.value = ''
  try {
    await deleteAnimation(id)
    await refresh()
  } catch (e) {
    error.value = e.message
  }
}
</script>

<template>
  <section class="admin">
    <header class="intro">
      <p class="eyebrow">Shipwright desk</p>
      <h1>Galley-La Dock</h1>
      <p>Upload films and posters to the Sunny’s media hold. Prefer H.264 MP4 for smooth sailing.</p>
    </header>

    <form class="form" @submit="onSubmit">
      <label>
        Name
        <input v-model="name" type="text" required maxlength="200" placeholder="Film title" />
      </label>
      <label>
        Description
        <textarea
          v-model="description"
          rows="4"
          maxlength="2000"
          placeholder="Log entry / synopsis"
        />
      </label>
      <div class="files">
        <label>
          Video file
          <input type="file" accept="video/*" required @change="onVideoChange" />
        </label>
        <label>
          Poster image
          <input type="file" accept="image/*" required @change="onPosterChange" />
        </label>
      </div>
      <button class="submit" type="submit" :disabled="submitting">
        {{ submitting ? 'Loading cargo…' : 'Stow aboard' }}
      </button>
    </form>

    <p v-if="success" class="banner ok">{{ success }}</p>
    <p v-if="error" class="banner err">{{ error }}</p>

    <div class="inventory">
      <h2>Cargo hold</h2>
      <div v-if="loading" class="state">Counting barrels…</div>
      <ul v-else-if="films.length" class="list">
        <li v-for="film in films" :key="film.id">
          <img :src="film.poster_url" :alt="film.name" />
          <div>
            <strong>{{ film.name }}</strong>
            <p>{{ film.description || '—' }}</p>
          </div>
          <div class="actions">
            <RouterLink :to="{ name: 'watch', params: { id: film.id } }">Watch</RouterLink>
            <button type="button" class="danger" @click="remove(film.id, film.name)">Delete</button>
          </div>
        </li>
      </ul>
      <div v-else class="state">Hold is empty.</div>
    </div>
  </section>
</template>

<style scoped>
.admin {
  padding: 0.5rem clamp(1rem, 4vw, 3rem) 3rem;
  animation: fadeRise 0.55s ease both;
}

.intro {
  max-width: 40rem;
  margin-bottom: 1.5rem;
}

.eyebrow {
  margin: 0;
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--wood-brown);
}

.intro h1 {
  margin: 0.35rem 0 0.6rem;
  font-family: var(--font-display);
  font-size: clamp(2.2rem, 6vw, 3.4rem);
  color: var(--cream-sail);
  text-shadow: 0 3px 0 color-mix(in srgb, var(--wood-brown) 50%, transparent);
}

.intro p {
  margin: 0;
  color: var(--ink);
}

.form {
  display: grid;
  gap: 1rem;
  max-width: 40rem;
  padding: 1.25rem;
  background: color-mix(in srgb, var(--cream-sail) 92%, transparent);
  border: 2px solid color-mix(in srgb, var(--wood-brown) 30%, transparent);
  border-radius: 1rem;
}

label {
  display: grid;
  gap: 0.4rem;
  font-weight: 700;
  color: var(--wood-brown);
}

input,
textarea {
  width: 100%;
  padding: 0.7rem 0.85rem;
  border: 2px solid color-mix(in srgb, var(--wood-brown) 40%, transparent);
  border-radius: 0.65rem;
  background: #fff;
  color: var(--ink);
}

.files {
  display: grid;
  gap: 1rem;
}

@media (min-width: 640px) {
  .files {
    grid-template-columns: 1fr 1fr;
  }
}

.submit {
  justify-self: start;
  padding: 0.75rem 1.4rem;
  border: none;
  border-radius: 999px;
  font-weight: 700;
  color: var(--cream-sail);
  background: var(--ship-orange);
  box-shadow: 0 4px 0 color-mix(in srgb, var(--wood-brown) 55%, transparent);
  transition: transform 0.2s ease;
}

.submit:hover:not(:disabled) {
  transform: translateY(-2px);
}

.submit:disabled {
  opacity: 0.7;
  cursor: wait;
}

.banner {
  margin: 1rem 0 0;
  padding: 0.85rem 1rem;
  border-radius: 0.75rem;
  font-weight: 700;
  max-width: 40rem;
}

.banner.ok {
  background: color-mix(in srgb, var(--sea-teal) 20%, var(--cream-sail));
  color: var(--sea-teal);
}

.banner.err {
  background: #fde8e8;
  color: #8b1e1e;
}

.inventory {
  margin-top: 2.5rem;
}

.inventory h2 {
  margin: 0 0 1rem;
  font-family: var(--font-display);
  color: var(--cream-sail);
}

.list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.85rem;
}

.list li {
  display: grid;
  grid-template-columns: 64px 1fr auto;
  gap: 0.9rem;
  align-items: center;
  padding: 0.75rem;
  background: color-mix(in srgb, var(--cream-sail) 90%, transparent);
  border: 2px solid color-mix(in srgb, var(--wood-brown) 25%, transparent);
  border-radius: 0.85rem;
}

.list img {
  width: 64px;
  height: 96px;
  object-fit: cover;
  border-radius: 0.4rem;
  border: 2px solid var(--wood-brown);
}

.list p {
  margin: 0.25rem 0 0;
  font-size: 0.9rem;
  color: color-mix(in srgb, var(--ink) 75%, transparent);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.actions {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.actions a,
.danger {
  padding: 0.35rem 0.7rem;
  border-radius: 999px;
  font-weight: 700;
  text-align: center;
  font-size: 0.85rem;
}

.actions a {
  background: var(--sea-teal);
  color: var(--cream-sail);
}

.danger {
  border: none;
  background: transparent;
  color: #8b1e1e;
  border: 1px solid #8b1e1e;
}

.state {
  padding: 1.25rem;
  background: color-mix(in srgb, var(--cream-sail) 88%, transparent);
  border-radius: 0.85rem;
}

@media (max-width: 560px) {
  .list li {
    grid-template-columns: 56px 1fr;
  }

  .actions {
    grid-column: 1 / -1;
    flex-direction: row;
  }
}
</style>
