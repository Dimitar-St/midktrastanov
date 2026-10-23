<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { CalendarClock, ExternalLink, History, MapPin, Search, X } from 'lucide-vue-next'
import { api, formatDate, formatTime, type AddressSummary, type Event } from './api'

const addresses = ref<AddressSummary[]>([])
const search = ref('')
const loading = ref(true)
const error = ref('')
const dialog = ref<HTMLDialogElement>()
const selected = ref<AddressSummary>()
const history = ref<Event[]>([])
const historyLoading = ref(false)
const historyError = ref('')
const filtered = computed(() => addresses.value.filter(item => item.address.toLocaleLowerCase().includes(search.value.toLocaleLowerCase())))
function eventLabel(event?: Event) { return event ? `${formatDate(event.startsAt)} · ${formatTime(event.startsAt)}` : 'Няма запис' }
function googleMapsUrl(address: string) { return `https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(address)}` }
async function load() { loading.value = true; error.value = ''; try { addresses.value = await api<AddressSummary[]>('/addresses') } catch (cause) { error.value = (cause as Error).message } finally { loading.value = false } }
async function openAddress(item: AddressSummary) {
 selected.value = item; history.value = []; historyError.value = ''; historyLoading.value = true
 dialog.value?.showModal()
 try { history.value = await api<Event[]>(`/addresses/detail?address=${encodeURIComponent(item.address)}`) } catch (cause) { historyError.value = (cause as Error).message } finally { historyLoading.value = false }
}
function closeDialog() { dialog.value?.close() }
onMounted(load)
</script>

<template>
 <section class="page-heading"><div><p class="eyebrow">АДРЕСИ</p><h1>История по адрес</h1><p>Вижте последното и следващото планирано посещение за всеки адрес.</p></div></section>
 <section class="panel addresses-panel"><label class="search"><Search :size="17" /><input v-model="search" placeholder="Търсене по адрес" /></label><p v-if="error" class="error" role="alert">{{ error }}</p><p v-else-if="loading" class="empty">Зареждане на адреси...</p><p v-else-if="!filtered.length" class="empty">Няма адреси, които да отговарят на търсенето.</p><article v-for="item in filtered" :key="item.address" class="address-row"><button type="button" class="address-open" @click="openAddress(item)"><div class="address-name"><MapPin :size="20" /><strong>{{ item.address }}</strong></div><div><small><History :size="14" />ПРЕДИШНО</small><p>{{ eventLabel(item.previousEvent) }}</p><span v-if="item.previousEvent">{{ item.previousEvent.title }}</span></div><div><small><CalendarClock :size="14" />СЛЕДВАЩО</small><p>{{ eventLabel(item.nextEvent) }}</p><span v-if="item.nextEvent">{{ item.nextEvent.title }}</span></div></button><a class="map-link" :href="googleMapsUrl(item.address)" target="_blank" rel="noopener noreferrer">Отвори в Google Maps <ExternalLink :size="15" /></a></article></section>
 <dialog ref="dialog" class="address-dialog" @click.self="closeDialog"><header><div><p class="eyebrow">АДРЕС</p><h2>{{ selected?.address }}</h2><a v-if="selected" class="map-link" :href="googleMapsUrl(selected.address)" target="_blank" rel="noopener noreferrer">Отвори в Google Maps <ExternalLink :size="15" /></a></div><button type="button" class="dialog-close" aria-label="Затвори" @click="closeDialog"><X :size="20" /></button></header><section class="history-content"><p v-if="historyError" class="error" role="alert">{{ historyError }}</p><p v-else-if="historyLoading" class="empty">Зареждане на историята...</p><p v-else-if="!history.length" class="empty">Все още няма минали или текущи събития на този адрес.</p><template v-else><h3>Текущи и минали събития</h3><article v-for="event in history" :key="`${event.id}-${event.startsAt}`" class="history-event"><div class="history-time"><strong>{{ formatDate(event.startsAt) }}</strong><span>{{ formatTime(event.startsAt) }} - {{ formatTime(event.endsAt) }}</span></div><div><h3>{{ event.title }}</h3><p v-if="event.description">{{ event.description }}</p><small>Създадено от {{ event.createdBy }}<template v-if="event.assignedTo"> · Възложено на {{ event.assignedTo }}</template></small></div></article></template></section></dialog>
</template>
