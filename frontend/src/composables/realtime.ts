import type { ConversationActivity, Message } from '../types'

interface MessageCreatedEnvelope { type: 'message.created'; sequence: number; payload: Message }
interface MessageUpdatedEnvelope { type: 'message.updated'; sequence: number; payload: Message }
interface ConversationDeletedEnvelope { type: 'conversation.deleted'; payload: { id: string } }
interface ConversationActivityEnvelope { type: 'conversation.activity'; payload: ConversationActivity }
type EventEnvelope = MessageCreatedEnvelope | MessageUpdatedEnvelope | ConversationDeletedEnvelope | ConversationActivityEnvelope
type Socket = Pick<WebSocket, 'onopen' | 'onmessage' | 'onclose' | 'close'>
type SocketConstructor = new (url: string) => Socket

export function useRealtime(
  onMessage: (message: Message) => void,
  Socket: SocketConstructor = WebSocket,
  onConversationDeleted: (conversationID: string) => void = () => {},
  onReconnect: () => void = () => {},
  onActivity: (activity: ConversationActivity) => void = () => {},
  onMessageUpdated: (message: Message, eventSequence: number) => void = () => {},
) {
  let socket: Socket | undefined
  let stopped = false
  let reconnectTimer: ReturnType<typeof setTimeout> | undefined
  let lastSequence = 0
  const seen = new Set<string>()

  const connect = () => {
    stopped = false
    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
    socket = new Socket(`${protocol}//${location.host}/api/ws?after=${lastSequence}`)
    socket.onopen = onReconnect
    socket.onmessage = (raw) => {
      const event = JSON.parse(String(raw.data)) as EventEnvelope
      if (event.type === 'conversation.activity') {
        onActivity(event.payload)
        return
      }
      if (event.type === 'conversation.deleted') {
        onConversationDeleted(event.payload.id)
        return
      }
      if (event.type !== 'message.created' && event.type !== 'message.updated') return
      lastSequence = Math.max(lastSequence, event.sequence)
      if (event.type === 'message.updated') {
        onMessageUpdated(event.payload, event.sequence)
        return
      }
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
