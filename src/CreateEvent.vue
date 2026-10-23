<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeft, CalendarPlus, Repeat2 } from 'lucide-vue-next'
import { api, dateKey, type User } from './api'

const router = useRouter()
const users = ref<User[]>([])
const error = ref('')
const saving = ref(false)
const form = ref({ title: '', address: '', date: dateKey(new Date()), startTime: '09:00', endTime: '10:00', recurrenceMonths: 3, assignedTo: '', description: '' })
onMounted(async () => { try { users.value = await api<User[]>('/users') } catch (cause) { error.value = (cause as Error).message } })
async function save() {
 if (saving.value) return
 saving.value = true; error.value = ''
 try {
  const startsAt = new Date(`${form.value.date}T${form.value.startTime}:00`)
  const endsAt = new Date(`${form.value.date}T${form.value.endTime}:00`)
  await api('/events', { method: 'POST', body: JSON.stringify({ title: form.value.title, address: form.value.address, startsAt: startsAt.toISOString(), endsAt: endsAt.toISOString(), recurrenceMonths: form.value.recurrenceMonths, assignedTo: form.value.assignedTo || undefined, description: form.value.description }) })
  await router.push('/day')
 } catch (cause) { error.value = (cause as Error).message } finally { saving.value = false }
}
</script>

<template>
 <RouterLink to="/day" class="back-link"><ArrowLeft :size="16" />Към графика</RouterLink><section class="page-heading"><div><p class="eyebrow">НОВО СЪБИТИЕ</p><h1>Насрочване на посещение</h1><p>Адресът се пази към събитието, за да бъде лесно проследим при следващи посещения.</p></div></section>
 <form class="panel event-form" @submit.prevent="save"><p v-if="error" class="error" role="alert">{{ error }}</p><label>ЗАГЛАВИЕ *<input v-model="form.title" required maxlength="200" placeholder="Напр. Периодична обработка" /></label><label>АДРЕС *<input v-model="form.address" required maxlength="500" placeholder="Град, улица, номер" /></label><div class="form-grid"><label>ДАТА *<input v-model="form.date" type="date" required /></label><label>НАЧАЛЕН ЧАС *<input v-model="form.startTime" type="time" required /></label><label>КРАЕН ЧАС *<input v-model="form.endTime" type="time" required /></label><label>ВЪЗЛОЖЕНО НА<select v-model="form.assignedTo"><option value="">Без назначен</option><option v-for="user in users" :key="user.id" :value="user.id">{{ user.name }}</option></select></label></div><label>ПОВТОРЕНИЕ<div class="interval-options"><button v-for="months in [0, 3, 6, 12]" :key="months" type="button" :class="{ selected: form.recurrenceMonths === months }" @click="form.recurrenceMonths = months"><Repeat2 :size="15" />{{ months ? `На ${months} месеца` : 'Без повторение' }}</button></div></label><label>БЕЛЕЖКИ<textarea v-model="form.description" rows="4" maxlength="5000" placeholder="Детайли за достъп или работа на адреса" /></label><div class="form-actions"><RouterLink to="/day" class="button secondary">Отказ</RouterLink><button class="button primary" :disabled="saving"><CalendarPlus :size="18" />{{ saving ? 'Запазване...' : 'Запази събитието' }}</button></div></form>
</template>
