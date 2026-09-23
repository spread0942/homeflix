import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'
import WatchView from '../views/WatchView.vue'
import AdminView from '../views/AdminView.vue'
import SeriesView from '../views/SeriesView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    { path: '/series/:id', name: 'series', component: SeriesView, props: true },
    { path: '/watch/:id', name: 'watch', component: WatchView, props: true },
    { path: '/admin', redirect: { name: 'admin-films' } },
    { path: '/admin/upload', redirect: { name: 'admin-films' } },
    { path: '/admin/films', name: 'admin-films', component: AdminView },
    { path: '/admin/series', name: 'admin-series', component: AdminView },
    { path: '/admin/conversions', name: 'admin-conversions', component: AdminView },
    { path: '/conversions', redirect: { name: 'admin-conversions' } },
  ],
  scrollBehavior() {
    return { top: 0 }
  },
})

export default router
