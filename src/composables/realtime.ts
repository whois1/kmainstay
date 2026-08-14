import type { Message } from '../types'

interface EventEnvelope { type: string; sequence: number; payload: Message }
type SocketConstructor = new (url: string) => Pick<WebSocket, 'onmessage' | 'onclose' | 'close'>

export function useRealtime(onMessage: (message: Message) => void, Socket: SocketConstructor = WebSocket) {
  let socket: Pick<WebSocket, 'onmessage' | 'onclose' | 'close'> | undefined
  let stopped = false
  let reconnectTimer: ReturnType<typeof setTimeout> | undefined
  let lastSequence = 0
  const seen = new Set<string>()

  const connect = () => {
    stopped = false
    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
    socket = new Socket(`${protocol}//${location.host}/api/ws?after=${lastSequence}`)
    socket.onmessage = (raw) => {
      const event = JSON.parse(String(raw.data)) as EventEnvelope
      if (event.type !== 'message.created') return
      lastSequence = Math.max(lastSequence, event.sequence)
      if (seen.has(event.payload.id)) return
      seen.add(event.payload.id)
      onMessage(event.payload)
    }
    socket.onclose = () => {
      if (!stopped) reconnectTimer = setTimeout(connect, 1000)
    }
  }
  const disconnect = () => {
    stopped = true
    if (reconnectTimer) clearTimeout(reconnectTimer)
    socket?.close()
  }
  const seed = (messages: Message[]) => {
    for (const message of messages) {
      seen.add(message.id)
      lastSequence = Math.max(lastSequence, message.sequence)
    }
  }
  return { connect, disconnect, seed }
}
