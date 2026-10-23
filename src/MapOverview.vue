<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ExternalLink, MapPin } from 'lucide-vue-next'
import { api, type AddressSummary } from './api'

declare global { interface Window { google?: any; googleMapsPromise?: Promise<void> } }

const apiKey = import.meta.env.VITE_GOOGLE_MAPS_API_KEY
const addresses = ref<AddressSummary[]>([])
const loading = ref(true)
const error = ref('')
const mapError = ref('')
const mapCanvas = ref<HTMLElement>()

function googleMapsUrl(address: string) { return `https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(address)}` }
function loadGoogleMaps() {
 if (window.google?.maps) return Promise.resolve()
 if (window.googleMapsPromise) return window.googleMapsPromise
 window.googleMapsPromise = new Promise((resolve, reject) => {
  const script = document.createElement('script')
  script.src = `https://maps.googleapis.com/maps/api/js?key=${encodeURIComponent(apiKey)}&libraries=places&language=bg&region=BG`
  script.async = true
  script.onload = () => resolve()
  script.onerror = () => reject(new Error('Google Maps could not be loaded.'))
  document.head.append(script)
 })
 return window.googleMapsPromise
}
async function showMap() {
 if (!apiKey || !mapCanvas.value || !addresses.value.length) return
 try {
  await loadGoogleMaps()
  const map = new window.google.maps.Map(mapCanvas.value, { center: { lat: 42.7339, lng: 25.4858 }, zoom: 7, mapTypeControl: false, streetViewControl: false })
  const bounds = new window.google.maps.LatLngBounds()
  const { Place } = await window.google.maps.importLibrary('places')
  const info = new window.google.maps.InfoWindow()
  let found = 0
  for (const item of addresses.value) {
   const { places } = await Place.searchByText({ textQuery: item.address, fields: ['displayName', 'location'] })
   const place = places[0]
   if (place?.location) {
     const marker = new window.google.maps.Marker({ map, position: place.location, title: item.address })
     marker.addListener('click', () => { const link = document.createElement('a'); link.href = googleMapsUrl(item.address); link.target = '_blank'; link.rel = 'noopener noreferrer'; link.textContent = item.address; info.setContent(link); info.open({ map, anchor: marker }) })
     bounds.extend(place.location); found++
   }
  }
  if (found) map.fitBounds(bounds)
  else mapError.value = 'Не успяхме да намерим адресите в Google Maps.'
 } catch (cause) { mapError.value = (cause as Error).message }
}
async function load() { loading.value = true; error.value = ''; try { addresses.value = await api<AddressSummary[]>('/addresses'); await showMap() } catch (cause) { error.value = (cause as Error).message } finally { loading.value = false } }
onMounted(load)
</script>

<template>
 <section class="page-heading"><div><p class="eyebrow">КАРТА</p><h1>Карта на адресите</h1><p>Визуален преглед на всички адреси на вашата компания.</p></div></section>
 <section v-if="!apiKey" class="panel map-message"><MapPin :size="24" /><div><h2>Google Maps не е конфигуриран</h2><p>Добавете browser-restricted <code>VITE_GOOGLE_MAPS_API_KEY</code> в <code>.env</code>, след това рестартирайте Vite.</p></div></section>
 <section v-else class="panel map-panel"><p v-if="loading" class="empty">Зареждане на картата...</p><p v-else-if="error || mapError" class="error" role="alert">{{ error || mapError }}</p><div ref="mapCanvas" class="google-map" :class="{ hidden: loading || error || mapError }"></div></section>
 <section class="panel map-addresses"><h2>Адреси <span>{{ addresses.length }}</span></h2><a v-for="item in addresses" :key="item.address" :href="googleMapsUrl(item.address)" target="_blank" rel="noopener noreferrer"><MapPin :size="17" />{{ item.address }}<ExternalLink :size="14" /></a></section>
</template>
