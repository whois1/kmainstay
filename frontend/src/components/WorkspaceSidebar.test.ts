import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import WorkspaceSidebar from './WorkspaceSidebar.vue'
import type { Conversation, User } from '../types'

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
    expect(wrapper.findAll('[data-testid=group-conversations] .conversation-row .conversation-label').map(label => label.text())).toEqual(['Launch', 'Planning'])
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

    const directButton = wrapper.get('[data-testid=direct-contact-hector] .direct-topic:last-child')
    expect(directButton.attributes('aria-current')).toBe('page')
    expect(wrapper.findAll('[data-testid=direct-contact-hector] [aria-current="page"]')).toHaveLength(1)
    await wrapper.setProps({ settingsActive: true })
    expect(directButton.attributes('aria-current')).toBeUndefined()
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
