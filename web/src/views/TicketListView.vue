<script setup>
import { onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../api'
import { useAuthStore } from '../stores/auth'
import { useTicketStore } from '../stores/tickets'
import { PRIORITY, STATUS, slaLabel } from '../labels'

const route = useRoute()
const auth = useAuthStore()
const store = useTicketStore()
const showForm = ref(false)
const form = reactive({ title: '', description: '', priority: 'medium', categoryId: null, assetId: null })
const categories = ref([])
const assets = ref([])
const error = ref('')

store.assignee = route.query.assignee || ''
onMounted(async () => {
  store.load()
  categories.value = await api('/categories')
  assets.value = await api('/assets') // agente: todos · cliente: os seus
})
watch(() => [store.status, store.assignee], () => store.load())
let timer
watch(() => store.q, () => { clearTimeout(timer); timer = setTimeout(() => store.load(), 250) })

const catName = (id) => categories.value.find((c) => c.id === id)?.name || '—'

async function submit() {
  error.value = ''
  try {
    await store.create({ ...form })
    Object.assign(form, { title: '', description: '', priority: 'medium', categoryId: null, assetId: null })
    showForm.value = false
    store.load()
  } catch (e) { error.value = e.message }
}
</script>

<template>
  <div class="head">
    <h1>Chamados</h1>
    <input v-model="store.q" class="search" type="search" placeholder="Buscar por número ou título" />
    <select v-if="auth.isAgent" v-model="store.assignee" aria-label="Fila">
      <option value="">Todas as filas</option><option value="me">Meus chamados</option><option value="none">Sem agente</option>
    </select>
    <select v-model="store.status" aria-label="Filtrar por status">
      <option value="">Todos os status</option>
      <option v-for="(label, key) in STATUS" :key="key" :value="key">{{ label }}</option>
    </select>
    <button @click="showForm = !showForm">{{ showForm ? 'Cancelar' : 'Abrir chamado' }}</button>
  </div>

  <form v-if="showForm" class="card formgrid" @submit.prevent="submit">
    <label class="full">Título <input v-model="form.title" maxlength="200" required /></label>
    <label class="full">Descrição <textarea v-model="form.description" rows="3" /></label>
    <label>Prioridade
      <select v-model="form.priority"><option v-for="(l, k) in PRIORITY" :key="k" :value="k">{{ l }}</option></select>
    </label>
    <label>Categoria
      <select v-model="form.categoryId"><option :value="null">— escolha —</option>
        <option v-for="c in categories" :key="c.id" :value="c.id">{{ c.name }}</option></select>
    </label>
    <label>Ativo afetado
      <select v-model="form.assetId"><option :value="null">— nenhum —</option>
        <option v-for="a in assets" :key="a.id" :value="a.id">{{ a.tag }} · {{ a.name }}</option></select>
    </label>
    <p v-if="error" class="error full" role="alert">{{ error }}</p>
    <button>Enviar chamado</button>
  </form>

  <p v-if="!store.loading && !store.items.length" class="empty">
    Nenhum chamado encontrado. Ajuste os filtros ou use “Abrir chamado”.
  </p>
  <table v-else>
    <thead><tr><th>Número</th><th>Título</th><th>Categoria</th><th>Prioridade</th><th>Status</th><th>SLA</th></tr></thead>
    <tbody>
      <tr v-for="t in store.items" :key="t.id">
        <td><router-link :to="{ name: 'ticket', params: { id: t.id } }">{{ t.number }}</router-link></td>
        <td>{{ t.title }}</td>
        <td>{{ catName(t.categoryId) }}</td>
        <td><span class="pill" :class="t.priority">{{ PRIORITY[t.priority] }}</span></td>
        <td>{{ STATUS[t.status] }}</td>
        <td :class="{ breach: t.slaBreached }">{{ slaLabel(t) }}</td>
      </tr>
    </tbody>
  </table>
</template>
