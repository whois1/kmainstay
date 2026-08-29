import { readFileSync } from 'node:fs'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import WorkspaceSidebar from './WorkspaceSidebar.vue'
import type { Conversation, User } from '../types'

const styles = readFileSync('frontend/src/style.css', 'utf8')

const conversations: Conversation[] = [
  { id: 'pinned-old', name: 'General', visibility: 'organisation', latest_sequence: 2 },
  { id: 'pinned-new', name: 'Announcements', visibility: 'organisation', latest_sequence: 12 },
  { id: 'direct-old', name: 'direct:direct-old', visibility: 'members', member_ids: ['michael', 'hector'], latest_sequence: 4 },
  { id: 'direct-hector-new', name: 'K-Mainstay code', visibility: 'members', member_ids: ['michael', 'hector'], latest_sequence: 11 },
  { id: 'direct-new', name: 'Forecast review', visibility: 'members', member_ids: ['michael', 'mary'], latest_sequence: 10 },
  { id: 'group-old', name: 'Planning', visibility: 'members', member_ids: ['michael', 'hector', 'mary'], latest_sequence: 3 },
  { id: 'group-new', name: 'Launch', visibility: 'members', member_ids: ['michael', 'hector', 'mary'], latest_sequence: 8 },
]

const users: User[] = [
  { id: 'michael', name: 'Michael', kind: 'human', role: 'admin' },
  { id: 'hector', name: 'Hector', kind: 'bot', role: 'member' },
  { id: 'mary', name: 'Mary', kind: 'human', role: 'member' },
  { id: 'zoe', name: 'Zoe', kind: 'human', role: 'member' },
  { id: 'alfred', name: 'Alfred', kind: 'bot', role: 'member' },
]

describe('WorkspaceSidebar', () => {
  it('keeps archived conversations out of active groups and exposes them separately', () => {
    const wrapper = mount(WorkspaceSidebar, {
      props: {
        organisation: { id: 'organisation', name: 'Mainstay', role: 'admin' },
        principal: { id: 'michael', name: 'Michael', kind: 'human' },
        conversations: [
          { id: 'active', name: 'Active', visibility: 'organisation' },
          { id: 'archived', name: 'Done', visibility: 'organisation', archived: true },
        ],
        users,
        selected: null,
        settingsActive: false,
      },
    })

    expect(wrapper.get('[data-testid=pinned-conversations]').text()).toContain('Active')
    expect(wrapper.get('[data-testid=pinned-conversations]').text()).not.toContain('Done')
    expect(wrapper.get('[data-testid=archived-conversations]').text()).toContain('Done')
  })

  it('keeps Everyone out of bulk selection', () => {
    const wrapper = mount(WorkspaceSidebar, {
      props: {
        organisation: { id: 'organisation', name: 'Mainstay', role: 'admin' },
        principal: { id: 'michael', name: 'Michael', kind: 'human' },
        conversations: [{ id: 'everyone', name: 'Everyone', visibility: 'organisation', is_everyone: true }],
        users,
        selected: null,
        settingsActive: false,
      },
    })

    expect(wrapper.get('[data-testid=pinned-conversations]').text()).toContain('Everyone')
    expect(wrapper.find('[data-testid=conversation-checkbox]').exists()).toBe(false)
  })

  it('puts a newly created empty topic first by latest activity', () => {
    const wrapper = mount(WorkspaceSidebar, {
      props: {
        organisation: { id: 'organisation', name: 'Mainstay', role: 'admin' },
        principal: { id: 'michael', name: 'Michael', kind: 'human' },
        conversations: [
          { id: 'older-active', name: 'Older active', visibility: 'members', member_ids: ['michael', 'hector'], latest_sequence: 20, activity_at: '2026-08-24T08:00:00Z' },
          { id: 'new-empty', name: 'New Hector session', visibility: 'members', member_ids: ['michael', 'hector'], latest_sequence: 0, activity_at: '2026-08-24T09:00:00Z' },
          { id: 'mary-active', name: 'Mary active', visibility: 'members', member_ids: ['michael', 'mary'], latest_sequence: 30, activity_at: '2026-08-24T08:30:00Z' },
        ],
        users,
        selected: null,
        settingsActive: false,
      },
    })

    expect(wrapper.findAll('[data-testid=direct-contact-hector] .direct-topic .conversation-label').map(label => label.text())).toEqual([
      'New Hector session',
      'Older active',
    ])
    expect(wrapper.findAll('[data-testid=direct-conversations] .direct-contact-heading .conversation-label').map(label => label.text()).slice(0, 2)).toEqual(['Hector', 'Mary'])
  })

  it('groups conversations by purpose and orders each section by latest message', () => {
    const wrapper = mount(WorkspaceSidebar, {
      props: {
        organisation: { id: 'organisation', name: 'Mainstay', role: 'admin' },
        principal: { id: 'michael', name: 'Michael', kind: 'human' },
        conversations,
        users,
        selected: null,
        settingsActive: false,
      },
    })

    const createButton = wrapper.get('[data-testid=new-conversation]')
    expect(createButton.attributes('aria-label')).toBe('New group chat')
    expect(createButton.element.closest('section')).toBeNull()
    expect(wrapper.findAll('.sidebar-navigation h2').map(heading => heading.attributes('aria-label'))).toEqual(['Pinned', 'Direct messages', 'Group chats'])
    expect(wrapper.findAll('[data-testid=pinned-conversations] button').map(button => button.text())).toEqual(['#Announcements', '#General'])
    expect(wrapper.findAll('[data-testid=direct-conversations] .direct-contact > .direct-contact-heading .conversation-label').map(label => label.text())).toEqual(['Hector', 'Mary', 'Alfred', 'Zoe'])
    expect(wrapper.findAll('[data-testid=direct-contact-hector] .direct-topic .conversation-label').map(label => label.text())).toEqual(['K-Mainstay code', 'General'])
    expect(wrapper.findAll('[data-testid=direct-contact-mary] .direct-topic .conversation-label').map(label => label.text())).toEqual(['Forecast review'])
    expect(wrapper.get('[data-testid="group-thread-hector-mary"] .group-thread-heading .conversation-label').text()).toBe('Hector, Mary')
    expect(wrapper.findAll('[data-testid="group-thread-hector-mary"] .group-topic .conversation-label').map(label => label.text())).toEqual(['Launch', 'Planning'])
  })

  it('opens the latest group topic and starts another with the identical member set', async () => {
    const wrapper = mount(WorkspaceSidebar, {
      props: {
        organisation: { id: 'organisation', name: 'Mainstay', role: 'admin' },
        principal: { id: 'michael', name: 'Michael', kind: 'human' },
        conversations,
        users,
        selected: null,
        settingsActive: false,
      },
    })

    await wrapper.get('[data-testid="group-thread-hector-mary"] .group-thread-button').trigger('click')
    expect(wrapper.emitted('selectConversation')?.[0]).toEqual([conversations[6]])

    await wrapper.get('[data-testid="new-group-topic-hector-mary"]').trigger('click')
    expect(wrapper.emitted('newTopic')?.[0]).toEqual([conversations[6]])
  })

  it('exposes the current chat programmatically unless settings are active', async () => {
    const wrapper = mount(WorkspaceSidebar, {
      props: {
        organisation: { id: 'organisation', name: 'Mainstay', role: 'admin' },
        principal: { id: 'michael', name: 'Michael', kind: 'human' },
        conversations,
        users,
        selected: conversations[2],
        settingsActive: false,
      },
    })

    const directButton = wrapper.get('[data-testid=direct-contact-hector] .selectable-conversation:last-child .direct-topic')
    expect(directButton.attributes('aria-current')).toBe('page')
    expect(wrapper.findAll('[data-testid=direct-contact-hector] [aria-current="page"]')).toHaveLength(1)
    await wrapper.setProps({ settingsActive: true })
    expect(directButton.attributes('aria-current')).toBeUndefined()
  })

  it('selects a contiguous conversation range with Shift-click', async () => {
    const wrapper = mount(WorkspaceSidebar, {
      props: {
        organisation: { id: 'organisation', name: 'Mainstay', role: 'admin' },
        principal: { id: 'michael', name: 'Michael', kind: 'human' },
        conversations,
        users,
        selected: null,
        settingsActive: false,
      },
    })

    const checkboxes = wrapper.findAll('[data-testid=conversation-checkbox]')
    expect(checkboxes).toHaveLength(conversations.length)
    await checkboxes[0].trigger('click')
    await checkboxes[2].trigger('click', { shiftKey: true })

    expect(checkboxes.map(checkbox => (checkbox.element as HTMLInputElement).checked)).toEqual([
      true, true, true, false, false, false, false,
    ])
    const actionBar = wrapper.get('[data-testid=bulk-conversation-actions]')
    expect(actionBar.text()).toContain('3 selected')
    expect(actionBar.attributes('aria-live')).toBe('polite')
  })

  it('identifies the direct contact in checkbox names', () => {
    const wrapper = mount(WorkspaceSidebar, {
      props: {
        organisation: { id: 'organisation', name: 'Mainstay', role: 'admin' },
        principal: { id: 'michael', name: 'Michael', kind: 'human' },
        conversations,
        users,
        selected: null,
        settingsActive: false,
      },
    })

    expect(wrapper.findAll('[data-testid=direct-contact-hector] [data-testid=conversation-checkbox]').map(checkbox => checkbox.attributes('aria-label'))).toEqual([
      'Select K-Mainstay code with Hector',
      'Select General with Hector',
    ])
  })

  it('emits the selected active conversations for bulk archive', async () => {
    const wrapper = mount(WorkspaceSidebar, {
      props: {
        organisation: { id: 'organisation', name: 'Mainstay', role: 'admin' },
        principal: { id: 'michael', name: 'Michael', kind: 'human' },
        conversations,
        users,
        selected: null,
        settingsActive: false,
      },
    })

    const checkboxes = wrapper.findAll('[data-testid=conversation-checkbox]')
    await checkboxes[0].trigger('click')
    await checkboxes[1].trigger('click')
    await wrapper.get('[data-testid=bulk-archive-conversations]').trigger('click')

    expect(wrapper.emitted('archiveConversations')?.[0]).toEqual([[conversations[1], conversations[0]]])
  })

  it('keeps conversation selectors compact beside their labels', () => {
    const wrapper = mount(WorkspaceSidebar, {
      props: {
        organisation: { id: 'organisation', name: 'Mainstay', role: 'admin' },
        principal: { id: 'michael', name: 'Michael', kind: 'human' },
        conversations,
        users,
        selected: null,
        settingsActive: false,
      },
    })

    expect(wrapper.get('[data-testid=conversation-checkbox]').classes()).toContain('conversation-selector')
    expect(cssRule('.conversation-selector')).toContain('width: 16px')
    expect(cssRule('.conversation-selector')).toContain('min-height: 16px')
    expect(cssRule('.sidebar-navigation')).toContain('scrollbar-width: thin')
  })

  it('keeps a user-created direct-prefixed topic name', () => {
    const wrapper = mount(WorkspaceSidebar, {
      props: {
        organisation: { id: 'organisation', name: 'Mainstay', role: 'admin' },
        principal: { id: 'michael', name: 'Michael', kind: 'human' },
        conversations: [{ id: 'custom-topic', name: 'direct:roadmap', visibility: 'members', member_ids: ['michael', 'hector'] }],
        users,
        selected: null,
        settingsActive: false,
      },
    })

    expect(wrapper.get('[data-testid=direct-contact-hector] .direct-topic .conversation-label').text()).toBe('direct:roadmap')
  })

  it('opens a direct topic and starts another with the same user', async () => {
    const wrapper = mount(WorkspaceSidebar, {
      props: {
        organisation: { id: 'organisation', name: 'Mainstay', role: 'admin' },
        principal: { id: 'michael', name: 'Michael', kind: 'human' },
        conversations,
        users,
        selected: null,
        settingsActive: false,
      },
    })

    await wrapper.get('[data-testid=direct-contact-hector] .direct-topic').trigger('click')
    expect(wrapper.emitted('selectConversation')?.[0]).toEqual([conversations[3]])

    await wrapper.get('[data-testid=new-direct-topic-hector]').trigger('click')
    expect(wrapper.emitted('newDirectTopic')?.[0]).toEqual([users[1]])
  })
})

function cssRule(selector: string) {
  const selectorStart = styles.indexOf(`\n${selector} {`)
  expect(selectorStart, `Missing CSS rule for ${selector}`).toBeGreaterThanOrEqual(0)
  const ruleStart = styles.indexOf('{', selectorStart)
  const ruleEnd = styles.indexOf('}', ruleStart)
  return styles.slice(ruleStart + 1, ruleEnd)
}
