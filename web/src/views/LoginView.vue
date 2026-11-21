<script setup>
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()
const mode = ref('login')
const form = reactive({ name: '', email: '', password: '' })
const error = ref('')

async function submit() {
  error.value = ''
  try {
    if (mode.value === 'login') await auth.login(form.email, form.password)
    else await auth.register(form.email, form.name, form.password)
    router.push({ name: auth.isAgent ? 'dashboard' : 'tickets' })
  } catch (e) {
    error.value = e.message
  }
}
</script>

<template>
  <section class="auth">
    <h1>{{ mode === 'login' ? 'Entrar no GoTicket' : 'Criar conta de cliente' }}</h1>
    <form @submit.prevent="submit">
      <label v-if="mode === 'register'">Nome <input v-model="form.name" required /></label>
      <label>E-mail <input v-model="form.email" type="email" required /></label>
      <label>Senha <input v-model="form.password" type="password" minlength="8" required /></label>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <button>{{ mode === 'login' ? 'Entrar' : 'Criar conta' }}</button>
    </form>
    <button class="link" @click="mode = mode === 'login' ? 'register' : 'login'">
      {{ mode === 'login' ? 'Ainda não tenho conta' : 'Já tenho conta' }}
    </button>
  </section>
</template>
