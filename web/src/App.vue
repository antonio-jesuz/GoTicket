<script setup>
import { watch } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from './stores/auth'
import { useTicketStore } from './stores/tickets'

const auth = useAuthStore()
const tickets = useTicketStore()
const router = useRouter()

watch(() => auth.token, (t) => (t ? tickets.connect() : tickets.disconnect()), { immediate: true })

function logout() {
  auth.logout()
  router.push({ name: 'login' })
}
</script>

<template>
  <header v-if="auth.token" class="bar">
    <router-link to="/" class="brand">GoTicket</router-link>
    <nav aria-label="Principal">
      <router-link v-if="auth.isAgent" :to="{ name: 'dashboard' }">Painel</router-link>
      <router-link :to="{ name: 'tickets' }">Chamados</router-link>
      <router-link v-if="auth.isAgent" :to="{ name: 'assets' }">Ativos</router-link>
      <router-link v-if="auth.isAgent" :to="{ name: 'inventory' }">Estoque</router-link>
    </nav>
    <span class="live" :class="{ on: tickets.live }">{{ tickets.live ? 'Ao vivo' : 'Reconectando…' }}</span>
    <span class="who">{{ auth.user?.name }} · {{ auth.isAgent ? 'Agente' : 'Cliente' }}</span>
    <button class="ghost" @click="logout">Sair</button>
  </header>
  <main><router-view /></main>
</template>
