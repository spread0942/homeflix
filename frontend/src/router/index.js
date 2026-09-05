import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'
import WatchView from '../views/WatchView.vue'
import AdminView from '../views/AdminView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    { path: '/watch/:id', name: 'watch', component: WatchView, props: true },
    { path: '/admin', name: 'admin', component: AdminView },
  ],
  scrollBehavior() {
    return { top: 0 }
  },
})

export default router
