<script setup>
import { onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../api'
import { useAuthStore } from '../stores/auth'
import { useTicketStore } from '../stores/tickets'
import { NEXT, PRIORITY, STATUS, fmtDateTime, slaLabel } from '../labels'

const route = useRoute()
const auth = useAuthStore()
const store = useTicketStore()
const detail = ref(null)
const agents = ref([])
const stock = ref([])
const body = ref('')
const internal = ref(false)
const part = reactive({ itemId: '', quantity: 1 })
const error = ref('')
const id = () => route.params.id

const load = async () => {
  try { detail.value = await api(`/tickets/${id()}`) } catch (e) { error.value = e.message }
}
onMounted(async () => {
  load()
  if (auth.isAgent) {
    agents.value = await api('/users?role=agent')
    stock.value = await api('/inventory')
  }
})
// Recarrega quando chega evento em tempo real deste chamado (comentário, status, peça).
watch(() => store.lastEvent, (e) => { if (e?.ticketId === Number(id())) load() })

async function act(path, payload) {
  error.value = ''
  try { await api(`/tickets/${id()}/${path}`, { method: 'POST', body: payload }); await load() }
  catch (e) { error.value = e.message }
}
const send = async () => { await act('comments', { body: body.value, internal: internal.value }); body.value = '' }
async function usePart() {
  error.value = ''
  try {
    await api(`/inventory/${part.itemId}/movements`, {
      method: 'POST', body: { reason: 'out', quantity: part.quantity, ticketId: Number(id()), note: 'Uso em chamado' },
    })
    part.itemId = ''; part.quantity = 1
    stock.value = await api('/inventory')
    await load()
  } catch (e) { error.value = e.message }
}
</script>

<template>
  <router-link to="/">← Chamados</router-link>
  <p v-if="error" class="error" role="alert">{{ error }}</p>
  <template v-if="detail">
    <div class="head">
      <h1>{{ detail.ticket.number }} · {{ detail.ticket.title }}</h1>
      <span class="pill" :class="detail.ticket.priority">{{ PRIORITY[detail.ticket.priority] }}</span>
    </div>

    <dl class="facts card">
      <div><dt>Status</dt><dd>{{ STATUS[detail.ticket.status] }}</dd></div>
      <div><dt>SLA</dt><dd :class="{ breach: detail.ticket.slaBreached }">{{ slaLabel(detail.ticket) }}</dd></div>
      <div><dt>Solicitante</dt><dd>{{ detail.requesterName }}</dd></div>
      <div><dt>Responsável</dt><dd>{{ detail.assigneeName || 'Sem agente' }}</dd></div>
      <div><dt>Categoria</dt><dd>{{ detail.categoryName || '—' }}</dd></div>
      <div><dt>Ativo</dt><dd>{{ detail.assetLabel || '—' }}</dd></div>
    </dl>
    <p>{{ detail.ticket.description || 'Sem descrição.' }}</p>

    <div v-if="auth.isAgent" class="actions">
      <select :value="detail.ticket.assigneeId || ''" aria-label="Atribuir a"
              @change="$event.target.value && act('assign', { assigneeId: Number($event.target.value) })">
        <option value="">Atribuir a…</option>
        <option v-for="a in agents" :key="a.id" :value="a.id">{{ a.name }}</option>
      </select>
      <button v-for="s in NEXT[detail.ticket.status]" :key="s" class="ghost" @click="act('transition', { to: s })">
        Mover para {{ STATUS[s] }}
      </button>
    </div>

    <h2>Conversa</h2>
    <ul class="thread">
      <li v-for="c in detail.comments" :key="c.id" :class="{ internal: c.internal }">
        <small>{{ c.authorName }} · {{ fmtDateTime(c.createdAt) }}<template v-if="c.internal"> · nota interna</template></small>
        <p>{{ c.body }}</p>
      </li>
      <li v-if="!detail.comments.length" class="empty">Sem mensagens ainda.</li>
    </ul>
    <form class="card" @submit.prevent="send">
      <textarea v-model="body" rows="3" placeholder="Escreva uma mensagem" required />
      <label v-if="auth.isAgent" class="inline"><input v-model="internal" type="checkbox" /> Nota interna (o cliente não vê)</label>
      <button>Enviar mensagem</button>
    </form>

    <template v-if="detail.parts.length || auth.isAgent">
      <h2>Peças utilizadas</h2>
      <ul class="plain">
        <li v-for="(p, i) in detail.parts" :key="i">{{ p.quantity }}× {{ p.name }} <small>{{ fmtDateTime(p.at) }}</small></li>
        <li v-if="!detail.parts.length" class="empty">Nenhuma peça registrada.</li>
      </ul>
      <form v-if="auth.isAgent" class="inlineform" @submit.prevent="usePart">
        <select v-model="part.itemId" required aria-label="Peça">
          <option value="" disabled>Escolha a peça</option>
          <option v-for="i in stock" :key="i.id" :value="i.id" :disabled="i.quantity === 0">{{ i.name }} (saldo {{ i.quantity }})</option>
        </select>
        <input v-model.number="part.quantity" type="number" min="1" aria-label="Quantidade" />
        <button class="ghost">Baixar do estoque</button>
      </form>
    </template>

    <h2>Histórico</h2>
    <ol class="timeline">
      <li v-for="e in detail.events" :key="e.id">
        <small>{{ fmtDateTime(e.createdAt) }}</small> {{ e.actorName }}: {{ e.detail }}
      </li>
    </ol>
  </template>
</template>
