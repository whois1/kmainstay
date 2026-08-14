import assert from 'node:assert/strict'
import test from 'node:test'
import { startBot } from './reference-bot.mjs'

test('receives a human message event and posts one complete reply', async () => {
  const calls = []
  const fetcher = async (url, init) => {
    calls.push({ url, init })
    if (url.endsWith('/api/me')) return Response.json({ id: 'bot_1', kind: 'bot', name: 'Hector' })
    return Response.json({ id: 'reply_1' }, { status: 201 })
  }
  class Socket {
    addEventListener(type, listener) {
      if (type === 'open') queueMicrotask(() => listener({}))
      if (type === 'message') queueMicrotask(() => listener({ data: JSON.stringify({ type: 'message.created', sequence: 3, payload: { id: 'msg_1', conversation_id: 'con_1', author_id: 'human_1', body: 'Hello' } }) }))
    }
    close() {}
  }
  const bot = await startBot({ baseURL: 'http://example.test', apiKey: 'km_live_key', fetcher, WebSocketImpl: Socket, log: () => {} })
  await new Promise((resolve) => setTimeout(resolve, 10))
  assert.equal(calls.length, 2)
  assert.equal(calls[1].url, 'http://example.test/api/conversations/con_1/messages')
  assert.deepEqual(JSON.parse(calls[1].init.body), { body: 'Hector received: Hello', client_id: 'reference-msg_1' })
  bot.stop()
})

test('ignores messages authored by any bot', async () => {
  const calls = []
  const fetcher = async (url, init) => {
    calls.push({ url, init })
    return Response.json({ id: 'bot_1', kind: 'bot', name: 'Hector' })
  }
  class Socket {
    addEventListener(type, listener) {
      if (type === 'message') queueMicrotask(() => listener({ data: JSON.stringify({ type: 'message.created', sequence: 4, payload: { id: 'msg_2', conversation_id: 'con_1', author_id: 'bot_2', author_kind: 'bot', body: 'Hello from another bot' } }) }))
    }
    close() {}
  }
  const bot = await startBot({ baseURL: 'http://example.test', apiKey: 'km_live_key', fetcher, WebSocketImpl: Socket, log: () => {} })
  await new Promise((resolve) => setTimeout(resolve, 10))
  assert.equal(calls.length, 1)
  bot.stop()
})
