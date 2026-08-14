import WebSocket from 'ws'
import { pathToFileURL } from 'node:url'

export async function startBot({ baseURL, apiKey, fetcher = fetch, WebSocketImpl = WebSocket, log = console.log }) {
  const headers = { Authorization: `Bearer ${apiKey}`, 'Content-Type': 'application/json' }
  const meResponse = await fetcher(`${baseURL}/api/me`, { headers })
  if (!meResponse.ok) throw new Error(`Authentication failed (${meResponse.status})`)
  const me = await meResponse.json()
  let socket
  let stopped = false
  let lastSequence = 0
  let timer
  const seen = new Set()

  const connect = () => {
    const websocketURL = new URL('/api/ws', baseURL)
    websocketURL.protocol = websocketURL.protocol === 'https:' ? 'wss:' : 'ws:'
    websocketURL.searchParams.set('after', String(lastSequence))
    socket = new WebSocketImpl(websocketURL, { headers: { Authorization: `Bearer ${apiKey}` } })
    socket.addEventListener('open', () => log(`Reference bot ${me.name} connected`))
    socket.addEventListener('message', async ({ data }) => {
      const event = JSON.parse(String(data))
      lastSequence = Math.max(lastSequence, event.sequence ?? 0)
      const message = event.payload
      if (event.type !== 'message.created' || !message || message.author_kind === 'bot' || seen.has(message.id)) return
      seen.add(message.id)
      const response = await fetcher(`${baseURL}/api/conversations/${message.conversation_id}/messages`, {
        method: 'POST', headers,
        body: JSON.stringify({ body: `${me.name} received: ${message.body}`, client_id: `reference-${message.id}` }),
      })
      if (!response.ok) log(`Reply failed (${response.status})`)
      else log(`Replied to ${message.id}`)
    })
    socket.addEventListener('close', () => {
      if (!stopped) timer = setTimeout(connect, 1000)
    })
    socket.addEventListener?.('error', (error) => log(`WebSocket error: ${error.message ?? 'connection error'}`))
  }
  connect()
  return { stop() { stopped = true; clearTimeout(timer); socket?.close() } }
}

async function main() {
  const baseURL = process.env.KMAINSTAY_URL
  const apiKey = process.env.KMAINSTAY_API_KEY
  if (!baseURL || !apiKey) throw new Error('KMAINSTAY_URL and KMAINSTAY_API_KEY are required')
  await startBot({ baseURL: baseURL.replace(/\/$/, ''), apiKey })
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((error) => { console.error(error.message); process.exitCode = 1 })
}
