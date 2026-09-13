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
        <RouterLink to="/conversions">Conversions</RouterLink>
        <RouterLink to="/admin">Admin</RouterLink>
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
  border-radius: 50%;
  background: var(--accent);
  color: var(--cream);
  font-size: 0.75rem;
  box-shadow: 0 4px 0 color-mix(in srgb, var(--accent-yellow) 55%, transparent);
  animation: softPulse 4s ease-in-out infinite;
}

.nav {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  justify-content: flex-end;
}

.nav a {
  padding: 0.45rem 0.9rem;
  border-radius: 999px;
  font-weight: 700;
  color: var(--cream);
  background: var(--teal);
  transition: transform 0.2s ease, background 0.2s ease;
}

.nav a:hover,
.nav a.router-link-active {
  background: var(--accent);
  transform: translateY(-1px);
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
