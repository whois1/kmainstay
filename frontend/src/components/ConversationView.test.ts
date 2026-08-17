import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ConversationView from './ConversationView.vue'
import type { Conversation, Message } from '../types'

const conversation: Conversation = { id: 'conversation', name: 'general', visibility: 'organisation', read_sequence: 1 }
const messages: Message[] = [
  { id: 'first', conversation_id: 'conversation', author_id: 'reader', author_name: 'Reader', author_kind: 'human', body: 'Read', created_at: '2026-01-01T00:00:00Z', sequence: 1 },
  { id: 'second', conversation_id: 'conversation', author_id: 'writer', author_name: 'Writer', author_kind: 'human', body: 'Unread', created_at: '2026-01-01T00:00:01Z', sequence: 2 },
]

describe('ConversationView', () => {
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
