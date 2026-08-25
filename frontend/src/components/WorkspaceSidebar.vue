<script setup lang="ts">
import { computed } from 'vue'
import type { Conversation, Organisation, Principal, User } from '../types'

const props = defineProps<{
  organisation: Organisation | null
  principal: Principal
  conversations: Conversation[]
  users: User[]
  selected: Conversation | null
  settingsActive: boolean
}>()

const emit = defineEmits<{
  openSettings: []
  newConversation: []
  newDirectTopic: [user: User]
  newTopic: [conversation: Conversation]
  selectDirectUser: [user: User]
  selectConversation: [conversation: Conversation]
}>()

function byLatestActivity(left: Conversation, right: Conversation) {
  if (left.activity_at && right.activity_at && left.activity_at !== right.activity_at) {
    return Date.parse(right.activity_at) - Date.parse(left.activity_at)
      || right.activity_at.localeCompare(left.activity_at)
  }
  return (right.latest_sequence ?? 0) - (left.latest_sequence ?? 0)
    || props.conversations.indexOf(right) - props.conversations.indexOf(left)
}

const activeConversations = computed(() => props.conversations.filter(conversation => !conversation.archived))
const archivedConversations = computed(() => props.conversations.filter(conversation => conversation.archived).sort(byLatestActivity))
const pinnedConversations = computed(() => activeConversations.value.filter(conversation => conversation.visibility === 'organisation').sort(byLatestActivity))
const directConversations = computed(() => activeConversations.value.filter(conversation => conversation.visibility === 'members' && conversation.member_ids?.length === 2).sort(byLatestActivity))
const groupConversations = computed(() => activeConversations.value.filter(conversation => conversation.visibility === 'members' && (conversation.member_ids?.length ?? 0) > 2).sort(byLatestActivity))
const removedDirectConversations = computed(() => activeConversations.value.filter(conversation => conversation.visibility === 'members' && conversation.member_ids?.length === 1 && conversation.member_ids[0] === props.principal?.id).sort(byLatestActivity))

const directContacts = computed(() => {
  const contacts = props.users
    .filter(user => user.id !== props.principal.id)
    .map(user => {
      const topics = directConversations.value.filter(conversation => conversation.member_ids?.includes(user.id))
      return { user, topics, latestActivity: topics[0]?.activity_at, latestSequence: topics[0]?.latest_sequence ?? -1 }
    })
  return contacts.sort((left, right) => {
    if (left.latestActivity && right.latestActivity && left.latestActivity !== right.latestActivity) return Date.parse(right.latestActivity) - Date.parse(left.latestActivity) || right.latestActivity.localeCompare(left.latestActivity)
    if (left.latestActivity !== right.latestActivity) return left.latestActivity ? -1 : 1
    return right.latestSequence - left.latestSequence || left.user.name.localeCompare(right.user.name)
  })
})

function directTopicName(conversation: Conversation) {
  return isReservedDirectConversation(conversation) ? 'General' : conversation.name
}

function isReservedDirectConversation(conversation: Conversation) {
  const legacyPairName = `direct:${[...(conversation.member_ids ?? [])].sort().join(':')}`
  return conversation.name === `direct:${conversation.id}` || conversation.name === legacyPairName
}

function isCurrentConversation(conversation: Conversation) {
  return !props.settingsActive && props.selected?.id === conversation.id
}

function isCurrentDirectContact(topics: Conversation[]) {
  return topics.some(isCurrentConversation)
}

function openLatestDirectTopic(user: User, topics: Conversation[]) {
  if (topics[0]) emit('selectConversation', topics[0])
  else emit('selectDirectUser', user)
}
</script>

<template>
  <aside aria-label="Workspace navigation">
    <div data-testid="organisation-brand" class="brand">
      <span class="mark">K</span>
      <span class="brand-copy"><strong>{{ organisation?.name }}</strong><small>{{ organisation?.role }}</small></span>
      <button data-testid="organisation-settings" class="brand-settings" :class="{ active: settingsActive }" aria-label="Organisation settings" title="Organisation settings" @click="$emit('openSettings')">
        <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M19.43 12.98c.04-.32.07-.65.07-.98s-.03-.66-.07-.98l2.11-1.65c.19-.15.24-.42.12-.64l-2-3.46a.5.5 0 0 0-.6-.22l-2.49 1a7.3 7.3 0 0 0-1.69-.98l-.38-2.65A.5.5 0 0 0 14 2h-4a.5.5 0 0 0-.49.42l-.38 2.65c-.61.25-1.17.58-1.69.98l-2.49-1a.5.5 0 0 0-.6.22l-2 3.46a.5.5 0 0 0 .12.64l2.11 1.65a7.4 7.4 0 0 0 0 1.96l-2.11 1.65a.5.5 0 0 0-.12.64l2 3.46a.5.5 0 0 0 .6.22l2.49-1c.52.4 1.08.73 1.69.98l.38 2.65A.5.5 0 0 0 10 22h4a.5.5 0 0 0 .49-.42l.38-2.65c.61-.25 1.17-.58 1.69-.98l2.49 1a.5.5 0 0 0 .6-.22l2-3.46a.5.5 0 0 0-.12-.64l-2.11-1.65ZM12 15.5A3.5 3.5 0 1 1 12 8a3.5 3.5 0 0 1 0 7.5Z"/></svg>
      </button>
    </div>
    <div class="side-heading"><span>Conversations</span><button data-testid="new-conversation" class="icon-button" aria-label="New group chat" @click="$emit('newConversation')">＋</button></div>
    <nav class="sidebar-navigation" aria-label="Conversations">
      <section data-testid="pinned-conversations" aria-labelledby="pinned-heading">
        <h2 id="pinned-heading" class="sidebar-section-heading" aria-label="Pinned"><span aria-hidden="true">⌖</span><span>Pinned</span></h2>
        <button v-for="conversation in pinnedConversations" :key="conversation.id" :class="{ active: isCurrentConversation(conversation) }" :aria-current="isCurrentConversation(conversation) ? 'page' : undefined" @click="$emit('selectConversation', conversation)">
          <span class="conversation-hash">#</span><span class="conversation-label">{{ conversation.name }}</span>
        </button>
      </section>
      <section data-testid="direct-conversations" aria-labelledby="direct-heading">
        <h2 id="direct-heading" class="sidebar-section-heading" aria-label="Direct messages"><span aria-hidden="true">●</span><span>Direct messages</span></h2>
        <div v-for="contact in directContacts" :key="contact.user.id" class="direct-contact" :data-testid="`direct-contact-${contact.user.id}`">
          <div class="direct-contact-heading">
            <button class="direct-contact-button" :class="{ active: isCurrentDirectContact(contact.topics) }" @click="openLatestDirectTopic(contact.user, contact.topics)">
              <span class="direct-avatar" aria-hidden="true">{{ contact.user.name.slice(0, 1) }}</span>
              <span class="conversation-label">{{ contact.user.name }}</span><small v-if="contact.user.kind === 'bot'">bot</small>
            </button>
            <button :data-testid="`new-direct-topic-${contact.user.id}`" class="topic-add" :aria-label="`New chat with ${contact.user.name}`" :title="`New chat with ${contact.user.name}`" @click="$emit('newDirectTopic', contact.user)">＋</button>
          </div>
          <button v-for="conversation in contact.topics" :key="conversation.id" class="direct-topic" :class="{ active: isCurrentConversation(conversation) }" :aria-current="isCurrentConversation(conversation) ? 'page' : undefined" @click="$emit('selectConversation', conversation)">
            <span class="topic-branch" aria-hidden="true">↳</span><span class="conversation-label">{{ directTopicName(conversation) }}</span>
          </button>
        </div>
        <button v-for="conversation in removedDirectConversations" :key="conversation.id" class="direct-topic removed-direct-topic" :class="{ active: isCurrentConversation(conversation) }" :aria-current="isCurrentConversation(conversation) ? 'page' : undefined" @click="$emit('selectConversation', conversation)">
          <span class="direct-avatar" aria-hidden="true">?</span><span class="conversation-label">Removed user</span>
        </button>
      </section>
      <section data-testid="group-conversations" aria-labelledby="group-heading">
        <h2 id="group-heading" class="sidebar-section-heading" aria-label="Group chats"><span aria-hidden="true">◇</span><span>Group chats</span></h2>
        <div v-for="conversation in groupConversations" :key="conversation.id" class="conversation-row">
          <button class="conversation-main" :class="{ active: isCurrentConversation(conversation) }" :aria-current="isCurrentConversation(conversation) ? 'page' : undefined" @click="$emit('selectConversation', conversation)">
            <span class="conversation-hash">#</span><span class="conversation-label">{{ conversation.name }}</span>
          </button>
          <button :data-testid="`new-group-topic-${conversation.id}`" class="topic-add" :aria-label="`New chat with the people in ${conversation.name}`" title="New chat with the same people" @click="$emit('newTopic', conversation)">＋</button>
        </div>
      </section>
      <section v-if="archivedConversations.length" data-testid="archived-conversations" aria-labelledby="archived-heading">
        <h2 id="archived-heading" class="sidebar-section-heading"><span aria-hidden="true">▣</span><span>Archived</span></h2>
        <button v-for="conversation in archivedConversations" :key="conversation.id" :class="{ active: isCurrentConversation(conversation) }" :aria-current="isCurrentConversation(conversation) ? 'page' : undefined" @click="$emit('selectConversation', conversation)">
          <span class="conversation-hash">#</span><span class="conversation-label">{{ conversation.name }}</span>
        </button>
      </section>
    </nav>
    <div class="profile"><span>{{ principal.name.slice(0, 1) }}</span><div><strong>{{ principal.name }}</strong><small>Human</small></div></div>
  </aside>
</template>
