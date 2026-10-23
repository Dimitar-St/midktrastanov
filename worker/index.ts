interface Env { BACKEND_URL:string; FRONTEND_ORIGIN:string; BACKEND_API_TOKEN?:string; ASSETS:{fetch(request:Request):Promise<Response>} }
export default {
 async fetch(request:Request, env:Env):Promise<Response> {
  const url = new URL(request.url)
  if (!url.pathname.startsWith('/api/')) return env.ASSETS.fetch(request)
  const origin = request.headers.get('Origin')
  const allowed = origin === url.origin || origin === env.FRONTEND_ORIGIN
  if (origin && !allowed) return Response.json({error:'Origin is not allowed'}, {status:403})
  const cors:Record<string,string> = origin && allowed ? {'Access-Control-Allow-Origin':origin,'Vary':'Origin'} : {}
   if(request.method === 'OPTIONS') return new Response(null,{status:204,headers:{...cors,'Access-Control-Allow-Methods':'GET, POST, OPTIONS','Access-Control-Allow-Headers':'Content-Type, Authorization'}})
  try {
   const upstream = new URL(env.BACKEND_URL)
   upstream.pathname = upstream.pathname.replace(/\/$/,'') + url.pathname
   upstream.search = url.search
    const headers = new Headers({'Accept':'application/json','Content-Type':'application/json'})
    const authorization=request.headers.get('Authorization')
    if(authorization) headers.set('Authorization',authorization)
    if(env.BACKEND_API_TOKEN) headers.set('X-Gateway-Token',env.BACKEND_API_TOKEN)
   const response = await fetch(upstream, {method:request.method,headers,body:['GET','HEAD'].includes(request.method)?undefined:await request.arrayBuffer(),redirect:'manual',signal:AbortSignal.timeout(15000)})
   return new Response(response.body,{status:response.status,headers:{...cors,'Content-Type':response.headers.get('Content-Type') || 'application/json','Cache-Control':'no-store'}})
  } catch { return Response.json({error:'Връзката с бекенда е недостъпна. Опитайте отново.'},{status:502,headers:cors}) }
 }
}
