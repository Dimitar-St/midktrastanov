<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { CalendarDays, ChevronLeft, ChevronRight, ExternalLink, MapPin, RefreshCw, Search, UserRound, X } from 'lucide-vue-next'
import { api, dateKey, formatDate, formatTime, parseDate, type Event } from './api'

const props = defineProps<{ view: 'day' | 'week' | 'month' }>()
const selected = ref(dateKey(new Date()))
const search = ref('')
const events = ref<Event[]>([])
const loading = ref(false)
const error = ref('')
const dialog = ref<HTMLDialogElement>()
const selectedEvent = ref<Event>()
const days = computed(() => {
 const start = parseDate(selected.value)
 if (props.view === 'week') start.setDate(start.getDate() - ((start.getDay() + 6) % 7))
 if (props.view === 'month') {
  start.setDate(1)
  start.setDate(start.getDate() - ((start.getDay() + 6) % 7))
  const end = parseDate(selected.value); end.setMonth(end.getMonth() + 1, 0); end.setDate(end.getDate() + (7 - ((end.getDay() + 6) % 7) - 1) % 7)
  return Array.from({ length: Math.round((end.getTime() - start.getTime()) / 86400000) + 1 }, (_, index) => { const day = new Date(start); day.setDate(start.getDate() + index); return dateKey(day) })
 }
 return Array.from({ length: props.view === 'week' ? 7 : 1 }, (_, index) => { const day = new Date(start); day.setDate(start.getDate() + index); return dateKey(day) })
})
const filtered = computed(() => events.value.filter(event => `${event.title} ${event.address} ${event.assignedTo || ''}`.toLocaleLowerCase().includes(search.value.toLocaleLowerCase())))
const title = computed(() => props.view === 'day' ? formatDate(`${selected.value}T12:00:00`) : props.view === 'month' ? parseDate(selected.value).toLocaleDateString('bg-BG', { month: 'long', year: 'numeric' }) : `${formatDate(`${days.value[0]}T12:00:00`)} - ${formatDate(`${days.value.at(-1)}T12:00:00`)}`)
const heading = computed(() => props.view === 'day' ? 'Дневен график' : props.view === 'week' ? 'Седмичен график' : 'Месечен график')
const grouped = computed(() => Object.fromEntries(days.value.map(day => [day, filtered.value.filter(event => event.startsAt.slice(0, 10) === day)])))
function isCurrentMonth(day: string) { return day.slice(0, 7) === selected.value.slice(0, 7) }
function googleMapsUrl(address: string) { return `https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(address)}` }
function openEvent(event: Event) { selectedEvent.value = event; dialog.value?.showModal() }
function closeDialog() { dialog.value?.close() }
let requestId = 0
async function load() {
 const id = ++requestId; loading.value = true; error.value = ''
 try { const result = await api<Event[]>(`/events?from=${days.value[0]}&to=${days.value.at(-1)}`); if (id === requestId) events.value = result } catch (cause) { if (id === requestId) error.value = (cause as Error).message } finally { if (id === requestId) loading.value = false }
}
function move(amount: number) {
 const day = parseDate(selected.value)
 if (props.view === 'month') { day.setDate(1); day.setMonth(day.getMonth() + amount) } else day.setDate(day.getDate() + amount * (props.view === 'week' ? 7 : 1))
 selected.value = dateKey(day)
}
watch([selected, () => props.view], load, { immediate: true })
</script>

<template>
 <section class="page-heading"><div><p class="eyebrow">ГРАФИК</p><h1>{{ heading }}</h1><p>Събитията са видими само за вашата компания.</p></div></section>
 <section class="panel schedule-panel"><div class="toolbar"><div class="date-controls"><button aria-label="Предишен период" @click="move(-1)"><ChevronLeft :size="19" /></button><strong>{{ title }}</strong><button aria-label="Следващ период" @click="move(1)"><ChevronRight :size="19" /></button><button class="today" @click="selected = dateKey(new Date())">Днес</button></div><div class="view-switch"><RouterLink to="/day">Ден</RouterLink><RouterLink to="/week">Седмица</RouterLink><RouterLink to="/month">Месец</RouterLink></div></div><div class="filter-row"><label class="search"><Search :size="17" /><input v-model="search" placeholder="Търсене по адрес или събитие" /></label><button class="icon-text" title="Обнови" @click="load"><RefreshCw :size="16" />Обнови</button></div>
  <p v-if="error" class="error" role="alert">{{ error }}</p><p v-else-if="loading" class="empty">Зареждане на графика...</p>
  <div v-else class="schedule-grid" :class="{ week: props.view === 'week', month: props.view === 'month' }"><section v-for="day in days" :key="day" class="day-column" :class="{ 'outside-month': props.view === 'month' && !isCurrentMonth(day) }"><h2><span v-if="props.view === 'month'" class="weekday">{{ parseDate(day).toLocaleDateString('bg-BG', { weekday: 'short' }) }}</span>{{ props.view === 'month' ? parseDate(day).getDate() : formatDate(`${day}T12:00:00`) }}</h2><p v-if="!grouped[day].length && props.view !== 'month'" class="empty">Няма събития</p><button v-for="event in grouped[day]" :key="`${event.id}-${event.startsAt}`" type="button" class="event-card" @click="openEvent(event)"><div class="event-time"><CalendarDays :size="15" />{{ formatTime(event.startsAt) }} - {{ formatTime(event.endsAt) }}</div><h3>{{ event.title }}</h3><p v-if="props.view !== 'month'"><MapPin :size="16" />{{ event.address }}</p><small v-if="event.assignedTo && props.view !== 'month'"><UserRound :size="14" />{{ event.assignedTo }}</small></button></section></div>
 </section>
 <dialog ref="dialog" class="event-dialog" @click.self="closeDialog"><header><div><p class="eyebrow">СЪБИТИЕ</p><h2>{{ selectedEvent?.title }}</h2></div><button type="button" class="dialog-close" aria-label="Затвори" @click="closeDialog"><X :size="20" /></button></header><section v-if="selectedEvent" class="event-detail-content"><div class="event-detail-time"><CalendarDays :size="19" /><div><strong>{{ formatDate(selectedEvent.startsAt) }}</strong><span>{{ formatTime(selectedEvent.startsAt) }} - {{ formatTime(selectedEvent.endsAt) }}</span></div></div><a class="event-detail-address" :href="googleMapsUrl(selectedEvent.address)" target="_blank" rel="noopener noreferrer"><MapPin :size="19" /><span>{{ selectedEvent.address }}</span><ExternalLink :size="15" /></a><p v-if="selectedEvent.description" class="event-detail-description">{{ selectedEvent.description }}</p><p class="event-detail-meta">Създадено от <strong>{{ selectedEvent.createdBy }}</strong><template v-if="selectedEvent.assignedTo"> · Възложено на <strong>{{ selectedEvent.assignedTo }}</strong></template></p></section></dialog>
</template>
