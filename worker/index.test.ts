import { test } from 'node:test'
import assert from 'node:assert/strict'
import worker from './index.ts'
const env = {BACKEND_URL:'https://backend.example',BACKEND_API_TOKEN:'private-token',FRONTEND_ORIGIN:'https://frontend.example',ASSETS:{fetch:async()=>new Response('app')}}
test('proxy preserves API path and forwards only the browser session',async()=>{
 const original=globalThis.fetch
 try { globalThis.fetch=async(input,init)=>{assert.equal(String(input),'https://backend.example/api/events?from=2026-10-13');const headers=new Headers(init?.headers);assert.equal(headers.get('Authorization'),'Bearer user-session');assert.equal(headers.get('X-Gateway-Token'),'private-token');assert.equal(headers.get('Cookie'),null);return Response.json([])}
 const response=await worker.fetch(new Request('https://frontend.example/api/events?from=2026-10-13',{headers:{Origin:'https://frontend.example',Authorization:'Bearer user-session',Cookie:'secret'}}),env)
 assert.equal(response.status,200);assert.equal(response.headers.get('Access-Control-Allow-Origin'),'https://frontend.example');assert.equal(response.headers.get('Cache-Control'),'no-store')
 }finally{globalThis.fetch=original}
})
test('rejects foreign origins and handles preflight',async()=>{assert.equal((await worker.fetch(new Request('https://frontend.example/api/events',{headers:{Origin:'https://evil.example'}}),env)).status,403);assert.equal((await worker.fetch(new Request('https://frontend.example/api/events',{method:'OPTIONS',headers:{Origin:'https://frontend.example'}}),env)).status,204)})
test('reports unavailable backend',async()=>{const original=globalThis.fetch;try{globalThis.fetch=async()=>{throw new Error('offline')};assert.equal((await worker.fetch(new Request('https://frontend.example/api/events'),env)).status,502)}finally{globalThis.fetch=original}})
