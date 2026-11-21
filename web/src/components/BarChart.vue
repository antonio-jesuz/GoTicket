<script setup>
import { computed } from 'vue'
// Barras horizontais em SVG puro: sem biblioteca, legível e acessível.
const props = defineProps({
  data: { type: Array, required: true }, // [{ label, value }]
  color: { type: String, default: 'var(--accent)' },
})
const max = computed(() => Math.max(1, ...props.data.map((d) => d.value)))
</script>

<template>
  <ul class="bars" role="list">
    <li v-for="d in data" :key="d.label">
      <span class="bar-label">{{ d.label }}</span>
      <svg class="bar-track" viewBox="0 0 100 8" preserveAspectRatio="none" aria-hidden="true">
        <rect width="100" height="8" rx="2" class="track" />
        <rect :width="(d.value / max) * 100" height="8" rx="2" :fill="color" />
      </svg>
      <span class="bar-value">{{ d.value }}</span>
    </li>
    <li v-if="!data.length" class="empty">Sem dados no período.</li>
  </ul>
</template>

<style scoped>
.bars { list-style: none; margin: 0; padding: 0; display: grid; gap: 8px; }
.bars li { display: grid; grid-template-columns: 130px 1fr 36px; align-items: center; gap: 10px; font-size: .88rem; }
.bar-track { width: 100%; height: 10px; } .track { fill: var(--line); }
.bar-value { text-align: right; font-variant-numeric: tabular-nums; }
</style>
