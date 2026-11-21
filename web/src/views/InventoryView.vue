<script setup>
import { onMounted, reactive, ref, watch } from 'vue'
import { api } from '../api'
import { REASON, fmtDateTime } from '../labels'

const items = ref([])
const q = ref('')
const showForm = ref(false)
const form = reactive({ sku: '', name: '', category: '', quantity: 0, minQuantity: 0, location: '' })
const selected = ref(null)
const history = ref([])
const move = reactive({ reason: 'in', quantity: 1, note: '' })
const error = ref('')

const load = async () => { items.value = await api(`/inventory?q=${encodeURIComponent(q.value)}`) }
onMounted(load)
watch(q, load)

async function create() {
  error.value = ''
  try {
    await api('/inventory', { method: 'POST', body: { ...form } })
    Object.assign(form, { sku: '', name: '', category: '', quantity: 0, minQuantity: 0, location: '' })
    showForm.value = false
    load()
  } catch (e) { error.value = e.message }
}
async function open(item) {
  selected.value = item
  Object.assign(move, { reason: 'in', quantity: 1, note: '' })
  history.value = await api(`/inventory/${item.id}/movements`)
}
async function submitMove() {
  error.value = ''
  try {
    selected.value = await api(`/inventory/${selected.value.id}/movements`, { method: 'POST', body: { ...move } })
    history.value = await api(`/inventory/${selected.value.id}/movements`)
    load()
  } catch (e) { error.value = e.message }
}
const pct = (i) => Math.min(100, Math.round((i.quantity / Math.max(1, i.minQuantity * 2)) * 100))
</script>

<template>
  <div class="head">
    <h1>Estoque</h1>
    <input v-model="q" class="search" type="search" placeholder="Buscar por nome ou SKU" />
    <button @click="showForm = !showForm">{{ showForm ? 'Cancelar' : 'Novo item' }}</button>
  </div>
  <p v-if="error" class="error" role="alert">{{ error }}</p>

  <form v-if="showForm" class="card formgrid" @submit.prevent="create">
    <label>SKU <input v-model="form.sku" required /></label>
    <label>Nome <input v-model="form.name" required /></label>
    <label>Categoria <input v-model="form.category" /></label>
    <label>Quantidade inicial <input v-model.number="form.quantity" type="number" min="0" /></label>
    <label>Estoque mínimo <input v-model.number="form.minQuantity" type="number" min="0" /></label>
    <label>Local <input v-model="form.location" /></label>
    <button>Salvar item</button>
  </form>

  <div class="split">
    <div>
      <p v-if="!items.length" class="empty">Nenhum item. Use “Novo item” para cadastrar peças e consumíveis.</p>
      <table v-else>
        <thead><tr><th>SKU</th><th>Item</th><th>Saldo</th><th>Mínimo</th><th></th></tr></thead>
        <tbody>
          <tr v-for="i in items" :key="i.id" :class="{ sel: selected?.id === i.id }">
            <td>{{ i.sku }}</td>
            <td>{{ i.name }}<br /><small>{{ i.category }} · {{ i.location }}</small></td>
            <td :class="{ breach: i.low }">
              {{ i.quantity }}
              <svg class="meter" viewBox="0 0 100 6" preserveAspectRatio="none" aria-hidden="true">
                <rect width="100" height="6" class="track" /><rect :width="pct(i)" height="6" :fill="i.low ? 'var(--bad)' : 'var(--accent)'" />
              </svg>
            </td>
            <td>{{ i.minQuantity }}</td>
            <td><button class="ghost" @click="open(i)">Movimentar</button></td>
          </tr>
        </tbody>
      </table>
    </div>

    <aside v-if="selected" class="card">
      <h2>{{ selected.name }} <small>saldo {{ selected.quantity }}</small></h2>
      <form @submit.prevent="submitMove">
        <label>Tipo
          <select v-model="move.reason"><option v-for="(l, k) in REASON" :key="k" :value="k">{{ l }}</option></select>
        </label>
        <label>{{ move.reason === 'adjust' ? 'Variação (use − para baixar)' : 'Quantidade' }}
          <input v-model.number="move.quantity" type="number" :min="move.reason === 'adjust' ? undefined : 1" required />
        </label>
        <label>Observação <input v-model="move.note" /></label>
        <button>Registrar movimentação</button>
      </form>
      <h2>Histórico</h2>
      <ul class="thread">
        <li v-for="m in history" :key="m.id">
          <small>{{ fmtDateTime(m.createdAt) }} · {{ m.userName }}</small>
          <p><b :class="{ breach: m.delta < 0 }">{{ m.delta > 0 ? '+' : '' }}{{ m.delta }}</b> {{ REASON[m.reason] }}
            <template v-if="m.ticketId"> · chamado #{{ m.ticketId }}</template>
            <template v-if="m.note"> — {{ m.note }}</template></p>
        </li>
        <li v-if="!history.length" class="empty">Sem movimentações.</li>
      </ul>
    </aside>
  </div>
</template>
