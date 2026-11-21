import { defineStore } from 'pinia'
import { api } from '../api'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('goticket.token'),
    user: JSON.parse(localStorage.getItem('goticket.user') || 'null'),
  }),
  getters: { isAgent: (s) => s.user?.role === 'agent' },
  actions: {
    async login(email, password) {
      const { token, user } = await api('/auth/login', { method: 'POST', body: { email, password } })
      this.token = token
      this.user = user
      localStorage.setItem('goticket.token', token)
      localStorage.setItem('goticket.user', JSON.stringify(user))
    },
    async register(email, name, password) {
      await api('/auth/register', { method: 'POST', body: { email, name, password } })
      await this.login(email, password)
    },
    logout() {
      this.token = null
      this.user = null
      localStorage.removeItem('goticket.token')
      localStorage.removeItem('goticket.user')
    },
  },
})
