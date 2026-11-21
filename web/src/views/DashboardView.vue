<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { api } from '../api'
import { useTicketStore } from '../stores/tickets'
import { labelOf } from '../labels'
import BarChart from '../components/BarChart.vue'
import DonutChart from '../components/DonutChart.vue'
import LineChart from '../components/LineChart.vue'

const store = useTicketStore()
const s = ref(null)
const error = ref('')
const load = async () => {
  try { s.value = await api('/reports/summary') } catch (e) { error.value = e.message }
}
onMounted(load)
watch(() => store.lastEvent, load) // painel acompanha o tempo real

const named = (list) => (list || []).map((c) => ({ label: labelOf(c.label), value: c.value }))
const compliance = computed(() => (s.value?.slaCompliance == null ? '—' : `${s.value.slaCompliance.toFixed(0)}%`))
const avg = computed(() => (s.value?.avgResolutionHours == null ? '—' : `${s.value.avgResolutionHours.toFixed(1)} h`))
</script>

<template>
  <h1>Painel</h1>
  <p v-if="error" class="error" role="alert">{{ error }}</p>
  <template v-if="s">
    <section class="kpis">
      <router-link to="/" class="kpi"><b>{{ s.open }}</b><span>Chamados abertos</span></router-link>
      <router-link :to="{ name: 'tickets', query: { assignee: 'none' } }" class="kpi">
        <b>{{ s.unassigned }}</b><span>Sem agente</span>
      </router-link>
      <div class="kpi" :class="{ bad: s.slaBreached }"><b>{{ s.slaBreached }}</b><span>SLA violado (abertos)</span></div>
      <div class="kpi"><b>{{ compliance }}</b><span>Resolvidos no prazo (30 dias)</span></div>
      <div class="kpi"><b>{{ avg }}</b><span>Tempo médio de resolução</span></div>
    </section>

    <section class="grid2">
      <div class="card wide"><h2>Chamados por dia (14 dias)</h2><LineChart :data="s.daily" /></div>
      <div class="card"><h2>Por status</h2><DonutChart :data="named(s.byStatus)" /></div>
      <div class="card"><h2>Abertos por prioridade</h2><BarChart :data="named(s.byPriority)" color="var(--warn)" /></div>
      <div class="card"><h2>Por categoria</h2><BarChart :data="s.byCategory" /></div>
      <div class="card"><h2>Ativos por status</h2><DonutChart :data="named(s.assetsByStatus)" /></div>
      <div class="card wide">
        <h2>Estoque baixo</h2>
        <p v-if="!s.lowStock.length" class="empty">Nenhum item abaixo do mínimo.</p>
        <table v-else>
          <thead><tr><th>SKU</th><th>Item</th><th>Saldo</th><th>Mínimo</th></tr></thead>
          <tbody>
            <tr v-for="i in s.lowStock" :key="i.id">
              <td>{{ i.sku }}</td><td>{{ i.name }}</td><td class="breach">{{ i.quantity }}</td><td>{{ i.minQuantity }}</td>
            </tr>
          </tbody>
        </table>
        <router-link to="/estoque">Ir para o estoque</router-link>
      </div>
    </section>
  </template>
</template>
