import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import WorkspaceSidebar from './WorkspaceSidebar.vue'
import type { Conversation, User } from '../types'

const conversations: Conversation[] = [
  { id: 'pinned-old', name: 'General', visibility: 'organisation', latest_sequence: 2 },
  { id: 'pinned-new', name: 'Announcements', visibility: 'organisation', latest_sequence: 12 },
  { id: 'direct-old', name: 'Conversation with Hector', visibility: 'members', member_ids: ['michael', 'hector'], latest_sequence: 4 },
  { id: 'direct-new', name: 'Conversation with Mary', visibility: 'members', member_ids: ['michael', 'mary'], latest_sequence: 10 },
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
    expect(wrapper.findAll('[data-testid=direct-conversations] .conversation-label').map(label => label.text())).toEqual(['Mary', 'Hector', 'Alfred', 'Zoe'])
    expect(wrapper.findAll('[data-testid=direct-conversations] button').map(button => button.find('small').exists() ? button.find('small').text() : '')).toEqual(['', 'bot', 'bot', ''])
    expect(wrapper.findAll('[data-testid=group-conversations] > button').map(button => button.text())).toEqual(['#Launch', '#Planning'])
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

    const directButton = wrapper.findAll('[data-testid=direct-conversations] button')[1]
    expect(directButton.attributes('aria-current')).toBe('page')
    await wrapper.setProps({ settingsActive: true })
    expect(directButton.attributes('aria-current')).toBeUndefined()
  })

  it('emits the direct user when their display name is selected', async () => {
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

    await wrapper.findAll('[data-testid=direct-conversations] button')[1].trigger('click')
    expect(wrapper.emitted('selectDirectUser')?.[0]).toEqual([users[1]])
  })
})
