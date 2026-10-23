<script setup lang="ts">
import { computed, ref } from 'vue'
import { LogIn, UserPlus } from 'lucide-vue-next'
import { api, setSessionToken, type User } from './api'

const emit = defineEmits<{ authenticated: [user: User] }>()
const registering = ref(false)
const companyName = ref('')
const name = ref('')
const username = ref('')
const password = ref('')
const confirmPassword = ref('')
const error = ref('')
const submitting = ref(false)
const title = computed(() => registering.value ? 'Създаване на профил' : 'Вход в графика')
const intro = computed(() => registering.value ? 'Създайте компания и първия профил, с който да управлявате графика.' : 'Въведете данните си, за да видите събитията на вашата компания.')

function switchMode() {
 registering.value = !registering.value
 error.value = ''
 password.value = ''
 confirmPassword.value = ''
}
async function submit() {
 if (submitting.value) return
 if (registering.value && password.value !== confirmPassword.value) { error.value = 'Паролите не съвпадат.'; return }
 submitting.value = true; error.value = ''
 try {
  const path = registering.value ? '/auth/register' : '/auth/login'
  const body = registering.value ? { companyName: companyName.value, name: name.value, username: username.value, password: password.value } : { username: username.value, password: password.value }
  const result = await api<{ token: string; user: User }>(path, { method: 'POST', body: JSON.stringify(body) })
  setSessionToken(result.token)
  emit('authenticated', result.user)
 } catch (cause) { error.value = (cause as Error).message } finally { submitting.value = false }
}
</script>

<template>
 <main class="login-page"><form class="login-card" @submit.prevent="submit"><span class="logo">ДДД</span><div><h1>{{ title }}</h1><p>{{ intro }}</p></div><p v-if="error" class="error" role="alert">{{ error }}</p><template v-if="registering"><label>ИМЕ НА КОМПАНИЯТА<input v-model="companyName" autocomplete="organization" minlength="2" maxlength="200" required autofocus /></label><label>ВАШЕТО ИМЕ<input v-model="name" autocomplete="name" minlength="2" maxlength="100" required /></label></template><label>ПОТРЕБИТЕЛСКО ИМЕ<input v-model="username" autocomplete="username" required :autofocus="!registering" /></label><label>ПАРОЛА<input v-model="password" type="password" :autocomplete="registering ? 'new-password' : 'current-password'" maxlength="72" required /></label><label v-if="registering">ПОТВЪРДЕТЕ ПАРОЛАТА<input v-model="confirmPassword" type="password" autocomplete="new-password" maxlength="72" required /></label><button class="button primary" :disabled="submitting"><component :is="registering ? UserPlus : LogIn" :size="18" />{{ submitting ? 'Изчакване...' : registering ? 'Създай профил' : 'Вход' }}</button><button type="button" class="auth-toggle" @click="switchMode">{{ registering ? 'Вече имате профил? Влезте' : 'Нямате профил? Регистрирайте се' }}</button></form></main>
</template>
