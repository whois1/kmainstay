import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ConversationView from './ConversationView.vue'
import type { Conversation, Message, User } from '../types'

const conversation: Conversation = { id: 'conversation', name: 'general', visibility: 'organisation', read_sequence: 1 }
const messages: Message[] = [
  { id: 'first', conversation_id: 'conversation', author_id: 'reader', author_name: 'Reader', author_kind: 'human', body: 'Read', created_at: '2026-01-01T00:00:00Z', sequence: 1 },
  { id: 'second', conversation_id: 'conversation', author_id: 'writer', author_name: 'Writer', author_kind: 'human', body: 'Unread', created_at: '2026-01-01T00:00:01Z', sequence: 2 },
]
const users: User[] = [
  { id: 'reader', name: 'Reader', kind: 'human', role: 'admin' },
  { id: 'hector', name: 'Hector', kind: 'bot', role: 'member' },
  { id: 'mary', name: 'Mary Jane', kind: 'human', role: 'member' },
]

describe('ConversationView', () => {
  it('renders authorised image attachments inline', () => {
    const imageMessage = {
      ...messages[0],
      body: '',
      attachments: [{ id: 'attachment', media_type: 'image/png' as const, byte_size: 12, width: 2, height: 1, original_filename: 'red.png', created_at: '2026-01-01T00:00:00Z', content_url: '/api/attachments/attachment/content' }],
    }
    const wrapper = mount(ConversationView, { props: { selected: conversation, messages: [imageMessage], composer: '', busy: false, error: '', canDelete: false } })
    const image = wrapper.get('[data-testid=message-image]')
    expect(image.attributes('src')).toBe('/api/attachments/attachment/content')
    expect(image.attributes('alt')).toBe('red.png')
    expect(image.attributes('loading')).toBe('lazy')
    expect(wrapper.find('.markdown').exists()).toBe(false)
  })

  it('selects and removes one JPEG or PNG from the composer', async () => {
    const wrapper = mount(ConversationView, { props: { selected: conversation, messages: [], composer: '', busy: false, error: '', canDelete: false } })
    const file = new File([new Uint8Array([1, 2, 3])], 'photo.png', { type: 'image/png' })
    const input = wrapper.get('input[type=file]')
    Object.defineProperty(input.element, 'files', { configurable: true, value: [file] })
    await input.trigger('change')
    expect(wrapper.emitted('update:image')?.at(-1)).toEqual([file])
    expect(wrapper.get('[data-testid=selected-image]').text()).toContain('photo.png')
    await wrapper.get('[data-testid=remove-image]').trigger('click')
    expect(wrapper.emitted('update:image')?.at(-1)).toEqual([null])
  })

  it('clears an invalid image error when changing conversations', async () => {
    const wrapper = mount(ConversationView, { props: { selected: conversation, messages: [], composer: '', busy: false, error: '', canDelete: false } })
    const invalid = new File([new Uint8Array([1])], 'document.svg', { type: 'image/svg+xml' })
    const input = wrapper.get('input[type=file]')
    Object.defineProperty(input.element, 'files', { configurable: true, value: [invalid] })
    await input.trigger('change')
    expect(wrapper.find('[role=alert]').exists()).toBe(true)
    await wrapper.setProps({ selected: { ...conversation, id: 'other' } })
    expect(wrapper.find('[role=alert]').exists()).toBe(false)
  })

  it('filters mention suggestions and provides accessible keyboard selection', async () => {
    const wrapper = mount(ConversationView, { props: { selected: conversation, messages: [], composer: '', busy: false, error: '', canDelete: false, users, currentUserID: 'reader' } })
    const textarea = wrapper.get('textarea')
    await textarea.setValue('Ask @ma')
    await textarea.trigger('input')

    const listbox = wrapper.get('[role=listbox]')
    expect(listbox.attributes('aria-label')).toBe('Mention a person or bot')
    expect(wrapper.findAll('[role=option] span').map(option => option.text())).toEqual(['Mary Jane'])
    expect(wrapper.text()).not.toContain('Readerhuman')
    expect(textarea.attributes('role')).toBe('combobox')
    expect(textarea.attributes('aria-activedescendant')).toBe(wrapper.get('[role=option]').attributes('id'))

    await textarea.trigger('keydown', { key: 'ArrowDown' })
    expect(wrapper.get('[role=option]').attributes('aria-selected')).toBe('true')
    await textarea.trigger('keydown', { key: 'Enter' })
    expect(wrapper.emitted('update:composer')?.at(-1)).toEqual(['Ask @Mary Jane '])
  })

  it('keeps Shift+Enter for multiline Markdown instead of sending', async () => {
    const wrapper = mount(ConversationView, { props: { selected: conversation, messages: [], composer: 'First line', busy: false, error: '', canDelete: false, users, currentUserID: 'reader' } })
    await wrapper.get('textarea').trigger('keydown', { key: 'Enter', shiftKey: true })
    expect(wrapper.emitted('sendMessage')).toBeUndefined()
  })

  it('supports click selection and preserves surrounding text and caret', async () => {
    const wrapper = mount(ConversationView, { props: { selected: conversation, messages: [], composer: 'Before @he after', busy: false, error: '', canDelete: false, users, currentUserID: 'reader' } })
    const textarea = wrapper.get('textarea').element as HTMLTextAreaElement
    textarea.focus()
    textarea.setSelectionRange(10, 10)
    await wrapper.get('textarea').trigger('input')
    await wrapper.get('[role=option]').trigger('click')
    await Promise.resolve()

    expect(wrapper.emitted('update:composer')?.at(-1)).toEqual(['Before @Hector  after'])
    expect(textarea.selectionStart).toBe('Before @Hector '.length)
  })

  it('closes the picker with Escape and when the conversation changes', async () => {
    const wrapper = mount(ConversationView, { props: { selected: conversation, messages: [], composer: '@', busy: false, error: '', canDelete: false, users, currentUserID: 'reader' } })
    await wrapper.get('textarea').trigger('input')
    expect(wrapper.find('[role=listbox]').exists()).toBe(true)
    await wrapper.get('textarea').trigger('keydown', { key: 'Escape' })
    expect(wrapper.find('[role=listbox]').exists()).toBe(false)
    await wrapper.get('textarea').trigger('input')
    await wrapper.setProps({ selected: { ...conversation, id: 'other' } })
    expect(wrapper.find('[role=listbox]').exists()).toBe(false)
  })

  it('explains automatic direct bot replies and mention-required routing', () => {
    const direct = mount(ConversationView, { props: { selected: { ...conversation, visibility: 'members', member_ids: ['reader', 'hector'] }, messages: [], composer: '', busy: false, error: '', canDelete: false, users, currentUserID: 'reader' } })
    expect(direct.get('[data-testid=bot-guidance]').text()).toContain('responds automatically')
    const group = mount(ConversationView, { props: { selected: { ...conversation, member_ids: ['reader', 'hector', 'mary'] }, messages: [], composer: '', busy: false, error: '', canDelete: false, users, currentUserID: 'reader' } })
    expect(group.get('[data-testid=bot-guidance]').text()).toContain('@mention')
  })

  it('uses the other user name for direct-chat titles and composer wording but keeps group names', () => {
    const direct = mount(ConversationView, { props: { selected: { ...conversation, name: 'legacy-internal-name', visibility: 'members', member_ids: ['reader', 'hector'] }, messages: [], composer: '', busy: false, error: '', canDelete: false, users, currentUserID: 'reader' } })
    expect(direct.get('h1').text()).toBe('Hector')
    expect(direct.get('textarea').attributes('placeholder')).toBe('Message Hector')

    const group = mount(ConversationView, { props: { selected: { ...conversation, name: 'Planning', visibility: 'members', member_ids: ['reader', 'hector', 'mary'] }, messages: [], composer: '', busy: false, error: '', canDelete: false, users, currentUserID: 'reader' } })
    expect(group.get('h1').text()).toBe('# Planning')
    expect(group.get('textarea').attributes('placeholder')).toBe('Message #Planning')
  })

  it('renders one new-messages divider immediately before the first unread message', async () => {
    const wrapper = mount(ConversationView, { props: { selected: conversation, messages, composer: '', busy: false, error: '', canDelete: false } })

    const divider = wrapper.get('[data-testid=new-messages-divider]')
    expect(wrapper.findAll('[data-testid=new-messages-divider]')).toHaveLength(1)
    expect(divider.text()).toBe('New messages')
    expect(divider.attributes('role')).toBe('separator')
    expect(divider.attributes('aria-label')).toBe('New messages')
    expect(divider.element.nextElementSibling?.getAttribute('data-message-id')).toBe('second')
  })

  it('positions the initial viewport at the first unread message', async () => {
	const scrollIntoView = vi.fn()
	HTMLElement.prototype.scrollIntoView = scrollIntoView
	mount(ConversationView, { props: { selected: conversation, messages, composer: '', busy: false, error: '', canDelete: false } })
	await Promise.resolve()

	expect(scrollIntoView).toHaveBeenCalledTimes(1)
	expect(scrollIntoView.mock.instances[0]?.getAttribute('data-testid')).toBe('new-messages-divider')
  })

  it('shows the jump control when the reader scrolls away from the bottom', async () => {
    const wrapper = mount(ConversationView, { props: { selected: { ...conversation, read_sequence: 2 }, messages, composer: '', busy: false, error: '', canDelete: false } })
    await Promise.resolve()
    const messageList = wrapper.get('.message-list').element
    Object.defineProperties(messageList, {
      scrollHeight: { configurable: true, value: 1000 },
      clientHeight: { configurable: true, value: 300 },
      scrollTop: { configurable: true, writable: true, value: 100 },
    })

    await wrapper.get('.message-list').trigger('scroll')

    expect(wrapper.get('[data-testid=jump-to-bottom]').attributes('aria-label')).toBe('Jump to latest message')
  })

  it('preserves a scrolled-up position and shows the jump control for a live message', async () => {
	const wrapper = mount(ConversationView, { props: { selected: { ...conversation, read_sequence: 2 }, messages, composer: '', busy: false, error: '', canDelete: false } })
	await Promise.resolve()
	const messageList = wrapper.get('.message-list').element
	Object.defineProperties(messageList, {
	  scrollHeight: { configurable: true, value: 1000 },
	  clientHeight: { configurable: true, value: 300 },
	  scrollTop: { configurable: true, writable: true, value: 100 },
	})
	await wrapper.setProps({ messages: [...messages, { ...messages[1], id: 'live', body: 'Live', sequence: 3 }] })

	expect(messageList.scrollTop).toBe(100)
	expect(wrapper.get('[data-testid=jump-to-bottom]').attributes('aria-label')).toBe('Jump to latest message')
  })

  it('shows no divider when there are no unread messages', () => {
	const wrapper = mount(ConversationView, { props: { selected: { ...conversation, read_sequence: 2 }, messages, composer: '', busy: false, error: '', canDelete: false } })
	expect(wrapper.find('[data-testid=new-messages-divider]').exists()).toBe(false)
  })

  it('requests the latest page before jumping across an unloaded gap', async () => {
    const wrapper = mount(ConversationView, { props: { selected: conversation, messages, composer: '', busy: false, error: '', canDelete: false, hasNewerMessages: true, jumpToLatestVersion: 0 } })
    await Promise.resolve()
    const messageList = wrapper.get('.message-list').element
    const scrollTo = vi.fn()
    Object.defineProperties(messageList, {
      scrollHeight: { configurable: true, value: 1000 }, clientHeight: { configurable: true, value: 300 },
      scrollTop: { configurable: true, writable: true, value: 100 }, scrollTo: { configurable: true, value: scrollTo },
    })

    await wrapper.get('[data-testid=jump-to-bottom]').trigger('click')
    expect(wrapper.emitted('jumpToLatest')).toEqual([[]])
    expect(scrollTo).not.toHaveBeenCalled()

    await wrapper.setProps({ hasNewerMessages: false, jumpToLatestVersion: 1, messages: [...messages, { ...messages[1], id: 'latest', sequence: 3 }] })
    await Promise.resolve()
    expect(scrollTo).toHaveBeenCalledWith({ top: 1000 })
    expect(wrapper.emitted('readThrough')?.at(-1)).toEqual([3])
  })

  it('jumps to latest, hides the control, and emits read-through', async () => {
	const wrapper = mount(ConversationView, { props: { selected: { ...conversation, read_sequence: 2 }, messages, composer: '', busy: false, error: '', canDelete: false } })
	await Promise.resolve()
	const messageList = wrapper.get('.message-list').element
	const scrollTo = vi.fn()
	Object.defineProperties(messageList, {
	  scrollHeight: { configurable: true, value: 1000 }, clientHeight: { configurable: true, value: 300 },
	  scrollTop: { configurable: true, writable: true, value: 100 }, scrollTo: { configurable: true, value: scrollTo },
	})
	await wrapper.setProps({ messages: [...messages, { ...messages[1], id: 'live', sequence: 3 }] })
	await wrapper.get('[data-testid=jump-to-bottom]').trigger('click')

	expect(scrollTo).toHaveBeenCalledWith({ top: 1000 })
	expect(wrapper.find('[data-testid=jump-to-bottom]').exists()).toBe(false)
	expect(wrapper.emitted('readThrough')?.at(-1)).toEqual([3])
  })

  it('keeps following live messages when already near bottom', async () => {
	const wrapper = mount(ConversationView, { props: { selected: { ...conversation, read_sequence: 2 }, messages, composer: '', busy: false, error: '', canDelete: false } })
	await Promise.resolve()
	const messageList = wrapper.get('.message-list').element
	const scrollTo = vi.fn()
	Object.defineProperties(messageList, {
	  scrollHeight: { configurable: true, value: 1000 }, clientHeight: { configurable: true, value: 300 },
	  scrollTop: { configurable: true, writable: true, value: 680 }, scrollTo: { configurable: true, value: scrollTo },
	})
	await wrapper.setProps({ messages: [...messages, { ...messages[1], id: 'live', sequence: 3 }] })
	await Promise.resolve()

	expect(scrollTo).toHaveBeenCalledWith({ top: 1000 })
	expect(wrapper.find('[data-testid=jump-to-bottom]').exists()).toBe(false)
	expect(wrapper.emitted('readThrough')?.at(-1)).toEqual([3])
  })
})
