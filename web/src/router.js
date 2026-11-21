import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from './stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: () => import('./views/LoginView.vue') },
    { path: '/', name: 'tickets', component: () => import('./views/TicketListView.vue'), meta: { auth: true } },
    { path: '/painel', name: 'dashboard', component: () => import('./views/DashboardView.vue'), meta: { auth: true, agent: true } },
    { path: '/ativos', name: 'assets', component: () => import('./views/AssetsView.vue'), meta: { auth: true, agent: true } },
    { path: '/estoque', name: 'inventory', component: () => import('./views/InventoryView.vue'), meta: { auth: true, agent: true } },
    { path: '/tickets/:id', name: 'ticket', component: () => import('./views/TicketDetailView.vue'), meta: { auth: true } },
  ],
})

router.beforeEach((to) => {
  const auth = useAuthStore()
  if (to.meta.auth && !auth.token) return { name: 'login' }
  if (to.meta.agent && !auth.isAgent) return { name: 'tickets' }
  if (to.name === 'login' && auth.token) return { name: auth.isAgent ? 'dashboard' : 'tickets' }
})

export default router
