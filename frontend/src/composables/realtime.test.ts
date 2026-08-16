import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useRealtime } from './realtime'

class FakeSocket {
  static instances: FakeSocket[] = []
  onopen: (() => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null
  onclose: (() => void) | null = null
  close = vi.fn()
  constructor(public url: string) { FakeSocket.instances.push(this) }
}

describe('realtime connection', () => {
  beforeEach(() => { FakeSocket.instances = []; vi.useFakeTimers() })
  it('deduplicates events, reports conversation deletion and reconciles after reconnect', () => {
    const received: string[] = []
    const deleted = vi.fn()
    const reconnected = vi.fn()
    const realtime = useRealtime((message) => received.push(message.id), FakeSocket as unknown as typeof WebSocket, deleted, reconnected)
    realtime.connect()
    expect(FakeSocket.instances[0].url).toMatch(/after=0$/)
    FakeSocket.instances[0].onopen?.()
    expect(reconnected).toHaveBeenCalledOnce()
    const event = { data: JSON.stringify({ type: 'message.created', sequence: 7, payload: { id: 'msg_1' } }) } as MessageEvent
    FakeSocket.instances[0].onmessage?.(event)
    FakeSocket.instances[0].onmessage?.(event)
    FakeSocket.instances[0].onmessage?.({ data: JSON.stringify({ type: 'conversation.deleted', payload: { id: 'con_1' } }) } as MessageEvent)
    expect(received).toEqual(['msg_1'])
    expect(deleted).toHaveBeenCalledWith('con_1')
    FakeSocket.instances[0].onclose?.()
    vi.advanceTimersByTime(1000)
    expect(FakeSocket.instances[1].url).toMatch(/after=7$/)
    FakeSocket.instances[1].onopen?.()
    expect(reconnected).toHaveBeenCalledTimes(2)
    realtime.disconnect()
  })
})
