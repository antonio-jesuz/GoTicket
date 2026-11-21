import { defineStore } from 'pinia'
import { api, getToken } from '../api'

let source = null

export const useTicketStore = defineStore('tickets', {
  state: () => ({ items: [], status: '', q: '', assignee: '', loading: false, live: false, lastEvent: null }),
  actions: {
    async load() {
      this.loading = true
      try {
        const p = new URLSearchParams()
        if (this.status) p.set('status', this.status)
        if (this.q) p.set('q', this.q)
        if (this.assignee) p.set('assignee', this.assignee)
        this.items = await api(`/tickets?${p}`)
      } finally {
        this.loading = false
      }
    },
    async create(payload) {
      return api('/tickets', { method: 'POST', body: payload })
    },
    // Tempo real: o servidor só envia o que este usuário pode ver (filtro no Hub).
    connect() {
      if (source) return
      source = new EventSource(`/api/events?token=${encodeURIComponent(getToken())}`)
      source.onopen = () => (this.live = true)
      source.onerror = () => (this.live = false) // EventSource reconecta sozinho
      for (const type of ['ticket.created', 'ticket.updated', 'comment.added', 'sla.breached', 'stock.changed']) {
        source.addEventListener(type, (e) => {
          this.lastEvent = JSON.parse(e.data)
          if (!['comment.added', 'stock.changed'].includes(type)) this.load()
        })
      }
    },
    disconnect() {
      source?.close()
      source = null
      this.live = false
    },
  },
})
