export const STATUS = {
  new: 'Novo', open: 'Aberto', in_progress: 'Em atendimento',
  pending: 'Aguardando cliente', resolved: 'Resolvido', closed: 'Fechado',
}
export const PRIORITY = { critical: 'Crítica', high: 'Alta', medium: 'Média', low: 'Baixa' }

// Espelha a máquina de estados do backend só para renderizar botões;
// a validação real é sempre do servidor (409 em transição inválida).
export const NEXT = {
  new: ['open'], open: ['in_progress', 'pending', 'resolved'],
  in_progress: ['open', 'pending', 'resolved'], pending: ['in_progress', 'resolved'],
  resolved: ['open', 'closed'], closed: [],
}

export function slaLabel(t) {
  if (t.slaBreached) return 'Violado'
  if (['resolved', 'closed'].includes(t.status)) return 'Cumprido'
  const mins = Math.round((new Date(t.slaDueAt) - Date.now()) / 60000)
  if (mins < 60) return `${Math.max(mins, 0)} min restantes`
  return `${Math.floor(mins / 60)} h restantes`
}

export const ASSET_STATUS = { in_stock: 'Em estoque', in_use: 'Em uso', maintenance: 'Em manutenção', retired: 'Baixado' }
export const ASSET_TYPE = {
  laptop: 'Notebook', desktop: 'Desktop', monitor: 'Monitor', phone: 'Celular',
  server: 'Servidor', printer: 'Impressora', network: 'Rede', other: 'Outro',
}
export const REASON = { in: 'Entrada', out: 'Saída', adjust: 'Ajuste de inventário' }
// Rótulo de qualquer chave conhecida (status de chamado, prioridade ou de ativo).
export const labelOf = (k) => STATUS[k] || PRIORITY[k] || ASSET_STATUS[k] || k
export const fmtDate = (d) => (d ? new Date(d).toLocaleDateString('pt-BR') : '—')
export const fmtDateTime = (d) => new Date(d).toLocaleString('pt-BR')
