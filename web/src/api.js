// Camada única de acesso HTTP: injeta o JWT e normaliza erros.
export const getToken = () => localStorage.getItem('goticket.token')

export async function api(path, { method = 'GET', body } = {}) {
  const res = await fetch(`/api${path}`, {
    method,
    headers: {
      'Content-Type': 'application/json',
      ...(getToken() && { Authorization: `Bearer ${getToken()}` }),
    },
    body: body ? JSON.stringify(body) : undefined,
  })
  const data = res.status === 204 ? null : await res.json().catch(() => null)
  if (!res.ok) {
    const err = new Error(data?.error || 'Erro inesperado')
    err.status = res.status
    throw err
  }
  return data
}
