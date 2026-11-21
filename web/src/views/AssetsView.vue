<script setup>
import { onMounted, reactive, ref, watch } from 'vue'
import { api } from '../api'
import { ASSET_STATUS, ASSET_TYPE, fmtDate } from '../labels'

const assets = ref([])
const users = ref([])
const status = ref('')
const q = ref('')
const showForm = ref(false)
const form = reactive({ name: '', type: 'laptop', serial: '', location: '', warrantyUntil: '' })
const error = ref('')

const load = async () => {
  const p = new URLSearchParams({ ...(status.value && { status: status.value }), ...(q.value && { q: q.value }) })
  assets.value = await api(`/assets?${p}`)
}
onMounted(async () => { users.value = await api('/users'); load() })
watch([status, q], load)

async function create() {
  error.value = ''
  try {
    await api('/assets', { method: 'POST', body: { ...form } })
    Object.assign(form, { name: '', serial: '', location: '', warrantyUntil: '' })
    showForm.value = false
    load()
  } catch (e) { error.value = e.message }
}
const act = async (id, path, body) => {
  error.value = ''
  try { await api(`/assets/${id}/${path}`, { method: 'POST', body }); load() } catch (e) { error.value = e.message }
}
const warrantyExpired = (a) => a.warrantyUntil && new Date(a.warrantyUntil) < new Date()
</script>

<template>
  <div class="head">
    <h1>Ativos</h1>
    <input v-model="q" class="search" type="search" placeholder="Buscar por nome, etiqueta ou série" />
    <select v-model="status" aria-label="Filtrar por status">
      <option value="">Todos os status</option>
      <option v-for="(l, k) in ASSET_STATUS" :key="k" :value="k">{{ l }}</option>
    </select>
    <button @click="showForm = !showForm">{{ showForm ? 'Cancelar' : 'Cadastrar ativo' }}</button>
  </div>
  <p v-if="error" class="error" role="alert">{{ error }}</p>

  <form v-if="showForm" class="card formgrid" @submit.prevent="create">
    <label>Nome <input v-model="form.name" required /></label>
    <label>Tipo
      <select v-model="form.type"><option v-for="(l, k) in ASSET_TYPE" :key="k" :value="k">{{ l }}</option></select>
    </label>
    <label>Número de série <input v-model="form.serial" /></label>
    <label>Local <input v-model="form.location" /></label>
    <label>Garantia até <input v-model="form.warrantyUntil" type="date" /></label>
    <button>Salvar ativo</button>
  </form>

  <p v-if="!assets.length" class="empty">Nenhum ativo encontrado. Use “Cadastrar ativo” para começar o inventário.</p>
  <table v-else>
    <thead><tr><th>Etiqueta</th><th>Ativo</th><th>Tipo</th><th>Status</th><th>Responsável</th><th>Garantia</th></tr></thead>
    <tbody>
      <tr v-for="a in assets" :key="a.id">
        <td>{{ a.tag }}</td>
        <td>{{ a.name }}<br /><small>{{ a.serial || 'sem série' }} · {{ a.location || 'sem local' }}</small></td>
        <td>{{ ASSET_TYPE[a.type] }}</td>
        <td>
          <select :value="a.status" :aria-label="`Status de ${a.tag}`" @change="act(a.id, 'status', { status: $event.target.value })">
            <option v-for="(l, k) in ASSET_STATUS" :key="k" :value="k">{{ l }}</option>
          </select>
        </td>
        <td>
          <select :value="a.assignedTo || ''" :aria-label="`Responsável por ${a.tag}`"
                  @change="act(a.id, 'assign', { userId: $event.target.value ? Number($event.target.value) : null })">
            <option value="">— ninguém —</option>
            <option v-for="u in users" :key="u.id" :value="u.id">{{ u.name }}</option>
          </select>
        </td>
        <td :class="{ breach: warrantyExpired(a) }">{{ fmtDate(a.warrantyUntil) }}</td>
      </tr>
    </tbody>
  </table>
</template>
