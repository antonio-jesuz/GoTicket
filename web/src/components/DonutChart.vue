<script setup>
import { computed } from 'vue'
const props = defineProps({ data: { type: Array, required: true } }) // [{ label, value }]
const palette = ['#0b6e69', '#2f6fb5', '#b45309', '#7a4fb0', '#b42318', '#5d6b7a']
const total = computed(() => props.data.reduce((s, d) => s + d.value, 0))
const R = 15.9155 // circunferência = 100, então o dasharray já é a porcentagem
const slices = computed(() => {
  let offset = 25 // começa às 12h
  return props.data.map((d, i) => {
    const pct = total.value ? (d.value / total.value) * 100 : 0
    const s = { ...d, pct, offset, color: palette[i % palette.length] }
    offset -= pct
    return s
  })
})
</script>

<template>
  <div class="donut">
    <svg viewBox="0 0 42 42" role="img" :aria-label="`Total: ${total}`">
      <circle cx="21" cy="21" :r="R" fill="none" stroke="var(--line)" stroke-width="5" />
      <circle v-for="s in slices" :key="s.label" cx="21" cy="21" :r="R" fill="none" :stroke="s.color"
              stroke-width="5" :stroke-dasharray="`${s.pct} ${100 - s.pct}`" :stroke-dashoffset="s.offset" />
      <text x="21" y="22.5" text-anchor="middle" class="total">{{ total }}</text>
    </svg>
    <ul>
      <li v-for="s in slices" :key="s.label">
        <i :style="{ background: s.color }" /> {{ s.label }} <b>{{ s.value }}</b>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.donut { display: flex; align-items: center; gap: 20px; }
svg { width: 140px; flex: none; } .total { font-size: 8px; font-weight: 700; fill: var(--ink); }
ul { list-style: none; margin: 0; padding: 0; font-size: .88rem; display: grid; gap: 4px; }
i { display: inline-block; width: 10px; height: 10px; border-radius: 2px; margin-right: 6px; }
</style>
