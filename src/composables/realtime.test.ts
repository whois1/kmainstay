import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useRealtime } from './realtime'

class FakeSocket {
  static instances: FakeSocket[] = []
  onmessage: ((event: MessageEvent) => void) | null = null
  onclose: (() => void) | null = null
  close = vi.fn()
  constructor(public url: string) { FakeSocket.instances.push(this) }
}

describe('realtime connection', () => {
  beforeEach(() => { FakeSocket.instances = []; vi.useFakeTimers() })
  it('deduplicates events and reconnects after the greatest observed sequence', () => {
    const received: string[] = []
    const realtime = useRealtime((message) => received.push(message.id), FakeSocket as unknown as typeof WebSocket)
    realtime.connect()
    expect(FakeSocket.instances[0].url).toMatch(/after=0$/)
    const event = { data: JSON.stringify({ type: 'message.created', sequence: 7, payload: { id: 'msg_1' } }) } as MessageEvent
    FakeSocket.instances[0].onmessage?.(event)
    FakeSocket.instances[0].onmessage?.(event)
    expect(received).toEqual(['msg_1'])
    FakeSocket.instances[0].onclose?.()
    vi.advanceTimersByTime(1000)
    expect(FakeSocket.instances[1].url).toMatch(/after=7$/)
    realtime.disconnect()
  })
})
