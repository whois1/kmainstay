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

  it('keeps the jump control inside the message region and suppresses stray scrollbars', () => {
    const shellRule = cssRule('.message-list-shell')
    expect(shellRule).toContain('min-height: 0')
    expect(shellRule).toContain('overflow: hidden')
    expect(shellRule).toContain('position: relative')

    const listRule = cssRule('.message-list')
    expect(listRule).toContain('height: 100%')
    expect(listRule).toContain('overflow-x: hidden')

    const jumpRule = cssRule('.jump-to-bottom')
    expect(jumpRule).toContain('bottom: 16px')
    expect(jumpRule).not.toContain('bottom: 112px')

    expect(cssRule('.message-list.has-jump-control')).toContain('padding-bottom: 80px')
    expect(cssRule('.markdown')).toContain('overflow-wrap: anywhere')

    const textareaRule = cssRule('.composer textarea')
    expect(textareaRule).toContain('field-sizing: content')
    expect(textareaRule).toContain('max-height: 160px')
    expect(textareaRule).toContain('overflow-y: auto')

    expect(styles).toContain('  .composer { grid-template-columns: minmax(0, 1fr); margin: 0 16px 16px; }')
    expect(styles).toContain('  .composer-actions { justify-content: flex-end; }')
  })

  it('keeps bulk conversation actions visible outside the scrolling navigation', () => {
    const rule = cssRule('.bulk-conversation-actions')
    expect(rule).toContain('display: grid')
    expect(rule).toContain('grid-template-columns: 1fr auto auto')
    expect(rule).toContain('border-top: 1px solid var(--border)')
    expect(styles).toContain('  .conversation-row.selectable-conversation { grid-template-columns: 24px minmax(0, 1fr); }')
    expect(styles).toContain('  .bulk-conversation-actions { grid-template-columns: 1fr; text-align: center; }')
    expect(styles).toContain('  .selectable-conversation > button, .sidebar-navigation section > button, .direct-contact-button, .direct-topic, .conversation-main { display: grid;')
  })

  it('constrains long conversation labels to the sidebar before applying an ellipsis', () => {
    expect(cssRule('.sidebar-navigation section')).toContain('min-width: 0')
    expect(cssRule('.direct-contact')).toContain('min-width: 0')
    expect(cssRule('.direct-topic')).toContain('min-width: 0')

    const rule = cssRule('.conversation-label')
    expect(rule).toContain('flex: 1 1 auto')
    expect(rule).toContain('min-width: 0')
    expect(rule).toContain('overflow: hidden')
    expect(rule).toContain('text-overflow: ellipsis')
    expect(rule).toContain('white-space: nowrap')
  })

  it('keeps conversation actions visible when the title is long', () => {
    expect(cssRule('.conversation > header')).toContain('min-width: 0')
    const copyRule = cssRule('.conversation-header-copy')
    expect(copyRule).toContain('flex: 1 1 auto')
    expect(copyRule).toContain('min-width: 0')
    expect(cssRule('.conversation-header-actions')).toContain('flex: 0 0 auto')
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

  it('does not expose General lifecycle mutations', async () => {
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(loadedFetcher()), socketFactory: class { close() {} } } } })
    await flushPromises()

    expect(wrapper.find('[data-testid=edit-conversation-title]').exists()).toBe(false)
    expect(wrapper.find('[data-testid=archive-conversation]').exists()).toBe(false)
    expect(wrapper.find('[data-testid=conversation-actions-menu]').exists()).toBe(false)
    expect(wrapper.find('[data-testid=pinned-conversations] [data-testid=conversation-checkbox]').exists()).toBe(false)
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

    const lastCheckbox = wrapper.findAll('[data-testid=create-conversation] input[type=checkbox]').at(-1)!
    ;(lastCheckbox.element as HTMLElement).focus()
    await dialog.trigger('keydown', { key: 'Tab' })
    expect(document.activeElement).toBe(wrapper.get('[data-testid=close-conversation-dialog]').element)

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
      .mockImplementationOnce(() => json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true }]))
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

  it('posts a selected reply and archives the conversation', async () => {
    const original = { id: 'm1', conversation_id: 'c1', author_id: 'b1', author_name: 'Hector', author_kind: 'bot', body: 'Original', created_at: '2026-01-01T00:00:00Z', sequence: 1 }
    let archived = false
    const fetcher = vi.fn((url: string, init?: RequestInit) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url.startsWith('/api/organisations/o1/conversations')) return json([{ id: 'c1', name: 'general', visibility: 'members', member_ids: ['u1', 'b1', 'u2'], archived }])
      if (url === '/api/conversations/c1/messages?limit=100') return json([original])
      if (url === '/api/organisations/o1/users') return json([])
      if (url === '/api/conversations/c1/read') return json({ sequence: 1 })
      if (url === '/api/conversations/c1/messages' && init?.method === 'POST') return json({ ...original, id: 'm2', author_id: 'u1', author_name: 'Michael', author_kind: 'human', body: 'Answer', sequence: 2, reply_to: { id: 'm1', author_name: 'Hector', body: 'Original' } }, 201)
      if (url === '/api/conversations/c1/archive' && init?.method === 'PUT') {
        archived = true
        return Promise.resolve(new Response(null, { status: 204 }))
      }
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: class { close() {} } } } })
    await flushPromises()

    await wrapper.get('[data-testid=reply-message]').trigger('click')
    await wrapper.get('textarea').setValue('Answer')
    await wrapper.get('[data-testid=composer]').trigger('submit')
    await flushPromises()
    const post = fetcher.mock.calls.find(([url, init]) => url === '/api/conversations/c1/messages' && init?.method === 'POST')
    expect(JSON.parse(String(post?.[1]?.body))).toMatchObject({ body: 'Answer', reply_to_message_id: 'm1' })

    await wrapper.get('[data-testid=archive-conversation]').trigger('click')
    await flushPromises()
    expect(fetcher).toHaveBeenCalledWith('/api/conversations/c1/archive', expect.objectContaining({ method: 'PUT' }))
    expect(wrapper.get('[data-testid=archived-conversations]').text()).toContain('general')
  })

  it('archives selected conversations in bulk', async () => {
    const archivedConversationIDs = new Set<string>()
    const fetcher = vi.fn((url: string, init?: RequestInit) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true') return json([
        { id: 'c1', name: 'general', visibility: 'members', member_ids: ['u1', 'b1', 'u2'], archived: archivedConversationIDs.has('c1') },
        { id: 'c2', name: 'planning', visibility: 'members', member_ids: ['u1', 'b1', 'u2'], archived: archivedConversationIDs.has('c2') },
      ])
      if (url === '/api/conversations/c1/messages?limit=100') return json([])
      if (url === '/api/organisations/o1/users') return json([])
      const archiveMatch = url.match(/^\/api\/conversations\/(c[12])\/archive$/)
      if (archiveMatch && init?.method === 'PUT') {
        archivedConversationIDs.add(archiveMatch[1])
        return Promise.resolve(new Response(null, { status: 204 }))
      }
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: class { close() {} } } } })
    await flushPromises()

    const checkboxes = wrapper.findAll('[data-testid=conversation-checkbox]')
    await checkboxes[0].trigger('click')
    await checkboxes[1].trigger('click')
    await wrapper.get('[data-testid=bulk-archive-conversations]').trigger('click')
    await flushPromises()

    expect(fetcher).toHaveBeenCalledWith('/api/conversations/c1/archive', expect.objectContaining({ method: 'PUT' }))
    expect(fetcher).toHaveBeenCalledWith('/api/conversations/c2/archive', expect.objectContaining({ method: 'PUT' }))
    expect(wrapper.get('[data-testid=archived-conversations]').text()).toContain('general')
    expect(wrapper.get('[data-testid=archived-conversations]').text()).toContain('planning')
  })

  it('keeps failed bulk archives selected for retry', async () => {
    let firstConversationArchived = false
    const fetcher = vi.fn((url: string, init?: RequestInit) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true') return json([
        { id: 'c1', name: 'general', visibility: 'members', member_ids: ['u1', 'b1', 'u2'], archived: firstConversationArchived },
        { id: 'c2', name: 'planning', visibility: 'members', member_ids: ['u1', 'b1', 'u2'] },
      ])
      if (url === '/api/conversations/c1/messages?limit=100') return json([])
      if (url === '/api/organisations/o1/users') return json([])
      if (url === '/api/conversations/c1/archive' && init?.method === 'PUT') {
        firstConversationArchived = true
        return Promise.resolve(new Response(null, { status: 204 }))
      }
      if (url === '/api/conversations/c2/archive' && init?.method === 'PUT') return json({ error: 'Archive failed' }, 500)
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: class { close() {} } } } })
    await flushPromises()

    const checkboxes = wrapper.findAll('[data-testid=conversation-checkbox]')
    await checkboxes[0].trigger('click')
    await checkboxes[1].trigger('click')
    await wrapper.get('[data-testid=bulk-archive-conversations]').trigger('click')
    await flushPromises()

    const checkedLabels = wrapper.findAll('[data-testid=conversation-checkbox]')
      .filter(checkbox => (checkbox.element as HTMLInputElement).checked)
      .map(checkbox => checkbox.attributes('aria-label'))
    expect(checkedLabels).toEqual(['Select planning'])
    expect(wrapper.get('[data-testid=bulk-conversation-actions]').text()).toContain('1 selected')
  })

  it('refreshes, expires and explicitly clears bot activity', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-27T12:00:00Z'))
    class EventSocket {
      static instance: EventSocket
      onopen: (() => void) | null = null
      onmessage: ((event: MessageEvent) => void) | null = null
      onclose: (() => void) | null = null
      close() {}
      constructor() { EventSocket.instance = this }
    }
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(loadedFetcher()), socketFactory: EventSocket } } })
    await flushPromises()
    const sendActivity = (active: boolean, expiresAt = new Date(Date.now() + 6000).toISOString()) => {
      EventSocket.instance.onmessage?.({ data: JSON.stringify({ type: 'conversation.activity', payload: { conversation_id: 'c1', user_id: 'b1', user_name: 'Hector', user_kind: 'bot', active, expires_at: expiresAt } }) } as MessageEvent)
    }

    sendActivity(true)
    await wrapper.vm.$nextTick()
    expect(wrapper.get('[data-testid=agent-activity]').text()).toBe('Hector is working…')

    vi.advanceTimersByTime(5000)
    sendActivity(true)
    vi.advanceTimersByTime(1001)
    await wrapper.vm.$nextTick()
    expect(wrapper.find('[data-testid=agent-activity]').exists()).toBe(true)
    vi.advanceTimersByTime(5000)
    await wrapper.vm.$nextTick()
    expect(wrapper.find('[data-testid=agent-activity]').exists()).toBe(false)

    sendActivity(true)
    sendActivity(false)
    await wrapper.vm.$nextTick()
    expect(wrapper.find('[data-testid=agent-activity]').exists()).toBe(false)

    sendActivity(true)
    EventSocket.instance.onmessage?.({ data: JSON.stringify({ type: 'message.created', sequence: 1, payload: { id: 'm1', conversation_id: 'c1', author_id: 'b1', author_name: 'Hector', author_kind: 'bot', body: 'Done', created_at: new Date().toISOString(), sequence: 1 } }) } as MessageEvent)
    await wrapper.vm.$nextTick()
    expect(wrapper.find('[data-testid=agent-activity]').exists()).toBe(false)

    wrapper.unmount()
    vi.useRealTimers()
  })

  it('uses the fetched active state when message activity commits after an archive', async () => {
    class EventSocket {
      static instance: EventSocket
      onopen: (() => void) | null = null
      onmessage: ((event: MessageEvent) => void) | null = null
      onclose: (() => void) | null = null
      close() {}
      constructor() { EventSocket.instance = this }
    }
    let finishArchive!: (response: Response) => void
    const pendingArchive = new Promise<Response>(resolve => { finishArchive = resolve })
    let conversationLoads = 0
    const realtimeMessage = { id: 'm1', conversation_id: 'c1', author_id: 'b1', author_name: 'Hector', author_kind: 'bot', body: 'New activity', created_at: '2026-01-01T00:00:01Z', sequence: 1 }
    const fetcher = vi.fn((url: string, init?: RequestInit) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true') {
        conversationLoads++
        return json([{ id: 'c1', name: 'general', visibility: 'members', member_ids: ['u1', 'b1', 'u2'], archived: false }])
      }
      if (url === '/api/conversations/c1/messages?limit=100') return json([])
      if (url === '/api/organisations/o1/users') return json([])
      if (url === '/api/conversations/c1/archive' && init?.method === 'PUT') return pendingArchive
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: EventSocket } } })
    await flushPromises()

    void wrapper.get('[data-testid=archive-conversation]').trigger('click')
    await Promise.resolve()
    EventSocket.instance.onmessage?.({ data: JSON.stringify({ type: 'message.created', sequence: 1, payload: realtimeMessage }) } as MessageEvent)
    finishArchive(new Response(null, { status: 204 }))
    await flushPromises()

    expect(conversationLoads).toBe(2)
    expect(wrapper.find('[data-testid=archived-conversations]').exists()).toBe(false)
    expect(wrapper.find('[data-testid=archive-conversation]').exists()).toBe(true)
  })

  it('uses the fetched archived state when an archive commits after realtime activity', async () => {
    class EventSocket {
      static instance: EventSocket
      onopen: (() => void) | null = null
      onmessage: ((event: MessageEvent) => void) | null = null
      onclose: (() => void) | null = null
      close() {}
      constructor() { EventSocket.instance = this }
    }
    let finishArchive!: (response: Response) => void
    const pendingArchive = new Promise<Response>(resolve => { finishArchive = resolve })
    let conversationLoads = 0
    const realtimeMessage = { id: 'm1', conversation_id: 'c1', author_id: 'b1', author_name: 'Hector', author_kind: 'bot', body: 'New activity', created_at: '2026-01-01T00:00:01Z', sequence: 1 }
    const fetcher = vi.fn((url: string, init?: RequestInit) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true') {
        conversationLoads++
        return json([{ id: 'c1', name: 'general', visibility: 'members', member_ids: ['u1', 'b1', 'u2'], archived: conversationLoads > 1 }])
      }
      if (url === '/api/conversations/c1/messages?limit=100') return json([])
      if (url === '/api/organisations/o1/users') return json([])
      if (url === '/api/conversations/c1/archive' && init?.method === 'PUT') return pendingArchive
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: EventSocket } } })
    await flushPromises()

    void wrapper.get('[data-testid=archive-conversation]').trigger('click')
    await Promise.resolve()
    EventSocket.instance.onmessage?.({ data: JSON.stringify({ type: 'message.created', sequence: 1, payload: realtimeMessage }) } as MessageEvent)
    finishArchive(new Response(null, { status: 204 }))
    await flushPromises()

    expect(conversationLoads).toBe(2)
    expect(wrapper.get('[data-testid=archived-conversations]').text()).toContain('general')
    expect(wrapper.find('[data-testid=restore-conversation]').exists()).toBe(true)
  })

  it('does not let a conversation refresh overwrite a newer realtime unarchive', async () => {
    class EventSocket {
      static instance: EventSocket
      onopen: (() => void) | null = null
      onmessage: ((event: MessageEvent) => void) | null = null
      onclose: (() => void) | null = null
      close() {}
      constructor() { EventSocket.instance = this }
    }
    let finishRefresh!: (response: Response) => void
    const pendingRefresh = new Promise<Response>(resolve => { finishRefresh = resolve })
    let conversationLoads = 0
    const realtimeMessage = { id: 'm1', conversation_id: 'c1', author_id: 'b1', author_name: 'Hector', author_kind: 'bot', body: 'New activity', created_at: '2026-01-01T00:00:01Z', sequence: 1 }
    const fetcher = vi.fn((url: string) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true') {
        conversationLoads++
        if (conversationLoads === 1) return json([{ id: 'c1', name: 'general', visibility: 'members', member_ids: ['u1', 'b1', 'u2'], archived: false }])
        return pendingRefresh
      }
      if (url === '/api/conversations/c1/messages?limit=100') return json([])
      if (url === '/api/organisations/o1/users') return json([])
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: EventSocket } } })
    await flushPromises()

    EventSocket.instance.onopen?.()
    await Promise.resolve()
    EventSocket.instance.onmessage?.({ data: JSON.stringify({ type: 'message.created', sequence: 1, payload: realtimeMessage }) } as MessageEvent)
    finishRefresh(await json([{ id: 'c1', name: 'general', visibility: 'members', member_ids: ['u1', 'b1', 'u2'], archived: true, latest_sequence: 0 }]))
    await flushPromises()

    expect(wrapper.find('[data-testid=archived-conversations]').exists()).toBe(false)
    expect(wrapper.find('[data-testid=archive-conversation]').exists()).toBe(true)
  })

  it('does not let a stale conversation refresh overwrite a completed archive action', async () => {
    class EventSocket {
      static instance: EventSocket
      onopen: (() => void) | null = null
      onmessage: ((event: MessageEvent) => void) | null = null
      onclose: (() => void) | null = null
      close() {}
      constructor() { EventSocket.instance = this }
    }
    let finishRefresh!: (response: Response) => void
    const pendingRefresh = new Promise<Response>(resolve => { finishRefresh = resolve })
    let conversationLoads = 0
    const fetcher = vi.fn((url: string, init?: RequestInit) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true') {
        conversationLoads++
        if (conversationLoads === 1) return json([{ id: 'c1', name: 'general', visibility: 'members', member_ids: ['u1', 'b1', 'u2'], archived: false }])
        if (conversationLoads === 2) return pendingRefresh
        return json([{ id: 'c1', name: 'general', visibility: 'members', member_ids: ['u1', 'b1', 'u2'], archived: true }])
      }
      if (url === '/api/conversations/c1/messages?limit=100') return json([])
      if (url === '/api/organisations/o1/users') return json([])
      if (url === '/api/conversations/c1/archive' && init?.method === 'PUT') return Promise.resolve(new Response(null, { status: 204 }))
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: EventSocket } } })
    await flushPromises()

    EventSocket.instance.onopen?.()
    await Promise.resolve()
    await wrapper.get('[data-testid=archive-conversation]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid=archived-conversations]').text()).toContain('general')

    finishRefresh(await json([{ id: 'c1', name: 'general', visibility: 'members', member_ids: ['u1', 'b1', 'u2'], archived: false }]))
    await flushPromises()

    expect(wrapper.get('[data-testid=archived-conversations]').text()).toContain('general')
  })

  it('shows chronological complete messages and posts from the composer', async () => {
    const fetcher = vi.fn()
      .mockImplementationOnce(() => json({ id: 'u1', name: 'Michael', kind: 'human' }))
      .mockImplementationOnce(() => json([{ id: 'o1', name: 'Mainstay', role: 'admin' }]))
      .mockImplementationOnce(() => json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true }]))
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

  it('posts multiple images in one multipart message', async () => {
    const attachments = [
      { id: 'first-attachment', media_type: 'image/png', byte_size: 3, width: 1, height: 1, original_filename: 'first.png', created_at: '2026-01-01T00:00:00Z', content_url: '/api/attachments/first-attachment/content' },
      { id: 'second-attachment', media_type: 'image/jpeg', byte_size: 3, width: 1, height: 1, original_filename: 'second.jpg', created_at: '2026-01-01T00:00:00Z', content_url: '/api/attachments/second-attachment/content' },
    ]
    const fetcher = vi.fn((url: string, init?: RequestInit) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true') return json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true }])
      if (url === '/api/conversations/c1/messages?limit=100' && !init) return json([])
      if (url === '/api/organisations/o1/users') return json([])
      if (url === '/api/conversations/c1/messages' && init?.method === 'POST') return json({ id: 'm1', conversation_id: 'c1', author_id: 'u1', author_name: 'Michael', author_kind: 'human', body: '', created_at: '2026-01-01T00:00:00Z', sequence: 1, attachments }, 201)
      if (url === '/api/conversations/c1/read') return json({ sequence: 1 })
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: class { close() {} } } } })
    await flushPromises()
    const first = new File([new Uint8Array([1, 2, 3])], 'first.png', { type: 'image/png' })
    const second = new File([new Uint8Array([4, 5, 6])], 'second.jpg', { type: 'image/jpeg' })
    const input = wrapper.get('input[type=file]')
    Object.defineProperty(input.element, 'files', { configurable: true, value: [first, second] })
    await input.trigger('change')
    await wrapper.get('[data-testid=composer]').trigger('submit')
    await flushPromises()

    const post = fetcher.mock.calls.find(([url, init]) => url === '/api/conversations/c1/messages' && init?.method === 'POST')
    expect(post).toBeDefined()
    const form = post?.[1]?.body as FormData
    expect(form).toBeInstanceOf(FormData)
    expect(form.get('body')).toBe('')
    expect(form.getAll('image')).toEqual([first, second])
    expect(wrapper.findAll('[data-testid=message-image]').map(image => image.attributes('src'))).toEqual(attachments.map(attachment => attachment.content_url))
    expect(wrapper.find('[data-testid=selected-image]').exists()).toBe(false)
  })

  it('reuses the image message client ID after an ambiguous response failure', async () => {
    let postCount = 0
    const fetcher = vi.fn((url: string, init?: RequestInit) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true') return json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true }])
      if (url === '/api/conversations/c1/messages?limit=100' && !init) return json([])
      if (url === '/api/organisations/o1/users') return json([])
      if (url === '/api/conversations/c1/messages' && init?.method === 'POST') {
        postCount++
        if (postCount === 1) return Promise.reject(new Error('response lost'))
        return json({ id: 'm1', conversation_id: 'c1', author_id: 'u1', author_name: 'Michael', author_kind: 'human', body: '', created_at: '2026-01-01T00:00:00Z', sequence: 1, attachments: [] }, 200)
      }
      if (url === '/api/conversations/c1/read') return json({ sequence: 1 })
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: class { close() {} } } } })
    await flushPromises()
    const file = new File([new Uint8Array([1])], 'photo.png', { type: 'image/png' })
    const input = wrapper.get('input[type=file]')
    Object.defineProperty(input.element, 'files', { configurable: true, value: [file] })
    await input.trigger('change')
    await wrapper.get('[data-testid=composer]').trigger('submit')
    await flushPromises()
    expect(wrapper.find('[data-testid=selected-image]').exists()).toBe(true)
    await wrapper.get('[data-testid=composer]').trigger('submit')
    await flushPromises()

    const posts = fetcher.mock.calls.filter(([url, init]) => url === '/api/conversations/c1/messages' && init?.method === 'POST')
    expect(posts).toHaveLength(2)
    const firstClientID = (posts[0][1]?.body as FormData).get('client_id')
    const secondClientID = (posts[1][1]?.body as FormData).get('client_id')
    expect(secondClientID).toBe(firstClientID)
  })

  it('reuses the message client ID when retrying the same reply target after response loss', async () => {
    const posts: Array<Record<string, string>> = []
    const original = { id: 'm1', conversation_id: 'c1', author_id: 'b1', author_name: 'Hector', author_kind: 'bot', body: 'Original', created_at: '2026-01-01T00:00:00Z', sequence: 1 }
    const fetcher = replyRetryFetcher(original, posts)
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: class { close() {} } } } })
    await flushPromises()

    await wrapper.get('[data-testid=reply-message]').trigger('click')
    await wrapper.get('textarea').setValue('Answer')
    await wrapper.get('[data-testid=composer]').trigger('submit')
    await flushPromises()
    await wrapper.get('[data-testid=composer]').trigger('submit')
    await flushPromises()

    expect(posts).toHaveLength(2)
    expect(posts[1].client_id).toBe(posts[0].client_id)
    expect(posts[1].reply_to_message_id).toBe('m1')
  })

  it.each([
    ['changes', async (wrapper: VueWrapper) => { await wrapper.findAll('[data-testid=reply-message]')[1].trigger('click') }],
    ['cancels', async (wrapper: VueWrapper) => { await wrapper.get('[aria-label="Cancel reply"]').trigger('click') }],
  ])('uses a new message client ID when the user %s the reply target after response loss', async (_action, changeReplyIntent) => {
    const posts: Array<Record<string, string>> = []
    const original = { id: 'm1', conversation_id: 'c1', author_id: 'b1', author_name: 'Hector', author_kind: 'bot', body: 'Original', created_at: '2026-01-01T00:00:00Z', sequence: 1 }
    const second = { ...original, id: 'm2', body: 'Second', sequence: 2 }
    const fetcher = replyRetryFetcher([original, second], posts)
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: class { close() {} } } } })
    await flushPromises()

    await wrapper.findAll('[data-testid=reply-message]')[0].trigger('click')
    await wrapper.get('textarea').setValue('Answer')
    await wrapper.get('[data-testid=composer]').trigger('submit')
    await flushPromises()
    await changeReplyIntent(wrapper)
    await wrapper.get('[data-testid=composer]').trigger('submit')
    await flushPromises()

    expect(posts).toHaveLength(2)
    expect(posts[1].client_id).not.toBe(posts[0].client_id)
    expect(posts[1].reply_to_message_id).toBe(_action === 'changes' ? 'm2' : undefined)
  })

  it('marks through the latest loaded message after initial positioning', async () => {
	const fetcher = vi.fn()
	  .mockImplementationOnce(() => json({ id: 'u1', name: 'Michael', kind: 'human' }))
	  .mockImplementationOnce(() => json([{ id: 'o1', name: 'Mainstay', role: 'admin' }]))
	  .mockImplementationOnce(() => json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true, read_sequence: 1 }]))
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
      if (url === '/api/organisations/o1/conversations?include_archived=true') return json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true, read_sequence: 2, latest_sequence: 3 }])
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
      if (url === '/api/organisations/o1/conversations?include_archived=true') return json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true, read_sequence: 2, latest_sequence: 3 }])
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
      if (url === '/api/organisations/o1/conversations?include_archived=true') return json([
        { id: 'c1', name: 'general', visibility: 'organisation', is_general: true, read_sequence: 1, latest_sequence: 2 },
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
      .mockImplementationOnce(() => json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true, read_sequence: 1, latest_sequence: 201 }]))
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
      if (url === '/api/organisations/o1/conversations?include_archived=true') return json([
        { id: 'c1', name: 'general', visibility: 'organisation', is_general: true, read_sequence: 0, latest_sequence: 0 },
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
      if (url === '/api/organisations/o1/conversations?include_archived=true') return json([
        { id: 'c1', name: 'general', visibility: 'organisation', is_general: true, read_sequence: 0, latest_sequence: 0 },
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
      if (url === '/api/organisations/o1/conversations?include_archived=true') return json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true, read_sequence: 1, latest_sequence: 201 }])
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

  it('creates a named group chat only after at least two other users are selected', async () => {
    const fetcher = loadedFetcher()
    fetcher.mockImplementationOnce(() => json([{ id: 'u1', name: 'Michael', kind: 'human' }, { id: 'b1', name: 'Hector', kind: 'bot' }, { id: 'u2', name: 'Mary', kind: 'human' }]))
    fetcher.mockImplementationOnce(() => json({ id: 'c2', name: 'Planning', visibility: 'members', member_ids: ['u1', 'b1', 'u2'] }, 201))
    fetcher.mockImplementationOnce(() => json([]))
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: class { close() {} } } } })
    await flushPromises()
    await wrapper.get('[data-testid=new-conversation]').trigger('click')
    await flushPromises()
    expect(wrapper.get('.dialog-context').text()).toBe('New group chat')
    expect(wrapper.get('#conversation-dialog-title').text()).toBe('Create group chat')
    const form = wrapper.get('[data-testid=create-conversation]')
    const nameInput = wrapper.get('[data-testid=conversation-name]')
    const closeButton = wrapper.get('[data-testid=close-conversation-dialog]')
    expect(closeButton.element.closest('.dialog-header')).not.toBeNull()
    expect(closeButton.element.closest('form')).toBeNull()
    expect(form.attributes('autocomplete')).toBe('off')
    expect(nameInput.attributes('name')).toBe('group_chat_title')
    expect(nameInput.attributes('autocomplete')).toBe('off')
    expect(nameInput.attributes('data-1p-ignore')).toBe('true')
    expect(nameInput.attributes('data-lpignore')).toBe('true')
    expect(nameInput.attributes('data-bwignore')).toBe('true')
    const submit = wrapper.get('[data-testid=create-conversation] button[type=submit]')
    expect(submit.attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid=conversation-name]').setValue('Planning')
    await wrapper.get('input[value=b1]').setValue(true)
    expect(submit.attributes('disabled')).toBeDefined()
    await wrapper.get('input[value=u2]').setValue(true)
    expect(submit.attributes('disabled')).toBeUndefined()
    await wrapper.get('[data-testid=create-conversation]').trigger('submit')
    await flushPromises()
    expect(fetcher).toHaveBeenCalledWith('/api/organisations/o1/conversations', expect.objectContaining({ body: JSON.stringify({ name: 'Planning', visibility: 'members', member_ids: ['b1', 'u2'] }) }))
    expect(wrapper.get('h1').text()).toContain('Planning')
  })

  it('keeps a direct draft when a delayed group roster arrives', async () => {
    let finishGroupRoster!: (response: Response) => void
    const pendingGroupRoster = new Promise<Response>(resolve => { finishGroupRoster = resolve })
    let rosterRequests = 0
    const fetcher = vi.fn((url: string) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true') return json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true }])
      if (url === '/api/conversations/c1/messages?limit=100') return json([])
      if (url === '/api/organisations/o1/users') {
        rosterRequests++
        if (rosterRequests === 1) return json([
          { id: 'u1', name: 'Michael', kind: 'human', role: 'admin' },
          { id: 'b1', name: 'Hector', kind: 'bot', role: 'member' },
        ])
        return pendingGroupRoster
      }
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: class { close() {} } } } })
    await flushPromises()

    void wrapper.get('[data-testid=new-conversation]').trigger('click')
    await Promise.resolve()
    await wrapper.get('[data-testid=new-direct-topic-b1]').trigger('click')
    await wrapper.get('[data-testid=composer] textarea').setValue('Agent navigation')

    finishGroupRoster(await json([
      { id: 'u1', name: 'Michael', kind: 'human', role: 'admin' },
      { id: 'b1', name: 'Hector', kind: 'bot', role: 'member' },
      { id: 'u2', name: 'Mary', kind: 'human', role: 'member' },
    ]))
    await flushPromises()

    expect(wrapper.find('[data-testid=create-conversation]').exists()).toBe(false)
    expect(wrapper.get('h1').text()).toBe('New Hector session')
    expect((wrapper.get('[data-testid=composer] textarea').element as HTMLTextAreaElement).value).toBe('Agent navigation')
  })

  it('starts a direct chat without a dialog and creates it when the first message is sent', async () => {
    const fetcher = vi.fn((url: string, init?: RequestInit) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true' && !init) return json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true }])
      if (url === '/api/organisations/o1/conversations' && init?.method === 'POST') return json({ id: 'c2', name: 'New Hector session', visibility: 'members', member_ids: ['u1', 'b1'], title_automatic: true, activity_at: '2026-08-24T09:00:00Z' }, 201)
      if (url === '/api/conversations/c1/messages?limit=100') return json([])
      if (url === '/api/conversations/c2/messages' && init?.method === 'POST') return json({ id: 'm1', conversation_id: 'c2', author_id: 'u1', author_name: 'Michael', author_kind: 'human', body: 'Improve agent navigation', created_at: '2026-08-24T09:00:01Z', sequence: 1 }, 201)
      if (url === '/api/organisations/o1/users') return json([
        { id: 'u1', name: 'Michael', kind: 'human', role: 'admin' },
        { id: 'b1', name: 'Hector', kind: 'bot', role: 'member' },
      ])
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { attachTo: document.body, global: { provide: { fetcher, socketFactory: class { close() {} } } } })
    await flushPromises()

    await wrapper.get('[data-testid=new-direct-topic-b1]').trigger('click')

    expect(wrapper.find('[data-testid=create-conversation]').exists()).toBe(false)
    expect(wrapper.get('h1').text()).toBe('New Hector session')
    expect(wrapper.get('[data-testid=composer] textarea').attributes('placeholder')).toBe('Message New Hector session')
    expect(document.activeElement).toBe(wrapper.get('[data-testid=composer] textarea').element)
    expect(fetcher.mock.calls.filter(([url, init]) => url === '/api/organisations/o1/conversations' && init?.method === 'POST')).toHaveLength(0)

    await wrapper.get('[data-testid=composer] textarea').setValue('Improve agent navigation')
    await wrapper.get('[data-testid=composer]').trigger('submit')
    await flushPromises()

    const creationBody = JSON.parse(String(fetcher.mock.calls.find(([url, init]) => url === '/api/organisations/o1/conversations' && init?.method === 'POST')?.[1]?.body))
    expect(creationBody).toEqual({ name: 'New Hector session', visibility: 'members', member_ids: ['b1'], automatic_title: true, client_id: expect.any(String) })
    expect(fetcher).toHaveBeenCalledWith('/api/conversations/c2/messages', expect.objectContaining({ method: 'POST' }))
    expect(wrapper.get('h1').text()).toBe('Improve agent navigation')
    expect(wrapper.text()).toContain('Improve agent navigation')
    wrapper.unmount()
  })

  it('keeps an unsent chat draft across a reconnect refresh', async () => {
    class EventSocket {
      static instance: EventSocket
      onopen: (() => void) | null = null
      onmessage: ((event: MessageEvent) => void) | null = null
      onclose: (() => void) | null = null
      close() {}
      constructor() { EventSocket.instance = this }
    }
    let conversationLoads = 0
    const fetcher = vi.fn((url: string) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true') {
        conversationLoads++
        return json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true }])
      }
      if (url === '/api/conversations/c1/messages?limit=100') return json([])
      if (url === '/api/organisations/o1/users') return json([
        { id: 'u1', name: 'Michael', kind: 'human', role: 'admin' },
        { id: 'b1', name: 'Hector', kind: 'bot', role: 'member' },
      ])
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: EventSocket } } })
    await flushPromises()

    await wrapper.get('[data-testid=new-direct-topic-b1]').trigger('click')
    await wrapper.get('[data-testid=composer] textarea').setValue('Unsent draft')
    EventSocket.instance.onopen?.()
    await flushPromises()

    expect(conversationLoads).toBe(2)
    expect(wrapper.get('h1').text()).toBe('New Hector session')
    expect((wrapper.get('[data-testid=composer] textarea').element as HTMLTextAreaElement).value).toBe('Unsent draft')
  })

  it('ignores a stale reconnect refresh after a draft becomes a conversation', async () => {
    class EventSocket {
      static instance: EventSocket
      onopen: (() => void) | null = null
      onmessage: ((event: MessageEvent) => void) | null = null
      onclose: (() => void) | null = null
      close() {}
      constructor() { EventSocket.instance = this }
    }
    let finishRefresh!: (response: Response) => void
    const pendingRefresh = new Promise<Response>(resolve => { finishRefresh = resolve })
    let conversationLoads = 0
    const fetcher = vi.fn((url: string, init?: RequestInit) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true' && !init) {
        conversationLoads++
        if (conversationLoads === 1) return json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true }])
        return pendingRefresh
      }
      if (url === '/api/organisations/o1/conversations' && init?.method === 'POST') return json({ id: 'c2', name: 'New Hector session', visibility: 'members', member_ids: ['u1', 'b1'], title_automatic: true }, 201)
      if (url === '/api/conversations/c1/messages?limit=100') return json([])
      if (url === '/api/conversations/c2/messages' && init?.method === 'POST') return json({ id: 'm1', conversation_id: 'c2', author_id: 'u1', author_name: 'Michael', author_kind: 'human', body: 'Keep this chat selected', created_at: '2026-08-24T09:00:01Z', sequence: 1 }, 201)
      if (url === '/api/organisations/o1/users') return json([
        { id: 'u1', name: 'Michael', kind: 'human', role: 'admin' },
        { id: 'b1', name: 'Hector', kind: 'bot', role: 'member' },
      ])
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: EventSocket } } })
    await flushPromises()

    await wrapper.get('[data-testid=new-direct-topic-b1]').trigger('click')
    EventSocket.instance.onopen?.()
    await wrapper.get('[data-testid=composer] textarea').setValue('Keep this chat selected')
    await wrapper.get('[data-testid=composer]').trigger('submit')
    await flushPromises()

    finishRefresh(await json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true }]))
    await flushPromises()

    expect(wrapper.get('h1').text()).toBe('Keep this chat selected')
    expect(wrapper.text()).toContain('Keep this chat selected')
    expect(wrapper.findAll('[data-testid=direct-contact-b1] .direct-topic')).toHaveLength(1)
  })

  it('does not duplicate a created conversation already returned by reconnect refresh', async () => {
    class EventSocket {
      static instance: EventSocket
      onopen: (() => void) | null = null
      onmessage: ((event: MessageEvent) => void) | null = null
      onclose: (() => void) | null = null
      close() {}
      constructor() { EventSocket.instance = this }
    }
    let finishCreation!: (response: Response) => void
    const pendingCreation = new Promise<Response>(resolve => { finishCreation = resolve })
    let conversationLoads = 0
    const fetcher = vi.fn((url: string, init?: RequestInit) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true' && !init) {
        conversationLoads++
        if (conversationLoads === 1) return json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true }])
        return json([
          { id: 'c1', name: 'general', visibility: 'organisation', is_general: true },
          { id: 'c2', name: 'New Hector session', visibility: 'members', member_ids: ['u1', 'b1'], title_automatic: true },
        ])
      }
      if (url === '/api/organisations/o1/conversations' && init?.method === 'POST') return pendingCreation
      if (url === '/api/conversations/c1/messages?limit=100') return json([])
      if (url === '/api/conversations/c2/messages' && init?.method === 'POST') return json({ id: 'm1', conversation_id: 'c2', author_id: 'u1', author_name: 'Michael', author_kind: 'human', body: 'One chat only', created_at: '2026-08-24T09:00:01Z', sequence: 1 }, 201)
      if (url === '/api/organisations/o1/users') return json([
        { id: 'u1', name: 'Michael', kind: 'human', role: 'admin' },
        { id: 'b1', name: 'Hector', kind: 'bot', role: 'member' },
      ])
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: EventSocket } } })
    await flushPromises()

    await wrapper.get('[data-testid=new-direct-topic-b1]').trigger('click')
    await wrapper.get('[data-testid=composer] textarea').setValue('One chat only')
    void wrapper.get('[data-testid=composer]').trigger('submit')
    await Promise.resolve()
    EventSocket.instance.onopen?.()
    await flushPromises()

    finishCreation(await json({ id: 'c2', name: 'New Hector session', visibility: 'members', member_ids: ['u1', 'b1'], title_automatic: true }, 201))
    await flushPromises()

    expect(wrapper.findAll('[data-testid=direct-contact-b1] .direct-topic')).toHaveLength(1)
    expect(wrapper.get('h1').text()).toBe('One chat only')
  })

  it('keeps the first message in the draft when session creation fails', async () => {
    const fetcher = vi.fn((url: string, init?: RequestInit) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true' && !init) return json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true }])
      if (url === '/api/organisations/o1/conversations' && init?.method === 'POST') return json({ error: 'Could not create chat' }, 500)
      if (url === '/api/conversations/c1/messages?limit=100') return json([])
      if (url === '/api/organisations/o1/users') return json([
        { id: 'u1', name: 'Michael', kind: 'human', role: 'admin' },
        { id: 'b1', name: 'Hector', kind: 'bot', role: 'member' },
      ])
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: class { close() {} } } } })
    await flushPromises()

    await wrapper.get('[data-testid=new-direct-topic-b1]').trigger('click')
    await wrapper.get('[data-testid=composer] textarea').setValue('Keep this message')
    await wrapper.get('[data-testid=composer]').trigger('submit')
    await flushPromises()

    expect(wrapper.get('[role=alert]').text()).toContain('Could not create chat')
    expect((wrapper.get('[data-testid=composer] textarea').element as HTMLTextAreaElement).value).toBe('Keep this message')
    expect(wrapper.get('h1').text()).toBe('New Hector session')
  })

  it('reuses the conversation creation client ID after an ambiguous response failure', async () => {
    let creationAttempts = 0
    const fetcher = vi.fn((url: string, init?: RequestInit) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true' && !init) return json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true }])
      if (url === '/api/organisations/o1/conversations' && init?.method === 'POST') {
        creationAttempts++
        if (creationAttempts === 1) return Promise.reject(new Error('response lost'))
        return json({ id: 'c2', name: 'New Hector session', visibility: 'members', member_ids: ['u1', 'b1'], title_automatic: true }, 200)
      }
      if (url === '/api/conversations/c1/messages?limit=100') return json([])
      if (url === '/api/conversations/c2/messages' && init?.method === 'POST') return json({ id: 'm1', conversation_id: 'c2', author_id: 'u1', author_name: 'Michael', author_kind: 'human', body: 'Retry safely', created_at: '2026-08-24T09:00:01Z', sequence: 1 }, 201)
      if (url === '/api/organisations/o1/users') return json([
        { id: 'u1', name: 'Michael', kind: 'human', role: 'admin' },
        { id: 'b1', name: 'Hector', kind: 'bot', role: 'member' },
      ])
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: class { close() {} } } } })
    await flushPromises()

    await wrapper.get('[data-testid=new-direct-topic-b1]').trigger('click')
    await wrapper.get('[data-testid=composer] textarea').setValue('Retry safely')
    await wrapper.get('[data-testid=composer]').trigger('submit')
    await flushPromises()
    await wrapper.get('[data-testid=composer]').trigger('submit')
    await flushPromises()

    const creationBodies = fetcher.mock.calls
      .filter(([url, init]) => url === '/api/organisations/o1/conversations' && init?.method === 'POST')
      .map(([, init]) => JSON.parse(String(init?.body)))
    expect(creationBodies).toHaveLength(2)
    expect(creationBodies[0].client_id).toEqual(expect.any(String))
    expect(creationBodies[1].client_id).toBe(creationBodies[0].client_id)
    expect(wrapper.get('h1').text()).toBe('Retry safely')
    expect(wrapper.findAll('[data-testid=direct-contact-b1] .direct-topic')).toHaveLength(1)
  })

  it('keeps title saving disabled per conversation across navigation', async () => {
    let finishFirstSave!: (response: Response) => void
    let finishSecondSave!: (response: Response) => void
    const firstSave = new Promise<Response>(resolve => { finishFirstSave = resolve })
    const secondSave = new Promise<Response>(resolve => { finishSecondSave = resolve })
    const fetcher = vi.fn((url: string, init?: RequestInit) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true') return json([
        { id: 'c1', name: 'Alpha', visibility: 'members', member_ids: ['u1', 'b1', 'u2'] },
        { id: 'c2', name: 'Beta', visibility: 'members', member_ids: ['u1', 'b1', 'u2'] },
      ])
      if (url === '/api/conversations/c1/messages?limit=100' || url === '/api/conversations/c2/messages?limit=100') return json([])
      if (url === '/api/organisations/o1/users') return json([])
      if (url === '/api/conversations/c1/title' && init?.method === 'PUT') return firstSave
      if (url === '/api/conversations/c2/title' && init?.method === 'PUT') return secondSave
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: class { close() {} } } } })
    await flushPromises()

    await wrapper.get('[data-testid=edit-conversation-title]').trigger('click')
    await wrapper.get('[data-testid=conversation-title-input]').setValue('Alpha edited')
    await wrapper.get('[data-testid=conversation-title-form]').trigger('submit')
    await wrapper.findAll('nav button').find(button => button.text().includes('Beta'))!.trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid=edit-conversation-title]').trigger('click')
    await wrapper.get('[data-testid=conversation-title-input]').setValue('Beta edited')
    await wrapper.get('[data-testid=conversation-title-form]').trigger('submit')
    await wrapper.findAll('nav button').find(button => button.text().includes('Alpha'))!.trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid=edit-conversation-title]').attributes('disabled')).toBeDefined()
    finishSecondSave(await json({ id: 'c2', name: 'Beta edited', title_automatic: false }))
    await flushPromises()
    expect(wrapper.get('[data-testid=edit-conversation-title]').attributes('disabled')).toBeDefined()
    finishFirstSave(await json({ id: 'c1', name: 'Alpha edited', title_automatic: false }))
    await flushPromises()
    expect(wrapper.get('[data-testid=edit-conversation-title]').attributes('disabled')).toBeUndefined()
  })

  it('creates and lists a second automatically titled chat with the same direct user', async () => {
    const fetcher = vi.fn((url: string, init?: RequestInit) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true' && !init) return json([
        { id: 'c1', name: 'general', visibility: 'organisation', is_general: true },
        { id: 'c2', name: 'First topic', visibility: 'members', member_ids: ['u1', 'b1'], latest_sequence: 2 },
      ])
      if (url === '/api/organisations/o1/conversations' && init?.method === 'POST') return json({ id: 'c3', name: 'New Hector session', visibility: 'members', member_ids: ['u1', 'b1'], title_automatic: true }, 201)
      if (url === '/api/conversations/c1/messages?limit=100') return json([])
      if (url === '/api/conversations/c3/messages' && init?.method === 'POST') return json({ id: 'm1', conversation_id: 'c3', author_id: 'u1', author_name: 'Michael', author_kind: 'human', body: 'Second topic', created_at: '2026-08-24T09:00:01Z', sequence: 1 }, 201)
      if (url === '/api/organisations/o1/users') return json([
        { id: 'u1', name: 'Michael', kind: 'human', role: 'admin' },
        { id: 'b1', name: 'Hector', kind: 'bot', role: 'member' },
      ])
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: class { close() {} } } } })
    await flushPromises()

    await wrapper.get('[data-testid=new-direct-topic-b1]').trigger('click')
    await wrapper.get('[data-testid=composer] textarea').setValue('Second topic')
    await wrapper.get('[data-testid=composer]').trigger('submit')
    await flushPromises()

    const creationBody = JSON.parse(String(fetcher.mock.calls.find(([url, init]) => url === '/api/organisations/o1/conversations' && init?.method === 'POST')?.[1]?.body))
    expect(creationBody).toEqual({ name: 'New Hector session', visibility: 'members', member_ids: ['b1'], automatic_title: true, client_id: expect.any(String) })
    expect(wrapper.findAll('[data-testid=direct-contact-b1] .direct-topic .conversation-label').map(label => label.text())).toEqual(['First topic', 'Second topic'])
    expect(wrapper.get('h1').text()).toBe('Second topic')
  })

  it('starts a group topic without a dialog and creates it when the first message is sent', async () => {
    const fetcher = vi.fn((url: string, init?: RequestInit) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true' && !init) return json([
        { id: 'c1', name: 'general', visibility: 'organisation', is_general: true },
        { id: 'group1', name: 'Launch planning', visibility: 'members', member_ids: ['u1', 'b1', 'u2'], latest_sequence: 2 },
      ])
      if (url === '/api/organisations/o1/conversations' && init?.method === 'POST') return json({ id: 'group2', name: 'New Launch planning session', visibility: 'members', member_ids: ['u1', 'b1', 'u2'], title_automatic: true }, 201)
      if (url === '/api/conversations/c1/messages?limit=100' || url === '/api/conversations/group1/messages?limit=100&after_sequence=0') return json([])
      if (url === '/api/conversations/group2/messages' && init?.method === 'POST') return json({ id: 'm1', conversation_id: 'group2', author_id: 'u1', author_name: 'Michael', author_kind: 'human', body: 'Launch risks', created_at: '2026-08-24T09:00:01Z', sequence: 1 }, 201)
      if (url === '/api/organisations/o1/users') return json([
        { id: 'u1', name: 'Michael', kind: 'human', role: 'admin' },
        { id: 'b1', name: 'Hector', kind: 'bot', role: 'member' },
        { id: 'u2', name: 'Mary', kind: 'human', role: 'member' },
      ])
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: class { close() {} } } } })
    await flushPromises()

    await wrapper.get('[data-testid=group-conversations] .conversation-main').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid=new-group-topic-group1]').trigger('click')

    expect(wrapper.find('[data-testid=create-conversation]').exists()).toBe(false)
    expect(wrapper.get('h1').text()).toBe('# New Launch planning session')
    expect(fetcher.mock.calls.filter(([url, init]) => url === '/api/organisations/o1/conversations' && init?.method === 'POST')).toHaveLength(0)

    await wrapper.get('[data-testid=composer] textarea').setValue('Launch risks')
    await wrapper.get('[data-testid=composer]').trigger('submit')
    await flushPromises()

    const creationBody = JSON.parse(String(fetcher.mock.calls.find(([url, init]) => url === '/api/organisations/o1/conversations' && init?.method === 'POST')?.[1]?.body))
    expect(creationBody).toEqual({ name: 'New Launch planning session', visibility: 'members', member_ids: ['b1', 'u2'], automatic_title: true, client_id: expect.any(String) })
    expect(wrapper.get('h1').text()).toBe('# Launch risks')
  })

  it('shows each legacy direct topic and opens the selected conversation', async () => {
    const fetcher = vi.fn((url: string) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true') return json([
        { id: 'c1', name: 'general', visibility: 'organisation', is_general: true, latest_sequence: 0 },
        { id: 'direct-old', name: 'Hector legacy', visibility: 'members', member_ids: ['u1', 'b1'], latest_sequence: 0 },
        { id: 'direct-new', name: 'Another legacy title', visibility: 'members', member_ids: ['u1', 'b1'], latest_sequence: 0 },
      ])
      if (url === '/api/conversations/c1/messages?limit=100' || url === '/api/conversations/direct-new/messages?limit=100') return json([])
      if (url === '/api/organisations/o1/users') return json([
        { id: 'u1', name: 'Michael', kind: 'human', role: 'admin' },
        { id: 'b1', name: 'Hector', kind: 'bot', role: 'member' },
      ])
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: class { close() {} } } } })
    await flushPromises()

    const directTopics = wrapper.findAll('[data-testid=direct-conversations] .direct-topic')
    expect(directTopics).toHaveLength(2)
    expect(directTopics.map(topic => topic.get('.conversation-label').text()).sort()).toEqual(['Another legacy title', 'Hector legacy'])
    const latestTopic = directTopics.find(topic => topic.text().includes('Another legacy title'))!
    expect(latestTopic.attributes('aria-current')).toBeUndefined()
    await latestTopic.trigger('click')
    await flushPromises()

    expect(fetcher).toHaveBeenCalledWith('/api/conversations/direct-new/messages?limit=100', undefined)
    expect(wrapper.get('h1').text()).toBe('Another legacy title')
    expect(wrapper.get('header p').text()).toBe('With Hector')
    expect(latestTopic.attributes('aria-current')).toBe('page')
  })

  it('lazily creates and selects one direct conversation when a chatless user is clicked repeatedly', async () => {
    let finishCreation!: (response: Response) => void
    const pendingCreation = new Promise<Response>(resolve => { finishCreation = resolve })
    const fetcher = vi.fn((url: string, init?: RequestInit) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true' && !init) return json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true }])
      if (url === '/api/organisations/o1/direct-conversations/b1') return pendingCreation
      if (url === '/api/conversations/c1/messages?limit=100' || url === '/api/conversations/c2/messages?limit=100') return json([])
      if (url === '/api/organisations/o1/users') return json([
        { id: 'u1', name: 'Michael', kind: 'human', role: 'admin' },
        { id: 'b1', name: 'Hector', kind: 'bot', role: 'member' },
      ])
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: class { close() {} } } } })
    await flushPromises()

    const directUser = conversationButton(wrapper, 'Hector')
    await directUser.trigger('click')
    await directUser.trigger('click')

    const creationCalls = fetcher.mock.calls.filter(([url, init]) => url === '/api/organisations/o1/direct-conversations/b1' && init)
    expect(creationCalls).toHaveLength(1)
    expect(creationCalls[0]?.[1]).toEqual(expect.objectContaining({ method: 'POST' }))
    expect(creationCalls[0]?.[1]?.body).toBeUndefined()

    finishCreation(await json({ id: 'c2', name: 'Hector', visibility: 'members', member_ids: ['u1', 'b1'] }, 201))
    await flushPromises()
    expect(wrapper.get('h1').text()).toContain('Hector')
    expect(directUser.attributes('aria-current')).toBeUndefined()
    expect(wrapper.findAll('[data-testid=direct-contact-b1] [aria-current="page"]')).toHaveLength(1)
  })

  it('shows a direct-chat creation failure and lets the user retry', async () => {
    let creationAttempts = 0
    const fetcher = vi.fn((url: string, init?: RequestInit) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true' && !init) return json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true }])
      if (url === '/api/organisations/o1/direct-conversations/b1') {
        creationAttempts++
        return creationAttempts === 1
          ? json({ error: 'Could not start direct chat' }, 500)
          : json({ id: 'c2', name: 'internal-name', visibility: 'members', member_ids: ['u1', 'b1'] }, 201)
      }
      if (url === '/api/conversations/c1/messages?limit=100' || url === '/api/conversations/c2/messages?limit=100') return json([])
      if (url === '/api/organisations/o1/users') return json([
        { id: 'u1', name: 'Michael', kind: 'human', role: 'admin' },
        { id: 'b1', name: 'Hector', kind: 'bot', role: 'member' },
      ])
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: class { close() {} } } } })
    await flushPromises()

    await conversationButton(wrapper, 'Hector').trigger('click')
    await flushPromises()
    expect(wrapper.get('[role=alert]').text()).toContain('Could not start direct chat')

    await conversationButton(wrapper, 'Hector').trigger('click')
    await flushPromises()
    expect(creationAttempts).toBe(2)
    expect(wrapper.get('h1').text()).toBe('internal-name')
    expect(wrapper.get('header p').text()).toBe('With Hector')
  })

  it('does not let a delayed direct-chat response replace a newer conversation selection', async () => {
    let finishCreation!: (response: Response) => void
    const pendingCreation = new Promise<Response>(resolve => { finishCreation = resolve })
    const fetcher = vi.fn((url: string) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true') return json([
        { id: 'c1', name: 'general', visibility: 'organisation', is_general: true },
        { id: 'c2', name: 'planning', visibility: 'organisation' },
      ])
      if (url === '/api/organisations/o1/direct-conversations/b1') return pendingCreation
      if (url === '/api/conversations/c1/messages?limit=100' || url === '/api/conversations/c2/messages?limit=100') return json([])
      if (url === '/api/organisations/o1/users') return json([
        { id: 'u1', name: 'Michael', kind: 'human', role: 'admin' },
        { id: 'b1', name: 'Hector', kind: 'bot', role: 'member' },
      ])
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: class { close() {} } } } })
    await flushPromises()

    await conversationButton(wrapper, 'Hector').trigger('click')
    await conversationButton(wrapper, 'planning').trigger('click')
    await flushPromises()
    finishCreation(await json({ id: 'c3', name: 'direct:u1:b1', visibility: 'members', member_ids: ['u1', 'b1'] }))
    await flushPromises()

    expect(wrapper.get('h1').text()).toBe('# planning')
    expect(conversationButton(wrapper, 'Hector').exists()).toBe(true)
    expect(fetcher).not.toHaveBeenCalledWith('/api/conversations/c3/messages?limit=100', undefined)
  })

  it('does not show a delayed direct-chat failure after newer navigation', async () => {
    let finishCreation!: (response: Response) => void
    const pendingCreation = new Promise<Response>(resolve => { finishCreation = resolve })
    const fetcher = vi.fn((url: string) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true') return json([
        { id: 'c1', name: 'general', visibility: 'organisation', is_general: true },
        { id: 'c2', name: 'planning', visibility: 'organisation' },
      ])
      if (url === '/api/organisations/o1/direct-conversations/b1') return pendingCreation
      if (url === '/api/conversations/c1/messages?limit=100' || url === '/api/conversations/c2/messages?limit=100') return json([])
      if (url === '/api/organisations/o1/users') return json([
        { id: 'u1', name: 'Michael', kind: 'human', role: 'admin' },
        { id: 'b1', name: 'Hector', kind: 'bot', role: 'member' },
      ])
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: class { close() {} } } } })
    await flushPromises()

    await conversationButton(wrapper, 'Hector').trigger('click')
    await conversationButton(wrapper, 'planning').trigger('click')
    await flushPromises()
    finishCreation(await json({ error: 'Could not start direct chat' }, 500))
    await flushPromises()

    expect(wrapper.get('h1').text()).toBe('# planning')
    expect(conversationButton(wrapper, 'planning').attributes('aria-current')).toBe('page')
    expect(wrapper.find('[role=alert]').exists()).toBe(false)
  })

  it('lets an admin delete the selected conversation for everyone', async () => {
    const fetcher = vi.fn()
      .mockImplementationOnce(() => json({ id: 'u1', name: 'Michael', kind: 'human' }))
      .mockImplementationOnce(() => json([{ id: 'o1', name: 'Mainstay', role: 'admin' }]))
      .mockImplementationOnce(() => json([
        { id: 'c1', name: 'Roadmap', visibility: 'members', member_ids: ['u1', 'b1', 'u2'] },
        { id: 'c2', name: 'planning', visibility: 'members', member_ids: ['u1', 'b1', 'u2'] },
      ]))
      .mockImplementationOnce(() => json([]))
      .mockImplementationOnce(() => Promise.resolve(new Response(null, { status: 204 })))
      .mockImplementationOnce(() => json([]))
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true)
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: class { close() {} } } } })
    await flushPromises()

    await wrapper.get('[data-testid=conversation-actions-menu]').trigger('click')
    await wrapper.get('[data-testid=delete-conversation]').trigger('click')
    await flushPromises()

    expect(confirm).toHaveBeenCalledWith('Delete #Roadmap for everyone? This permanently removes all messages in this conversation.')
    expect(fetcher).toHaveBeenCalledWith('/api/organisations/o1/conversations/c1', expect.objectContaining({ method: 'DELETE' }))
    expect(wrapper.findAll('nav button').some(button => button.text().includes('Roadmap'))).toBe(false)
    expect(wrapper.get('h1').text()).toBe('# planning')
    confirm.mockRestore()
  })

  it('lets an admin delete selected conversations after one bulk confirmation', async () => {
    const conversations = [
      { id: 'c1', name: 'general', visibility: 'members', member_ids: ['u1', 'b1', 'u2'] },
      { id: 'c2', name: 'planning', visibility: 'members', member_ids: ['u1', 'b1', 'u2'] },
    ]
    const fetcher = vi.fn((url: string, init?: RequestInit) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true') return json(conversations)
      if (url === '/api/conversations/c1/messages?limit=100') return json([])
      if (url === '/api/organisations/o1/users') return json([])
      if (/^\/api\/organisations\/o1\/conversations\/c[12]$/.test(url) && init?.method === 'DELETE') return Promise.resolve(new Response(null, { status: 204 }))
      throw new Error(`Unexpected request: ${url}`)
    })
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true)
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: class { close() {} } } } })
    await flushPromises()

    const checkboxes = wrapper.findAll('[data-testid=conversation-checkbox]')
    await checkboxes[0].trigger('click')
    await checkboxes[1].trigger('click')
    await wrapper.get('[data-testid=bulk-delete-conversations]').trigger('click')
    await flushPromises()

    expect(confirm).toHaveBeenCalledOnce()
    expect(confirm).toHaveBeenCalledWith('Delete 2 conversations? This permanently removes their messages for everyone.')
    expect(fetcher).toHaveBeenCalledWith('/api/organisations/o1/conversations/c1', expect.objectContaining({ method: 'DELETE' }))
    expect(fetcher).toHaveBeenCalledWith('/api/organisations/o1/conversations/c2', expect.objectContaining({ method: 'DELETE' }))
    expect(wrapper.text()).toContain('No conversations')
    confirm.mockRestore()
  })

  it('keeps selected conversations ticked when bulk delete is cancelled', async () => {
    const fetcher = vi.fn((url: string) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true') return json([
        { id: 'c1', name: 'general', visibility: 'members', member_ids: ['u1', 'b1', 'u2'] },
        { id: 'c2', name: 'planning', visibility: 'members', member_ids: ['u1', 'b1', 'u2'] },
      ])
      if (url === '/api/conversations/c1/messages?limit=100') return json([])
      if (url === '/api/organisations/o1/users') return json([])
      throw new Error(`Unexpected request: ${url}`)
    })
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false)
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: class { close() {} } } } })
    await flushPromises()

    const checkboxes = wrapper.findAll('[data-testid=conversation-checkbox]')
    await checkboxes[0].trigger('click')
    await checkboxes[1].trigger('click')
    await wrapper.get('[data-testid=bulk-delete-conversations]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid=bulk-conversation-actions]').text()).toContain('2 selected')
    expect(checkboxes.every(checkbox => (checkbox.element as HTMLInputElement).checked)).toBe(true)
    confirm.mockRestore()
  })

  it('does not expose permanent deletion for a direct conversation', async () => {
    const fetcher = vi.fn((url: string) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true') return json([{ id: 'c1', name: 'Forecast review', visibility: 'members', member_ids: ['u1', 'b1'] }])
      if (url === '/api/conversations/c1/messages?limit=100') return json([])
      if (url === '/api/organisations/o1/users') return json([
        { id: 'u1', name: 'Michael', kind: 'human', role: 'admin' },
        { id: 'b1', name: 'Hector', kind: 'bot', role: 'member' },
      ])
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: class { close() {} } } } })
    await flushPromises()

    expect(wrapper.find('[data-testid=conversation-actions-menu]').exists()).toBe(false)
    expect(wrapper.find('[data-testid=delete-conversation]').exists()).toBe(false)
  })

  it('presents a one-member private conversation as a removed direct conversation', async () => {
    const fetcher = vi.fn((url: string) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true') return json([{ id: 'c1', name: 'direct:u1:b1', visibility: 'members', member_ids: ['u1'] }])
      if (url === '/api/conversations/c1/messages?limit=100') return json([{ id: 'm1', conversation_id: 'c1', author_id: 'b1', author_name: 'Former bot', author_kind: 'bot', body: 'Historical message', created_at: '2026-01-01T00:00:00Z', sequence: 1 }])
      if (url === '/api/organisations/o1/users') return json([{ id: 'u1', name: 'Michael', kind: 'human', role: 'admin' }])
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: class { close() {} } } } })
    await flushPromises()

    expect(wrapper.findAll('[data-testid=direct-conversations] .conversation-label').map(label => label.text())).toEqual(['Removed user'])
    expect(wrapper.findAll('[data-testid=group-conversations] > button')).toHaveLength(0)
    expect(wrapper.get('h1').text()).toBe('Removed user')
    expect(wrapper.text()).toContain('Historical message')
    expect(wrapper.get('[data-testid=composer] textarea').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid=composer] textarea').attributes('placeholder')).toBe('Conversation unavailable')
    expect(wrapper.find('[data-testid=conversation-actions-menu]').exists()).toBe(false)
    expect(wrapper.find('[data-testid=delete-conversation]').exists()).toBe(false)
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
      .mockImplementationOnce(() => json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true, read_sequence: 1, latest_sequence: 2 }]))
      .mockImplementationOnce(() => json([message]))
      .mockImplementationOnce(() => json([]))
      .mockImplementationOnce(() => json({ error: 'Read sync failed' }, 500))
      .mockImplementationOnce(() => json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true, read_sequence: 1, latest_sequence: 2 }]))
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
      .mockImplementationOnce(() => json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true, read_sequence: 1, latest_sequence: 2 }]))
      .mockImplementationOnce(() => json([message]))
      .mockImplementationOnce(() => json([]))
      .mockImplementationOnce(() => json({ sequence: 2 }))
      .mockImplementationOnce(() => json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true, read_sequence: 1, latest_sequence: 2 }]))
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

  it('keeps a realtime automatic title and activity when an older refresh finishes later', async () => {
    class EventSocket {
      static instance: EventSocket
      onopen: (() => void) | null = null
      onmessage: ((event: MessageEvent) => void) | null = null
      onclose: (() => void) | null = null
      close() {}
      constructor() { EventSocket.instance = this }
    }
    let finishRefresh!: (response: Response) => void
    const pendingRefresh = new Promise<Response>(resolve => { finishRefresh = resolve })
    let conversationRequests = 0
    const placeholder = { id: 'c1', name: 'New Hector session', visibility: 'members', member_ids: ['u1', 'b1'], latest_sequence: 0, activity_at: '2026-08-24T08:00:00Z', title_automatic: true }
    const fetcher = vi.fn((url: string) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true') return ++conversationRequests === 1 ? json([placeholder]) : pendingRefresh
      if (url === '/api/conversations/c1/messages?limit=100') return json([])
      if (url === '/api/organisations/o1/users') return json([{ id: 'u1', name: 'Michael', kind: 'human', role: 'admin' }, { id: 'b1', name: 'Hector', kind: 'bot', role: 'member' }])
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: EventSocket } } })
    await flushPromises()

    EventSocket.instance.onopen?.()
    EventSocket.instance.onmessage?.({ data: JSON.stringify({ type: 'message.created', sequence: 1, payload: { id: 'm1', conversation_id: 'c1', author_id: 'b1', author_name: 'Hector', author_kind: 'bot', body: 'Realtime title', created_at: '2026-08-24T09:00:00Z', sequence: 1 } }) } as MessageEvent)
    finishRefresh(await json([placeholder]))
    await flushPromises()

    expect(wrapper.get('h1').text()).toBe('Realtime title')
    expect(wrapper.get('[data-testid=direct-contact-b1] .direct-topic .conversation-label').text()).toBe('Realtime title')
  })

  it('keeps a completed manual title when an older refresh finishes later', async () => {
    class EventSocket {
      static instance: EventSocket
      onopen: (() => void) | null = null
      onmessage: ((event: MessageEvent) => void) | null = null
      onclose: (() => void) | null = null
      close() {}
      constructor() { EventSocket.instance = this }
    }
    let finishRefresh!: (response: Response) => void
    const pendingRefresh = new Promise<Response>(resolve => { finishRefresh = resolve })
    let conversationRequests = 0
    const placeholder = { id: 'c1', name: 'New Hector session', visibility: 'members', member_ids: ['u1', 'b1'], latest_sequence: 0, activity_at: '2026-08-24T08:00:00Z', title_automatic: true }
    const fetcher = vi.fn((url: string, init?: RequestInit) => {
      if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
      if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
      if (url === '/api/organisations/o1/conversations?include_archived=true') return ++conversationRequests === 1 ? json([placeholder]) : pendingRefresh
      if (url === '/api/conversations/c1/messages?limit=100') return json([])
      if (url === '/api/conversations/c1/title' && init?.method === 'PUT') return json({ id: 'c1', name: 'Manual title', title_automatic: false })
      if (url === '/api/organisations/o1/users') return json([{ id: 'u1', name: 'Michael', kind: 'human', role: 'admin' }, { id: 'b1', name: 'Hector', kind: 'bot', role: 'member' }])
      throw new Error(`Unexpected request: ${url}`)
    })
    const wrapper = mount(App, { global: { provide: { fetcher, socketFactory: EventSocket } } })
    await flushPromises()

    EventSocket.instance.onopen?.()
    await wrapper.get('[data-testid=edit-conversation-title]').trigger('click')
    await wrapper.get('[data-testid=conversation-title-input]').setValue('Manual title')
    await wrapper.get('[data-testid=conversation-title-form]').trigger('submit')
    await flushPromises()
    finishRefresh(await json([placeholder]))
    await flushPromises()

    expect(wrapper.get('h1').text()).toBe('Manual title')
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
        { id: 'c1', name: 'general', visibility: 'organisation', is_general: true },
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
      { id: 'c1', name: 'general', visibility: 'organisation', is_general: true },
      { id: 'c2', name: 'planning', visibility: 'organisation' },
    ]))
    await flushPromises()

    expect(wrapper.findAll('nav button').some(button => button.text().includes('general'))).toBe(false)
    expect(wrapper.get('h1').text()).toBe('# planning')
    expect(fetcher).toHaveBeenCalledWith('/api/organisations/o1/conversations?include_archived=true', undefined)
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
        { id: 'c1', name: 'general', visibility: 'organisation', is_general: true },
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
        { id: 'c1', name: 'general', visibility: 'organisation', is_general: true },
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
        { id: 'c1', name: 'general', visibility: 'organisation', is_general: true },
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
      .mockImplementationOnce(() => json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true }]))
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
      .mockImplementationOnce(() => json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true }]))
    const wrapper = mount(App, { global: { provide: { fetcher: withInitialUsers(fetcher), socketFactory: class { close() {} } } } })
    await flushPromises()
    await wrapper.get('[data-testid=organisation-settings]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid=remove-bot-b1]').trigger('click')
    expect(wrapper.get('[data-testid=confirm-remove-b1]').text()).toContain('Remove Hector')
    await wrapper.get('[data-testid=confirm-remove-b1]').trigger('click')
    await flushPromises()
    expect(fetcher).toHaveBeenCalledWith('/api/organisations/o1/bots/b1', expect.objectContaining({ method: 'DELETE' }))
    expect(fetcher.mock.calls.filter(([url, init]) => url === '/api/organisations/o1/conversations?include_archived=true' && !init)).toHaveLength(2)
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
  const button = wrapper.findAll('.sidebar-navigation button').find(candidate => candidate.text().toLocaleLowerCase().includes(name.toLocaleLowerCase()))
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
    .mockImplementationOnce(() => json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true }]))
    .mockImplementationOnce(() => json([]))
}

function replyRetryFetcher(messages: unknown, posts: Array<Record<string, string>>) {
  return vi.fn((url: string, init?: RequestInit) => {
    if (url === '/api/me') return json({ id: 'u1', name: 'Michael', kind: 'human' })
    if (url === '/api/organisations') return json([{ id: 'o1', name: 'Mainstay', role: 'admin' }])
    if (url === '/api/organisations/o1/conversations?include_archived=true') return json([{ id: 'c1', name: 'general', visibility: 'organisation', is_general: true }])
    if (url === '/api/conversations/c1/messages?limit=100' && !init) return json(Array.isArray(messages) ? messages : [messages])
    if (url === '/api/organisations/o1/users') return json([])
    if (url === '/api/conversations/c1/messages' && init?.method === 'POST') {
      posts.push(JSON.parse(String(init.body)) as Record<string, string>)
      if (posts.length === 1) return Promise.reject(new Error('response lost'))
      return json({ id: 'm3', conversation_id: 'c1', author_id: 'u1', author_name: 'Michael', author_kind: 'human', body: posts[1].body, created_at: '2026-01-01T00:00:02Z', sequence: 3 }, 200)
    }
    if (url === '/api/conversations/c1/read') return json({ sequence: 3 })
    throw new Error(`Unexpected request: ${url}`)
  })
}

function messagesForReadTest() {
  return [
    { id: 'm1', conversation_id: 'c1', author_id: 'u1', author_name: 'Michael', author_kind: 'human', body: 'Read', created_at: '2026-01-01T00:00:00Z', sequence: 1 },
    { id: 'm2', conversation_id: 'c1', author_id: 'b1', author_name: 'Hector', author_kind: 'bot', body: 'Unread', created_at: '2026-01-01T00:00:01Z', sequence: 2 },
  ]
}
