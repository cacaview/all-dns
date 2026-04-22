import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import LoginView from '../views/LoginView.vue'
import DashboardView from '../views/DashboardView.vue'
import DomainsView from '../views/DomainsView.vue'
import BackupsView from '../views/BackupsView.vue'
import PropagationView from '../views/PropagationView.vue'
import NotificationsView from '../views/NotificationsView.vue'
import UsersView from '../views/UsersView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: LoginView, meta: { public: true } },
    { path: '/', redirect: '/dashboard' },
    { path: '/dashboard', name: 'dashboard', component: DashboardView },
    { path: '/domains', name: 'domains', component: DomainsView },
    { path: '/propagation', name: 'propagation-overview', component: PropagationView },
    { path: '/domains/:id/propagation', name: 'propagation', component: PropagationView },
    { path: '/notifications', name: 'notifications', component: NotificationsView },
    { path: '/backups', name: 'backups', component: BackupsView },
    { path: '/users', name: 'users', component: UsersView, meta: { adminOnly: true } },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  await auth.initialize()

  if (to.meta.public) {
    if (auth.isAuthenticated) {
      return { name: 'dashboard' }
    }
    return true
  }

  if (!auth.isAuthenticated) {
    return { name: 'login' }
  }

  if (to.meta.adminOnly && auth.user?.role !== 'admin') {
    return { name: 'dashboard' }
  }

  return true
})

export default router
