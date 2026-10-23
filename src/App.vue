<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Calendar, CalendarDays, CalendarRange, LogOut, Map, MapPinned, Plus } from 'lucide-vue-next'
import Login from './Login.vue'
import { api, clearSession, sessionToken, type User } from './api'

const router = useRouter()
const user = ref<User>()
const loading = ref(Boolean(sessionToken()))

async function loadUser() {
 if (!sessionToken()) { loading.value = false; return }
 try { user.value = await api<User>('/auth/me') } catch { user.value = undefined } finally { loading.value = false }
}
function signedIn(authenticated: User) { user.value = authenticated; loading.value = false; router.replace('/day') }
function signOut() { clearSession(); user.value = undefined; router.replace('/day') }
function expired() { user.value = undefined; loading.value = false; router.replace('/day') }
onMounted(() => { void loadUser(); window.addEventListener('session-expired', expired) })
onUnmounted(() => window.removeEventListener('session-expired', expired))
</script>

<template>
 <p v-if="loading" class="app-loading">Зареждане...</p>
 <Login v-else-if="!user" @authenticated="signedIn" />
 <template v-else>
  <aside class="sidebar">
   <RouterLink to="/day" class="brand"><span class="logo">ДДД</span><span>БулДез Контрол<small>График на посещенията</small></span></RouterLink>
   <nav class="main-nav">
    <RouterLink to="/day"><CalendarDays :size="19" />График</RouterLink>
     <RouterLink to="/week"><CalendarRange :size="19" />Седмица</RouterLink>
     <RouterLink to="/month"><Calendar :size="19" />Месец</RouterLink>
     <RouterLink to="/addresses"><MapPinned :size="19" />Адреси</RouterLink>
     <RouterLink to="/map"><Map :size="19" />Карта</RouterLink>
    <RouterLink to="/events/new"><Plus :size="19" />Ново събитие</RouterLink>
   </nav>
   <div class="account"><span class="avatar">{{ user.name.slice(0, 2).toUpperCase() }}</span><div><strong>{{ user.name }}</strong><small>{{ user.username }}</small></div><button aria-label="Изход" title="Изход" @click="signOut"><LogOut :size="17" /></button></div>
  </aside>
  <div class="shell">
   <header class="topbar"><span>Планиране на обработки</span><strong class="company-name">{{ user.company?.name }}</strong></header>
   <main><RouterView /></main>
  </div>
  <nav class="mobile-nav"><RouterLink to="/day"><CalendarDays :size="19" />График</RouterLink><RouterLink to="/month"><Calendar :size="19" />Месец</RouterLink><RouterLink to="/addresses"><MapPinned :size="19" />Адреси</RouterLink><RouterLink to="/map"><Map :size="19" />Карта</RouterLink><RouterLink to="/events/new"><Plus :size="19" />Ново</RouterLink></nav>
 </template>
</template>
