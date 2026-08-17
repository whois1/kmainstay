import { readFileSync } from 'node:fs'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import App from './App.vue'

const styles = readFileSync('frontend/src/style.css', 'utf8')

const json = (body: unknown, status = 200) => Promise.resolve(new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } }))

describe('K-Mainstay UI', () => {
  it('declares one dark colour scheme without exposing a theme control', async () => {
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(loadedFetcher()), socketFactory: class { close() {} } } } })
    await flushPromises()

    expect(document.documentElement.dataset.theme).toBe('dark')
    expect(document.documentElement.style.colorScheme).toBe('dark')
    expect(wrapper.find('[data-testid=theme-toggle]').exists()).toBe(false)
  })

  it('keeps navigation and conversation content in independent viewport scroll regions', () => {
    expect(cssRule('.workspace')).toContain('height: 100dvh')
    expect(cssRule('.workspace')).toContain('overflow: hidden')
    expect(cssRule('aside')).toContain('min-height: 0')
    expect(cssRule('aside')).toContain('overflow: hidden')
    expect(cssRule('.sidebar-navigation')).toContain('min-height: 0')
    expect(cssRule('.sidebar-navigation')).toContain('overflow-y: auto')
    expect(cssRule('.conversation')).toContain('min-height: 0')
    expect(cssRule('.conversation')).toContain('overflow: hidden')
    expect(cssRule('.message-list')).toContain('overflow-y: auto')
  })

  it('names the persistent navigation and active product surface', async () => {
    const fetcher = loadedFetcher()
    fetcher.mockImplementationOnce(() => json([{ id: 'u1', name: 'Michael', kind: 'human', role: 'admin' }]))
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: class { close() {} } } } })
    await flushPromises()

    expect(wrapper.get('aside[aria-label="Workspace navigation"]')).toBeTruthy()
    expect(wrapper.get('section[aria-label="Conversation"]')).toBeTruthy()

    await wrapper.get('[data-testid=organisation-settings]').trigger('click')
    await flushPromises()

    expect(wrapper.get('section[data-testid=settings-page][aria-label="Organisation settings"]')).toBeTruthy()
  })

  it('gives the compact delete control a complete accessible name', async () => {
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(loadedFetcher()), socketFactory: class { close() {} } } } })
    await flushPromises()

    const deleteButton = wrapper.get('[data-testid=delete-conversation]')
    expect(deleteButton.attributes('aria-label')).toBe('Delete conversation')
    expect(deleteButton.get('[aria-hidden=true]').text()).toBe('Delete')
  })

  it('contains modal focus, closes with Escape and restores the opener', async () => {
    const fetcher = loadedFetcher()
    fetcher.mockImplementationOnce(() => json([{ id: 'u1', name: 'Michael', kind: 'human' }, { id: 'b1', name: 'Hector', kind: 'bot' }]))
    const wrapper = mount(App, { attachTo: document.body, global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: class { close() {} } } } })
    await flushPromises()

    const opener = wrapper.get('[data-testid=new-conversation]')
    ;(opener.element as HTMLElement).focus()
    await opener.trigger('click')
    await flushPromises()

    const dialog = wrapper.get('[role=dialog]')
    const input = wrapper.get('[data-testid=conversation-name]')
    expect(document.activeElement).toBe(input.element)

    const submit = wrapper.get('[data-testid=create-conversation] button:not(.close)')
    ;(submit.element as HTMLElement).focus()
    await dialog.trigger('keydown', { key: 'Tab' })
    expect(document.activeElement).toBe(wrapper.get('[data-testid=create-conversation] .close').element)

    await dialog.trigger('keydown', { key: 'Escape' })
    await flushPromises()
    expect(wrapper.find('[data-testid=create-conversation]').exists()).toBe(false)
    expect(document.activeElement).toBe(opener.element)
    wrapper.unmount()
  })

  it('logs in and opens the single organisation conversation', async () => {
    const fetcher = vi.fn()
      .mockImplementationOnce(() => json({}, 401))
      .mockImplementationOnce(() => json({ id: 'u1', name: 'Michael', kind: 'human' }))
      .mockImplementationOnce(() => json([{ id: 'o1', name: 'Mainstay', role: 'admin' }]))
      .mockImplementationOnce(() => json([{ id: 'c1', name: 'general', visibility: 'organisation' }]))
      .mockImplementationOnce(() => json([]))
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: class { close() {} } } } })
    await flushPromises()
    expect(wrapper.get('h1').text()).toBe('Welcome back')
    await wrapper.get('input[type=email]').setValue('michael@example.com')
    await wrapper.get('input[type=password]').setValue('secret')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(fetcher).toHaveBeenCalledWith('/api/session', expect.objectContaining({ method: 'POST' }))
    expect(wrapper.text()).toContain('Mainstay')
    expect(wrapper.text()).toContain('general')
  })

  it('shows chronological complete messages and posts from the composer', async () => {
    const fetcher = vi.fn()
      .mockImplementationOnce(() => json({ id: 'u1', name: 'Michael', kind: 'human' }))
      .mockImplementationOnce(() => json([{ id: 'o1', name: 'Mainstay', role: 'admin' }]))
      .mockImplementationOnce(() => json([{ id: 'c1', name: 'general', visibility: 'organisation' }]))
      .mockImplementationOnce(() => json([
        { id: 'm1', author_name: 'Michael', author_kind: 'human', body: 'Hello **bot**', created_at: '2026-01-01T00:00:00Z', sequence: 1 },
        { id: 'm2', author_name: 'Hector', author_kind: 'bot', body: 'Hello human', created_at: '2026-01-01T00:00:01Z', sequence: 2 },
      ]))
	  .mockImplementationOnce(() => json({ sequence: 2 }))
      .mockImplementationOnce(() => json({ id: 'm3', author_name: 'Michael', author_kind: 'human', body: 'Next', created_at: '2026-01-01T00:00:02Z', sequence: 3 }, 201))
	  .mockImplementationOnce(() => json({ sequence: 3 }))
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: class { close() {} } } } })
    await flushPromises()
    const messages = wrapper.findAll('[data-testid=message]')
    expect(messages[0].text()).toContain('Michael')
    expect(messages[0].text()).toContain('Hello bot')
    expect(messages[1].text()).toContain('HectorBOT')
    expect(messages[1].text()).toContain('Hello human')
    expect(wrapper.find('[data-testid=message] img').exists()).toBe(false)
    await wrapper.get('textarea').setValue('Next')
    await wrapper.get('[data-testid=composer]').trigger('submit')
    await flushPromises()
    expect(fetcher).toHaveBeenCalledWith('/api/conversations/c1/messages', expect.objectContaining({ method: 'POST' }))
    expect(wrapper.text()).toContain('Next')
  })

  it('marks through the latest loaded message after initial positioning', async () => {
	const fetcher = vi.fn()
	  .mockImplementationOnce(() => json({ id: 'u1', name: 'Michael', kind: 'human' }))
	  .mockImplementationOnce(() => json([{ id: 'o1', name: 'Mainstay', role: 'admin' }]))
	  .mockImplementationOnce(() => json([{ id: 'c1', name: 'general', visibility: 'organisation', read_sequence: 1 }]))
	  .mockImplementationOnce(() => json(messagesForReadTest()))
	  .mockImplementationOnce(() => json({ sequence: 2 }))
	mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: class { close() {} } } } })
	await flushPromises()

	expect(fetcher).toHaveBeenCalledWith('/api/conversations/c1/read', expect.objectContaining({ method: 'PUT', body: JSON.stringify({ sequence: 2 }) }))
  })

  it('keeps recent history available before the first unread message', async () => {
    const recentHistory = messagesForReadTest()
    const unreadMessage = { ...recentHistory[1], id: 'm3', body: 'New', sequence: 3 }
    const fetcher = vi.fn((url: string) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations') return json([{ id: 'c1', name: 'general', visibility: 'organisation', read_sequence: 2, latest_sequence: 3 }])
      if (url === '/api/conversations/c1/messages?limit=100&after_sequence=2') return json([unreadMessage])
      if (url === '/api/conversations/c1/messages?limit=100&before=m3') return json(recentHistory)
      if (url === '/api/conversations/c1/read') return json({ sequence: 3 })
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: class { close() {} } } } })
    await flushPromises()

    expect(fetcher).toHaveBeenCalledWith('/api/conversations/c1/messages?limit=100&before=m3', undefined)
    expect(wrapper.findAll('[data-testid=message]').map(message => message.attributes('data-message-id'))).toEqual(['m1', 'm2', 'm3'])
    expect(wrapper.get('[data-testid=new-messages-divider]').element.nextElementSibling?.getAttribute('data-message-id')).toBe('m3')
  })

  it('keeps the unread page when recent history cannot be loaded', async () => {
    const unreadMessage = { ...messagesForReadTest()[1], id: 'm3', body: 'New', sequence: 3 }
    const fetcher = vi.fn((url: string) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations') return json([{ id: 'c1', name: 'general', visibility: 'organisation', read_sequence: 2, latest_sequence: 3 }])
      if (url === '/api/conversations/c1/messages?limit=100&after_sequence=2') return json([unreadMessage])
      if (url === '/api/conversations/c1/messages?limit=100&before=m3') return json({ error: 'History unavailable' }, 500)
      if (url === '/api/conversations/c1/read') return json({ sequence: 3 })
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: class { close() {} } } } })
    await flushPromises()

    expect(wrapper.findAll('[data-testid=message]').map(message => message.attributes('data-message-id'))).toEqual(['m3'])
  })

  it('does not request history after conversation selection becomes stale', async () => {
    let finishUnreadMessages!: (response: Response) => void
    const pendingUnreadMessages = new Promise<Response>(resolve => { finishUnreadMessages = resolve })
    const unreadMessage = messagesForReadTest()[1]
    const fetcher = vi.fn((url: string) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations') return json([
        { id: 'c1', name: 'general', visibility: 'organisation', read_sequence: 1, latest_sequence: 2 },
        { id: 'c2', name: 'planning', visibility: 'organisation', read_sequence: 0, latest_sequence: 0 },
      ])
      if (url === '/api/conversations/c1/messages?limit=100&after_sequence=1') return pendingUnreadMessages
      if (url === '/api/conversations/c1/messages?limit=100&before=m2') return json([])
      if (url === '/api/conversations/c2/messages?limit=100') return json([])
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: class { close() {} } } } })
    await flushPromises()

    await conversationButton(wrapper, 'planning').trigger('click')
    await flushPromises()
    finishUnreadMessages(await json([unreadMessage]))
    await flushPromises()

    expect(fetcher).not.toHaveBeenCalledWith('/api/conversations/c1/messages?limit=100&before=m2', undefined)
    expect(wrapper.get('h1').text()).toBe('# planning')
  })

  it('loads a bounded page at the true unread boundary, then jumps to latest', async () => {
    const unreadPage = Array.from({ length: 100 }, (_, index) => ({
      id: `m${index + 2}`, conversation_id: 'c1', author_id: 'b1', author_name: 'Hector', author_kind: 'bot', body: `Message ${index + 2}`, created_at: '2026-01-01T00:00:00Z', sequence: index + 2,
    }))
    const latestPage = Array.from({ length: 100 }, (_, index) => ({ ...unreadPage[index], id: `m${index + 102}`, body: `Message ${index + 102}`, sequence: index + 102 }))
    const fetcher = vi.fn()
      .mockImplementationOnce(() => json({ id: 'u1', name: 'Michael', kind: 'human' }))
      .mockImplementationOnce(() => json([{ id: 'o1', name: 'Mainstay', role: 'admin' }]))
      .mockImplementationOnce(() => json([{ id: 'c1', name: 'general', visibility: 'organisation', read_sequence: 1, latest_sequence: 201 }]))
      .mockImplementationOnce(() => json(unreadPage))
      .mockImplementationOnce(() => json([]))
      .mockImplementationOnce(() => json({ sequence: 101 }))
      .mockImplementationOnce(() => json(latestPage))
      .mockImplementationOnce(() => json({ sequence: 201 }))
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: class { close() {} } } } })
    await flushPromises()

    expect(fetcher).toHaveBeenCalledWith('/api/conversations/c1/messages?limit=100&after_sequence=1', undefined)
    expect(wrapper.findAll('[data-testid=message]')).toHaveLength(100)
    expect(wrapper.get('[data-testid=new-messages-divider]').element.nextElementSibling?.getAttribute('data-message-id')).toBe('m2')
    expect(wrapper.get('[data-testid=jump-to-bottom]').attributes('aria-label')).toBe('Jump to latest message')

    await wrapper.get('[data-testid=jump-to-bottom]').trigger('click')
    await flushPromises()

    expect(fetcher).toHaveBeenCalledWith('/api/conversations/c1/messages?limit=100', undefined)
    const renderedMessages = wrapper.findAll('[data-testid=message]')
    expect(renderedMessages[renderedMessages.length - 1]?.attributes('data-message-id')).toBe('m201')
    expect(wrapper.find('[data-testid=new-messages-divider]').exists()).toBe(false)
    expect(wrapper.find('[data-testid=jump-to-bottom]').exists()).toBe(false)
    expect(fetcher).toHaveBeenCalledWith('/api/conversations/c1/read', expect.objectContaining({ method: 'PUT', body: JSON.stringify({ sequence: 201 }) }))
  })

  it('keeps a bounded unread gap when history and realtime messages arrive during selection', async () => {
    class EventSocket {
      static instance: EventSocket
      onopen: (() => void) | null = null
      onmessage: ((event: MessageEvent) => void) | null = null
      onclose: (() => void) | null = null
      close() {}
      constructor() { EventSocket.instance = this }
    }
    const unreadPage = Array.from({ length: 100 }, (_, index) => ({
      id: `m${index + 2}`, conversation_id: 'c2', author_id: 'b1', author_name: 'Hector', author_kind: 'bot', body: `Message ${index + 2}`, created_at: '2026-01-01T00:00:00Z', sequence: index + 2,
    }))
    const history = [{ ...unreadPage[0], id: 'm1', body: 'History', sequence: 1 }]
    const realtimeMessage = { ...unreadPage[0], id: 'm202', body: 'Realtime', sequence: 202 }
    let finishHistory!: (response: Response) => void
    const pendingHistory = new Promise<Response>(resolve => { finishHistory = resolve })
    const fetcher = vi.fn((url: string) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations') return json([
        { id: 'c1', name: 'general', visibility: 'organisation', read_sequence: 0, latest_sequence: 0 },
        { id: 'c2', name: 'planning', visibility: 'organisation', read_sequence: 1, latest_sequence: 201 },
      ])
      if (url === '/api/conversations/c1/messages?limit=100') return json([])
      if (url === '/api/conversations/c2/messages?limit=100&after_sequence=1') return json(unreadPage)
      if (url === '/api/conversations/c2/messages?limit=100&before=m2') return pendingHistory
      if (url === '/api/conversations/c2/read') return json({ sequence: 101 })
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: EventSocket } } })
    await flushPromises()

    const selection = conversationButton(wrapper, 'planning').trigger('click')
    await Promise.resolve()
    await Promise.resolve()
    EventSocket.instance.onmessage?.({ data: JSON.stringify({ type: 'message.created', sequence: 202, payload: realtimeMessage }) } as MessageEvent)
    finishHistory(await json(history))
    await selection
    await flushPromises()

    const renderedMessages = wrapper.findAll('[data-testid=message]')
    expect(renderedMessages[renderedMessages.length - 1]?.attributes('data-message-id')).toBe('m101')
    expect(wrapper.get('[data-testid=jump-to-bottom]').attributes('aria-label')).toBe('Jump to latest message')
  })

  it('merges realtime arrivals that occur during conversation selection', async () => {
    class EventSocket {
      static instance: EventSocket
      onopen: (() => void) | null = null
      onmessage: ((event: MessageEvent) => void) | null = null
      onclose: (() => void) | null = null
      close() {}
      constructor() { EventSocket.instance = this }
    }
    let finishPlanningMessages!: (response: Response) => void
    const planningMessages = new Promise<Response>(resolve => { finishPlanningMessages = resolve })
    const fetchedMessage = { id: 'm2', conversation_id: 'c2', author_id: 'b1', author_name: 'Hector', author_kind: 'bot', body: 'Fetched', created_at: '2026-01-01T00:00:01Z', sequence: 2 }
    const realtimeMessage = { ...fetchedMessage, id: 'm3', body: 'Realtime', created_at: '2026-01-01T00:00:02Z', sequence: 3 }
    const fetcher = vi.fn((url: string) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations') return json([
        { id: 'c1', name: 'general', visibility: 'organisation', read_sequence: 0, latest_sequence: 0 },
        { id: 'c2', name: 'planning', visibility: 'organisation', read_sequence: 1, latest_sequence: 2 },
      ])
      if (url === '/api/conversations/c1/messages?limit=100') return json([])
      if (url === '/api/conversations/c2/messages?limit=100&after_sequence=1') return planningMessages
      if (url === '/api/conversations/c2/messages?limit=100&before=m2') return json([])
      if (url === '/api/conversations/c2/read') return json({ sequence: 3 })
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: EventSocket } } })
    await flushPromises()

    await conversationButton(wrapper, 'planning').trigger('click')
    EventSocket.instance.onmessage?.({ data: JSON.stringify({ type: 'message.created', sequence: 3, payload: realtimeMessage }) } as MessageEvent)
    finishPlanningMessages(await json([fetchedMessage]))
    await flushPromises()

    expect(wrapper.findAll('[data-testid=message]').map(message => message.attributes('data-message-id'))).toEqual(['m2', 'm3'])
    expect(wrapper.text()).toContain('Realtime')
  })

  it('merges realtime arrivals that occur while jumping to latest', async () => {
    class EventSocket {
      static instance: EventSocket
      onopen: (() => void) | null = null
      onmessage: ((event: MessageEvent) => void) | null = null
      onclose: (() => void) | null = null
      close() {}
      constructor() { EventSocket.instance = this }
    }
    const unreadPage = Array.from({ length: 100 }, (_, index) => ({
      id: `m${index + 2}`, conversation_id: 'c1', author_id: 'b1', author_name: 'Hector', author_kind: 'bot', body: `Message ${index + 2}`, created_at: '2026-01-01T00:00:00Z', sequence: index + 2,
    }))
    const latestPage = Array.from({ length: 100 }, (_, index) => ({ ...unreadPage[index], id: `m${index + 102}`, body: `Message ${index + 102}`, sequence: index + 102 }))
    const realtimeMessage = { ...latestPage[99], id: 'm202', body: 'Realtime 202', sequence: 202 }
    let finishLatestMessages!: (response: Response) => void
    const latestMessages = new Promise<Response>(resolve => { finishLatestMessages = resolve })
    const fetcher = vi.fn((url: string) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations') return json([{ id: 'c1', name: 'general', visibility: 'organisation', read_sequence: 1, latest_sequence: 201 }])
      if (url === '/api/conversations/c1/messages?limit=100&after_sequence=1') return json(unreadPage)
      if (url === '/api/conversations/c1/messages?limit=100&before=m2') return json([])
      if (url === '/api/conversations/c1/messages?limit=100') return latestMessages
      if (url === '/api/conversations/c1/read') return json({ sequence: 202 })
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: EventSocket } } })
    await flushPromises()

    await wrapper.get('[data-testid=jump-to-bottom]').trigger('click')
    EventSocket.instance.onmessage?.({ data: JSON.stringify({ type: 'message.created', sequence: 202, payload: realtimeMessage }) } as MessageEvent)
    finishLatestMessages(await json(latestPage))
    await flushPromises()

    const renderedMessages = wrapper.findAll('[data-testid=message]')
    expect(renderedMessages[renderedMessages.length - 1]?.attributes('data-message-id')).toBe('m202')
    expect(wrapper.text()).toContain('Realtime 202')
    expect(wrapper.find('[data-testid=jump-to-bottom]').exists()).toBe(false)
  })

  it('creates a bot and presents its API key once with copy warning', async () => {
    const fetcher = loadedFetcher()
    fetcher
      .mockImplementationOnce(() => json([{ id: 'u1', name: 'Michael', kind: 'human', role: 'admin' }]))
      .mockImplementationOnce(() => json({ id: 'b1', name: 'Hector', kind: 'bot', role: 'member', api_key: 'km_live_lookup_secret' }, 201))
    Object.assign(navigator, { clipboard: { writeText: vi.fn().mockRejectedValue(new Error('denied')) } })
    const wrapper = mount(App, { attachTo: document.body, global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: class { close() {} } } } })
    await flushPromises()
    await wrapper.get('[data-testid=organisation-settings]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid=add-bot]').trigger('click')
    await wrapper.get('[data-testid=bot-name]').setValue('Hector')
    await wrapper.get('[data-testid=create-bot]').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('Copy this key now')
    expect(wrapper.text()).toContain('km_live_lookup_secret')
    expect(document.activeElement).toBe(wrapper.get('[data-testid=copy-key]').element)
    await wrapper.get('[data-testid=copy-key]').trigger('click')
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith('km_live_lookup_secret')
    await flushPromises()
    expect(wrapper.text()).toContain('Select the visible key and copy it manually')
    expect(wrapper.get('.modal [role=alert]').text()).toContain('copy it manually')
    expect(wrapper.get('.modal .close').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('creates a private conversation with selected organisation users', async () => {
    const fetcher = loadedFetcher()
    fetcher.mockImplementationOnce(() => json([{ id: 'u1', name: 'Michael', kind: 'human' }, { id: 'b1', name: 'Hector', kind: 'bot' }]))
    fetcher.mockImplementationOnce(() => json({ id: 'c2', name: 'Planning', visibility: 'members' }, 201))
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: class { close() {} } } } })
    await flushPromises()
    await wrapper.get('[data-testid=new-conversation]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid=conversation-name]').setValue('Planning')
    await wrapper.get('input[value=b1]').setValue(true)
    await wrapper.get('[data-testid=create-conversation]').trigger('submit')
    await flushPromises()
    expect(fetcher).toHaveBeenCalledWith('/api/organisations/o1/conversations', expect.objectContaining({ body: JSON.stringify({ name: 'Planning', visibility: 'members', member_ids: ['b1'] }) }))
    expect(wrapper.text()).toContain('Planning')
  })

  it('lets an admin delete the selected conversation for everyone', async () => {
    const fetcher = vi.fn()
      .mockImplementationOnce(() => json({ id: 'u1', name: 'Michael', kind: 'human' }))
      .mockImplementationOnce(() => json([{ id: 'o1', name: 'Mainstay', role: 'admin' }]))
      .mockImplementationOnce(() => json([
        { id: 'c1', name: 'general', visibility: 'organisation' },
        { id: 'c2', name: 'planning', visibility: 'organisation' },
      ]))
      .mockImplementationOnce(() => json([]))
      .mockImplementationOnce(() => Promise.resolve(new Response(null, { status: 204 })))
      .mockImplementationOnce(() => json([]))
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true)
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: class { close() {} } } } })
    await flushPromises()

    await wrapper.get('[data-testid=delete-conversation]').trigger('click')
    await flushPromises()

    expect(confirm).toHaveBeenCalledWith('Delete #general? This removes its messages and nobody will be able to send to it.')
    expect(fetcher).toHaveBeenCalledWith('/api/organisations/o1/conversations/c1', expect.objectContaining({ method: 'DELETE' }))
    expect(wrapper.findAll('nav button').some(button => button.text().includes('general'))).toBe(false)
    expect(wrapper.get('h1').text()).toBe('# planning')
    confirm.mockRestore()
  })

  it('shows a clear empty state after deleting the last conversation', async () => {
    const fetcher = loadedFetcher()
    fetcher.mockImplementationOnce(() => Promise.resolve(new Response(null, { status: 204 })))
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true)
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: class { close() {} } } } })
    await flushPromises()

    await wrapper.get('[data-testid=delete-conversation]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('No conversations')
    expect(wrapper.text()).not.toContain('# Choose a conversation')
    expect(wrapper.find('[data-testid=composer]').exists()).toBe(false)
    expect(wrapper.findAll('.sidebar-navigation section > button')).toHaveLength(0)
    confirm.mockRestore()
  })

  it('does not preserve a failed read cursor across reconnect refreshes', async () => {
    class EventSocket {
      static instance: EventSocket
      onopen: (() => void) | null = null
      onmessage: ((event: MessageEvent) => void) | null = null
      onclose: (() => void) | null = null
      close() {}
      constructor() { EventSocket.instance = this }
    }
    const message = messagesForReadTest()[1]
    const fetcher = vi.fn()
      .mockImplementationOnce(() => json({ id: 'u1', name: 'Michael', kind: 'human' }))
      .mockImplementationOnce(() => json([{ id: 'o1', name: 'Mainstay', role: 'admin' }]))
      .mockImplementationOnce(() => json([{ id: 'c1', name: 'general', visibility: 'organisation', read_sequence: 1, latest_sequence: 2 }]))
      .mockImplementationOnce(() => json([message]))
      .mockImplementationOnce(() => json([]))
      .mockImplementationOnce(() => json({ error: 'Read sync failed' }, 500))
      .mockImplementationOnce(() => json([{ id: 'c1', name: 'general', visibility: 'organisation', read_sequence: 1, latest_sequence: 2 }]))
      .mockImplementationOnce(() => json([message]))
      .mockImplementationOnce(() => json([]))
      .mockImplementationOnce(() => json({ sequence: 2 }))
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: EventSocket } } })
    await flushPromises()

    EventSocket.instance.onopen?.()
    await flushPromises()
    await wrapper.get('nav button').trigger('click')
    await flushPromises()

    expect(fetcher).toHaveBeenCalledWith('/api/conversations/c1/messages?limit=100&after_sequence=1', undefined)
  })

  it('keeps the selected read cursor canonical across reconnect refreshes', async () => {
    class EventSocket {
      static instance: EventSocket
      onopen: (() => void) | null = null
      onmessage: ((event: MessageEvent) => void) | null = null
      onclose: (() => void) | null = null
      close() {}
      constructor() { EventSocket.instance = this }
    }
    const message = messagesForReadTest()[1]
    const fetcher = vi.fn()
      .mockImplementationOnce(() => json({ id: 'u1', name: 'Michael', kind: 'human' }))
      .mockImplementationOnce(() => json([{ id: 'o1', name: 'Mainstay', role: 'admin' }]))
      .mockImplementationOnce(() => json([{ id: 'c1', name: 'general', visibility: 'organisation', read_sequence: 1, latest_sequence: 2 }]))
      .mockImplementationOnce(() => json([message]))
      .mockImplementationOnce(() => json([]))
      .mockImplementationOnce(() => json({ sequence: 2 }))
      .mockImplementationOnce(() => json([{ id: 'c1', name: 'general', visibility: 'organisation', read_sequence: 1, latest_sequence: 2 }]))
      .mockImplementationOnce(() => json([message]))
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: EventSocket } } })
    await flushPromises()

    EventSocket.instance.onopen?.()
    await flushPromises()
    await wrapper.get('nav button').trigger('click')
    await flushPromises()

    expect(fetcher).toHaveBeenLastCalledWith('/api/conversations/c1/messages?limit=100', undefined)
    expect(fetcher).not.toHaveBeenLastCalledWith('/api/conversations/c1/messages?limit=100&after_sequence=1', undefined)
  })

  it('keeps a deleted conversation out when an older reconnect refresh finishes later', async () => {
    class EventSocket {
      static instance: EventSocket
      onopen: (() => void) | null = null
      onmessage: ((event: MessageEvent) => void) | null = null
      onclose: (() => void) | null = null
      close() {}
      constructor() { EventSocket.instance = this }
    }
    let finishStaleRefresh!: (response: Response) => void
    let finishDeletionRefresh!: (response: Response) => void
    const staleRefresh = new Promise<Response>(resolve => { finishStaleRefresh = resolve })
    const deletionRefresh = new Promise<Response>(resolve => { finishDeletionRefresh = resolve })
    const fetcher = vi.fn()
      .mockImplementationOnce(() => json({ id: 'u1', name: 'Michael', kind: 'human' }))
      .mockImplementationOnce(() => json([{ id: 'o1', name: 'Mainstay', role: 'admin' }]))
      .mockImplementationOnce(() => json([
        { id: 'c1', name: 'general', visibility: 'organisation' },
        { id: 'c2', name: 'planning', visibility: 'organisation' },
      ]))
      .mockImplementationOnce(() => json([]))
      .mockImplementationOnce(() => staleRefresh)
      .mockImplementationOnce(() => json([]))
      .mockImplementationOnce(() => deletionRefresh)
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: EventSocket } } })
    await flushPromises()

    EventSocket.instance.onopen?.()
    EventSocket.instance.onmessage?.({ data: JSON.stringify({ type: 'conversation.deleted', payload: { id: 'c1' } }) } as MessageEvent)
    finishDeletionRefresh(await json([{ id: 'c2', name: 'planning', visibility: 'organisation' }]))
    await flushPromises()
    finishStaleRefresh(await json([
      { id: 'c1', name: 'general', visibility: 'organisation' },
      { id: 'c2', name: 'planning', visibility: 'organisation' },
    ]))
    await flushPromises()

    expect(wrapper.findAll('nav button').some(button => button.text().includes('general'))).toBe(false)
    expect(wrapper.get('h1').text()).toBe('# planning')
    expect(fetcher).toHaveBeenCalledWith('/api/organisations/o1/conversations', undefined)
    expect(fetcher).toHaveBeenCalledWith('/api/conversations/c2/messages?limit=100', undefined)
  })

  it('removes a remotely deleted conversation even when list refresh fails', async () => {
    class EventSocket {
      static instance: EventSocket
      onopen: (() => void) | null = null
      onmessage: ((event: MessageEvent) => void) | null = null
      onclose: (() => void) | null = null
      close() {}
      constructor() { EventSocket.instance = this }
    }
    const fetcher = vi.fn()
      .mockImplementationOnce(() => json({ id: 'u1', name: 'Michael', kind: 'human' }))
      .mockImplementationOnce(() => json([{ id: 'o1', name: 'Mainstay', role: 'admin' }]))
      .mockImplementationOnce(() => json([
        { id: 'c1', name: 'general', visibility: 'organisation' },
        { id: 'c2', name: 'planning', visibility: 'organisation' },
      ]))
      .mockImplementationOnce(() => json([]))
      .mockImplementationOnce(() => json([]))
      .mockImplementationOnce(() => json({ error: 'List unavailable' }, 500))
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: EventSocket } } })
    await flushPromises()

    EventSocket.instance.onmessage?.({ data: JSON.stringify({ type: 'conversation.deleted', payload: { id: 'c1' } }) } as MessageEvent)
    await flushPromises()

    expect(wrapper.findAll('nav button').some(button => button.text().includes('general'))).toBe(false)
    expect(wrapper.get('h1').text()).toBe('# planning')
    expect(wrapper.text()).toContain('List unavailable')
  })

  it('does not show stale messages when coalesced deletions change the fallback conversation', async () => {
    class EventSocket {
      static instance: EventSocket
      onopen: (() => void) | null = null
      onmessage: ((event: MessageEvent) => void) | null = null
      onclose: (() => void) | null = null
      close() {}
      constructor() { EventSocket.instance = this }
    }
    let finishPlanningMessages!: (response: Response) => void
    const planningMessages = new Promise<Response>(resolve => { finishPlanningMessages = resolve })
    const fetcher = vi.fn()
      .mockImplementationOnce(() => json({ id: 'u1', name: 'Michael', kind: 'human' }))
      .mockImplementationOnce(() => json([{ id: 'o1', name: 'Mainstay', role: 'admin' }]))
      .mockImplementationOnce(() => json([
        { id: 'c1', name: 'general', visibility: 'organisation' },
        { id: 'c2', name: 'planning', visibility: 'organisation' },
        { id: 'c3', name: 'delivery', visibility: 'organisation' },
      ]))
      .mockImplementationOnce(() => json([]))
      .mockImplementationOnce(() => planningMessages)
      .mockImplementationOnce(() => json([{ id: 'm3', conversation_id: 'c3', author_name: 'Michael', author_kind: 'human', body: 'Delivery message', created_at: '2026-01-01T00:00:03Z', sequence: 3 }]))
      .mockImplementationOnce(() => json([{ id: 'c3', name: 'delivery', visibility: 'organisation' }]))
      .mockImplementationOnce(() => json([{ id: 'c3', name: 'delivery', visibility: 'organisation' }]))
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: EventSocket } } })
    await flushPromises()

    EventSocket.instance.onmessage?.({ data: JSON.stringify({ type: 'conversation.deleted', payload: { id: 'c1' } }) } as MessageEvent)
    EventSocket.instance.onmessage?.({ data: JSON.stringify({ type: 'conversation.deleted', payload: { id: 'c2' } }) } as MessageEvent)
    await flushPromises()
    finishPlanningMessages(await json([{ id: 'm2', conversation_id: 'c2', author_name: 'Michael', author_kind: 'human', body: 'Planning message', created_at: '2026-01-01T00:00:02Z', sequence: 2 }]))
    await flushPromises()

    expect(wrapper.get('h1').text()).toBe('# delivery')
    expect(wrapper.text()).toContain('Delivery message')
    expect(wrapper.text()).not.toContain('Planning message')
  })

  it('discards a late send response after that conversation is deleted', async () => {
    class EventSocket {
      static instance: EventSocket
      onopen: (() => void) | null = null
      onmessage: ((event: MessageEvent) => void) | null = null
      onclose: (() => void) | null = null
      close() {}
      constructor() { EventSocket.instance = this }
    }
    let finishSend!: (response: Response) => void
    const sendResponse = new Promise<Response>(resolve => { finishSend = resolve })
    const fetcher = vi.fn()
      .mockImplementationOnce(() => json({ id: 'u1', name: 'Michael', kind: 'human' }))
      .mockImplementationOnce(() => json([{ id: 'o1', name: 'Mainstay', role: 'admin' }]))
      .mockImplementationOnce(() => json([
        { id: 'c1', name: 'general', visibility: 'organisation' },
        { id: 'c2', name: 'planning', visibility: 'organisation' },
      ]))
      .mockImplementationOnce(() => json([]))
      .mockImplementationOnce(() => sendResponse)
      .mockImplementationOnce(() => json([]))
      .mockImplementationOnce(() => json([{ id: 'c2', name: 'planning', visibility: 'organisation' }]))
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: EventSocket } } })
    await flushPromises()

    await wrapper.get('textarea').setValue('Late general message')
    void wrapper.get('[data-testid=composer]').trigger('submit')
    await flushPromises()
    EventSocket.instance.onmessage?.({ data: JSON.stringify({ type: 'conversation.deleted', payload: { id: 'c1' } }) } as MessageEvent)
    await flushPromises()
    finishSend(await json({ id: 'm1', conversation_id: 'c1', author_name: 'Michael', author_kind: 'human', body: 'Late general message', created_at: '2026-01-01T00:00:01Z', sequence: 1 }, 201))
    await flushPromises()

    expect(wrapper.get('h1').text()).toBe('# planning')
    expect(wrapper.text()).not.toContain('Late general message')
  })

  it('opens a full settings page with separate people and bot sections', async () => {
    const fetcher = loadedFetcher()
    fetcher.mockImplementationOnce(() => json([
      { id: 'u1', name: 'Michael', kind: 'human', role: 'admin' },
      { id: 'b1', name: 'Hector', kind: 'bot', role: 'member' },
    ]))
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: class { close() {} } } } })
    await flushPromises()
    expect(wrapper.get('[data-testid=organisation-brand]').element.tagName).toBe('DIV')
    await wrapper.get('[data-testid=organisation-settings]').trigger('click')
    await flushPromises()
    expect(fetcher).toHaveBeenCalledWith('/api/organisations/o1/users', undefined)
    expect(wrapper.find('[data-testid=settings-page]').exists()).toBe(true)
    expect(wrapper.find('.scrim .organisation-modal').exists()).toBe(false)
    expect(wrapper.get('[data-testid=people-section]').text()).toContain('Michael')
    expect(wrapper.get('[data-testid=people-section]').text()).toContain('admin')
    expect(wrapper.get('[data-testid=bots-section]').text()).toContain('Hector')
    expect(wrapper.get('[data-testid=bots-section]').text()).toContain('member')
    expect(wrapper.get('[data-testid=add-existing-user]').text()).toContain('Add existing user')
    await wrapper.get('[data-testid=back-to-chat]').trigger('click')
    expect(wrapper.find('[data-testid=composer]').exists()).toBe(true)
  })

  it('lets an admin add an existing human as a member', async () => {
    const fetcher = loadedFetcher()
    fetcher
      .mockImplementationOnce(() => json([{ id: 'u1', name: 'Michael', kind: 'human', role: 'admin' }]))
      .mockImplementationOnce(() => json([{ id: 'u3', name: 'Casey', email: 'casey@example.com' }]))
      .mockImplementationOnce(() => json({ id: 'u3', kind: 'human', name: 'Casey', role: 'member' }, 201))
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: class { close() {} } } } })
    await flushPromises()
    await wrapper.get('[data-testid=organisation-settings]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid=add-existing-user]').trigger('click')
    await wrapper.get('[data-testid=existing-email]').setValue('casey@example.com')
    await wrapper.get('[data-testid=search-existing-user-form]').trigger('submit')
    await flushPromises()
    expect(fetcher).toHaveBeenCalledWith('/api/organisations/o1/eligible-users?email=casey%40example.com', undefined)
    await wrapper.get('[data-testid=add-existing-user-u3]').trigger('click')
    await flushPromises()
    expect(fetcher).toHaveBeenLastCalledWith('/api/organisations/o1/users', expect.objectContaining({ method: 'POST', body: JSON.stringify({ user_id: 'u3' }) }))
    expect(wrapper.get('[data-testid=people-section]').text()).toContain('Casey')
    expect(wrapper.text()).toContain('Casey added as a member')
    expect(wrapper.find('[data-testid=add-existing-user-u3]').exists()).toBe(false)
  })

  it('shows a settings load failure instead of silently doing nothing', async () => {
    const fetcher = loadedFetcher()
    fetcher.mockImplementationOnce(() => json({ error: 'Roster unavailable' }, 500))
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: class { close() {} } } } })
    await flushPromises()
    await wrapper.get('[data-testid=organisation-settings]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid=settings-page]').exists()).toBe(false)
    expect(wrapper.text()).toContain('Roster unavailable')
  })

  it('returns to chat when a conversation is selected from settings', async () => {
    const fetcher = loadedFetcher()
    fetcher
      .mockImplementationOnce(() => json([{ id: 'u1', name: 'Michael', kind: 'human', role: 'admin' }]))
      .mockImplementationOnce(() => json([]))
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: class { close() {} } } } })
    await flushPromises()
    await wrapper.get('[data-testid=organisation-settings]').trigger('click')
    await flushPromises()
    await wrapper.get('nav button').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid=settings-page]').exists()).toBe(false)
    expect(wrapper.find('[data-testid=composer]').exists()).toBe(true)
  })

  it('hides organisation administration controls from members', async () => {
    const fetcher = vi.fn()
      .mockImplementationOnce(() => json({ id: 'u2', name: 'Member', kind: 'human' }))
      .mockImplementationOnce(() => json([{ id: 'o1', name: 'Mainstay', role: 'member' }]))
      .mockImplementationOnce(() => json([{ id: 'c1', name: 'general', visibility: 'organisation' }]))
      .mockImplementationOnce(() => json([]))
      .mockImplementationOnce(() => json([{ id: 'b1', name: 'Hector', kind: 'bot', role: 'member' }]))
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: class { close() {} } } } })
    await flushPromises()
    await wrapper.get('[data-testid=organisation-settings]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid=add-bot]').exists()).toBe(false)
    expect(wrapper.find('[data-testid=add-existing-user]').exists()).toBe(false)
    expect(wrapper.find('[data-testid=rotate-key-b1]').exists()).toBe(false)
    expect(wrapper.find('[data-testid=revoke-key-b1]').exists()).toBe(false)
    expect(wrapper.find('[data-testid=remove-bot-b1]').exists()).toBe(false)
    expect(wrapper.find('[data-testid=delete-conversation]').exists()).toBe(false)
  })

  it('lets an admin rotate and revoke a bot key', async () => {
    const fetcher = loadedFetcher()
    const roster = [{ id: 'b1', name: 'Hector', kind: 'bot', role: 'member' }]
    let finishRotation!: (response: Response) => void
    const rotation = new Promise<Response>(resolve => { finishRotation = resolve })
    fetcher
      .mockImplementationOnce(() => json(roster))
      .mockImplementationOnce(() => rotation)
      .mockImplementationOnce(() => Promise.resolve(new Response(null, { status: 204 })))
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true)
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: class { close() {} } } } })
    await flushPromises()
    await wrapper.get('[data-testid=organisation-settings]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid=rotate-key-b1]').trigger('click')
    expect(confirm).toHaveBeenCalledWith("Rotate Hector's key? Its current key will stop working immediately.")
    expect(wrapper.get('[data-testid=rotate-key-b1]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid=revoke-key-b1]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid=rotate-key-b1]').trigger('click')
    expect(fetcher.mock.calls.filter(([url]) => url === '/api/bots/b1/key')).toHaveLength(1)
    finishRotation(new Response(JSON.stringify({ api_key: 'km_live_rotated_secret' }), { status: 201, headers: { 'Content-Type': 'application/json' } }))
    await flushPromises()
    expect(wrapper.text()).toContain('Copy the rotated key now')
    expect(wrapper.text()).toContain('km_live_rotated_secret')
    await wrapper.get('.modal .close').trigger('click')
    expect(wrapper.text()).toContain('Copy the rotated key now')
    await wrapper.get('[data-testid=key-saved]').setValue(true)
    await wrapper.get('.modal .close').trigger('click')
    confirm.mockClear()
    confirm.mockReturnValueOnce(false)
    await wrapper.get('[data-testid=revoke-key-b1]').trigger('click')
    expect(confirm).toHaveBeenCalledWith("Revoke Hector's key? The bot will be disconnected immediately and will need a new key to reconnect.")
    expect(fetcher.mock.calls.filter(([, init]) => init?.method === 'DELETE')).toHaveLength(0)

    confirm.mockReturnValueOnce(true)
    await wrapper.get('[data-testid=revoke-key-b1]').trigger('click')
    await flushPromises()
    expect(fetcher).toHaveBeenCalledWith('/api/bots/b1/key', expect.objectContaining({ method: 'DELETE' }))
    expect(wrapper.text()).toContain('Hector key revoked')
    confirm.mockRestore()
  })

  it('lets an admin confirm bot removal and updates the settings list', async () => {
    const fetcher = loadedFetcher()
    fetcher
      .mockImplementationOnce(() => json([
        { id: 'u1', name: 'Michael', kind: 'human', role: 'admin' },
        { id: 'b1', name: 'Hector', kind: 'bot', role: 'member' },
      ]))
      .mockImplementationOnce(() => Promise.resolve(new Response(null, { status: 204 })))
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: class { close() {} } } } })
    await flushPromises()
    await wrapper.get('[data-testid=organisation-settings]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid=remove-bot-b1]').trigger('click')
    expect(wrapper.get('[data-testid=confirm-remove-b1]').text()).toContain('Remove Hector')
    await wrapper.get('[data-testid=confirm-remove-b1]').trigger('click')
    await flushPromises()
    expect(fetcher).toHaveBeenCalledWith('/api/organisations/o1/bots/b1', expect.objectContaining({ method: 'DELETE' }))
    expect(wrapper.get('[data-testid=bots-section]').text()).not.toContain('Hector')
    expect(wrapper.text()).toContain('Hector removed')
  })
})

function cssRule(selector: string) {
  const selectorStart = styles.indexOf(`\n${selector} {`)
  expect(selectorStart, `Missing CSS rule for ${selector}`).toBeGreaterThanOrEqual(0)
  const ruleStart = styles.indexOf('{', selectorStart)
  const ruleEnd = styles.indexOf('}', ruleStart)
  expect(ruleStart).toBeGreaterThan(selectorStart)
  expect(ruleEnd).toBeGreaterThan(ruleStart)
  return styles.slice(ruleStart + 1, ruleEnd)
}

function conversationButton(wrapper: VueWrapper, name: string) {
  const button = wrapper.findAll('.sidebar-navigation section > button').find(candidate => candidate.text().toLocaleLowerCase().includes(name.toLocaleLowerCase()))
  expect(button, `Missing conversation button for ${name}`).toBeDefined()
  return button!
}

function withInitialUsers(fetcher: ReturnType<typeof vi.fn>) {
  let initialUsersLoaded = false
  return vi.fn((url: string, init?: RequestInit) => {
    if (!initialUsersLoaded && /\/api\/organisations\/[^/]+\/users$/.test(url)) {
      initialUsersLoaded = true
      return json([])
    }
    return fetcher(url, init)
  })
}

function loadedFetcher() {
  return vi.fn()
    .mockImplementationOnce(() => json({ id: 'u1', name: 'Michael', kind: 'human' }))
    .mockImplementationOnce(() => json([{ id: 'o1', name: 'Mainstay', role: 'admin' }]))
    .mockImplementationOnce(() => json([{ id: 'c1', name: 'general', visibility: 'organisation' }]))
    .mockImplementationOnce(() => json([]))
}

function messagesForReadTest() {
  return [
    { id: 'm1', conversation_id: 'c1', author_id: 'u1', author_name: 'Michael', author_kind: 'human', body: 'Read', created_at: '2026-01-01T00:00:00Z', sequence: 1 },
    { id: 'm2', conversation_id: 'c1', author_id: 'b1', author_name: 'Hector', author_kind: 'bot', body: 'Unread', created_at: '2026-01-01T00:00:01Z', sequence: 2 },
  ]
}
