import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: () => import('../views/LoginView.vue'), meta: { public: true } },
    { path: '/', redirect: '/dashboard' },
    { path: '/dashboard', name: 'dashboard', component: () => import('../views/DashboardView.vue') },
    { path: '/domains', name: 'domains', component: () => import('../views/DomainsView.vue') },
    { path: '/propagation', name: 'propagation-overview', component: () => import('../views/PropagationView.vue') },
    { path: '/domains/:id/propagation', name: 'propagation', component: () => import('../views/PropagationView.vue') },
    { path: '/notifications', name: 'notifications', component: () => import('../views/NotificationsView.vue') },
    { path: '/backups', name: 'backups', component: () => import('../views/BackupsView.vue') },
    { path: '/users', name: 'users', component: () => import('../views/UsersView.vue'), meta: { adminOnly: true } },
    { path: '/webhooks', name: 'webhooks', component: () => import('../views/WebhooksView.vue') },
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
