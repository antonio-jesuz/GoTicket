<script setup>
import { computed } from 'vue'
// Duas séries ao longo dos dias: abertos x resolvidos.
const props = defineProps({ data: { type: Array, required: true } }) // [{ date, created, resolved }]
const W = 600, H = 180, PAD = { l: 28, r: 8, t: 10, b: 22 }
const max = computed(() => Math.max(4, ...props.data.flatMap((d) => [d.created, d.resolved])))
const x = (i) => PAD.l + (i * (W - PAD.l - PAD.r)) / Math.max(1, props.data.length - 1)
const y = (v) => H - PAD.b - (v / max.value) * (H - PAD.t - PAD.b)
const path = (key) => props.data.map((d, i) => `${i ? 'L' : 'M'}${x(i).toFixed(1)},${y(d[key]).toFixed(1)}`).join(' ')
const ticks = computed(() => [0, Math.round(max.value / 2), max.value])
const label = (d) => d.date.slice(8) + '/' + d.date.slice(5, 7)
</script>

<template>
  <figure>
    <svg :viewBox="`0 0 ${W} ${H}`" role="img" aria-label="Chamados abertos e resolvidos por dia">
      <g v-for="t in ticks" :key="t">
        <line :x1="PAD.l" :x2="W - PAD.r" :y1="y(t)" :y2="y(t)" class="grid" />
        <text :x="PAD.l - 6" :y="y(t) + 3" text-anchor="end" class="tick">{{ t }}</text>
      </g>
      <path :d="path('created')" fill="none" stroke="#2f6fb5" stroke-width="2" />
      <path :d="path('resolved')" fill="none" stroke="#0b6e69" stroke-width="2" stroke-dasharray="5 3" />
      <text v-for="(d, i) in data" :key="d.date" v-show="i % 3 === 0 || i === data.length - 1"
            :x="x(i)" :y="H - 6" text-anchor="middle" class="tick">{{ label(d) }}</text>
    </svg>
    <figcaption>
      <span><i style="background:#2f6fb5" /> Abertos</span>
      <span><i class="dash" /> Resolvidos</span>
    </figcaption>
  </figure>
</template>

<style scoped>
figure { margin: 0; } svg { width: 100%; height: auto; }
.grid { stroke: var(--line); } .tick { font-size: 10px; fill: var(--muted); }
figcaption { display: flex; gap: 16px; font-size: .85rem; color: var(--muted); }
i { display: inline-block; width: 14px; height: 3px; margin-right: 6px; vertical-align: middle; }
i.dash { background: repeating-linear-gradient(90deg, #0b6e69 0 5px, transparent 5px 8px); }
</style>
