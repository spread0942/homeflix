import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'
import WatchView from '../views/WatchView.vue'
import AdminView from '../views/AdminView.vue'
import SeriesView from '../views/SeriesView.vue'
import ConversionsView from '../views/ConversionsView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    { path: '/series/:id', name: 'series', component: SeriesView, props: true },
    { path: '/watch/:id', name: 'watch', component: WatchView, props: true },
    { path: '/admin', name: 'admin', component: AdminView },
    { path: '/conversions', name: 'conversions', component: ConversionsView },
  ],
  scrollBehavior() {
    return { top: 0 }
  },
})

export default router
