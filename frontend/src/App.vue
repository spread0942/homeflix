<script setup>
import { RouterLink, RouterView, useRoute } from 'vue-router'
import { computed } from 'vue'

const route = useRoute()
const isHome = computed(() => route.name === 'home')
</script>

<template>
  <div class="shell">
    <header class="topbar" :class="{ compact: !isHome }">
      <RouterLink class="brand" to="/">
        <span class="brand-mark" aria-hidden="true">▶</span>
        <span class="brand-text">Homeflix</span>
      </RouterLink>
      <nav class="nav">
        <RouterLink to="/">Library</RouterLink>
        <RouterLink
          to="/admin"
          :class="{ 'router-link-active': route.path.startsWith('/admin') }"
        >
          Admin
        </RouterLink>
      </nav>
    </header>
    <main>
      <RouterView />
    </main>
    <footer class="footer">
      <p>Personal film library — for your home use only.</p>
    </footer>
  </div>
</template>

<style scoped>
.shell {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
}

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 1.25rem clamp(1rem, 4vw, 3rem);
  transition: padding 0.35s ease;
}

.topbar.compact {
  padding-block: 0.85rem;
}

.brand {
  display: inline-flex;
  align-items: center;
  gap: 0.55rem;
  font-family: var(--font-display);
  font-weight: 700;
  font-size: clamp(1.35rem, 3vw, 1.85rem);
  letter-spacing: 0.01em;
  color: var(--cream);
}

.brand-mark {
  display: grid;
  place-items: center;
  width: 2.1rem;
  height: 2.1rem;
  border-radius: 0.35rem;
  background: var(--accent);
  color: var(--cream);
  font-size: 0.75rem;
}

.nav {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem 1rem;
  justify-content: flex-end;
}

.nav a {
  padding: 0.35rem 0;
  border-bottom: 2px solid transparent;
  font-weight: 600;
  color: var(--cream);
  background: transparent;
  transition: color 0.15s ease, border-color 0.15s ease;
}

.nav a:hover,
.nav a.router-link-active {
  color: var(--accent);
  border-bottom-color: var(--accent);
}

main {
  flex: 1;
}

.footer {
  padding: 1.5rem clamp(1rem, 4vw, 3rem) 2rem;
  color: var(--cream);
  opacity: 0.9;
  font-size: 0.95rem;
}
</style>
