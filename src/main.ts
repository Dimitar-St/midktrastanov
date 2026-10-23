import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import Schedule from './Schedule.vue'
import Addresses from './Addresses.vue'
import MapOverview from './MapOverview.vue'
import CreateEvent from './CreateEvent.vue'
import './style.css'
import './address-details.css'
import './type-scale.css'
import './maps.css'
import './schedule-details.css'
import './week-layout.css'

const router = createRouter({ history: createWebHistory(), routes: [
 { path: '/', redirect: '/day' },
 { path: '/day', component: Schedule, props: { view: 'day' } },
 { path: '/week', component: Schedule, props: { view: 'week' } },
 { path: '/month', component: Schedule, props: { view: 'month' } },
 { path: '/addresses', component: Addresses },
 { path: '/map', component: MapOverview },
 { path: '/events/new', component: CreateEvent },
 { path: '/:pathMatch(.*)*', redirect: '/day' },
] })

createApp(App).use(router).mount('#app')
