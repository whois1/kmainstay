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

defineEmits<{
  openSettings: []
  newConversation: []
  selectConversation: [conversation: Conversation]
}>()

function byLatestMessage(left: Conversation, right: Conversation) {
  return (right.latest_sequence ?? 0) - (left.latest_sequence ?? 0) || left.name.localeCompare(right.name)
}

const pinnedConversations = computed(() => props.conversations.filter(conversation => conversation.visibility === 'organisation').sort(byLatestMessage))
const directConversations = computed(() => props.conversations.filter(conversation => conversation.visibility === 'members' && conversation.member_ids?.length === 2).sort(byLatestMessage))
const groupConversations = computed(() => props.conversations.filter(conversation => conversation.visibility === 'members' && conversation.member_ids?.length !== 2).sort(byLatestMessage))

function directUser(conversation: Conversation) {
  const otherUserID = conversation.member_ids?.find(userID => userID !== props.principal.id)
  return props.users.find(user => user.id === otherUserID)
}

function isCurrentConversation(conversation: Conversation) {
  return !props.settingsActive && props.selected?.id === conversation.id
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
    <div class="side-heading"><span>Conversations</span><button data-testid="new-conversation" class="icon-button" aria-label="New private conversation" @click="$emit('newConversation')">＋</button></div>
    <nav class="sidebar-navigation" aria-label="Conversations">
      <section data-testid="pinned-conversations" aria-labelledby="pinned-heading">
        <h2 id="pinned-heading" class="sidebar-section-heading" aria-label="Pinned"><span aria-hidden="true">⌖</span><span>Pinned</span></h2>
        <button v-for="conversation in pinnedConversations" :key="conversation.id" :class="{ active: isCurrentConversation(conversation) }" :aria-current="isCurrentConversation(conversation) ? 'page' : undefined" @click="$emit('selectConversation', conversation)">
          <span class="conversation-hash">#</span><span class="conversation-label">{{ conversation.name }}</span>
        </button>
      </section>
      <section data-testid="direct-conversations" aria-labelledby="direct-heading">
        <h2 id="direct-heading" class="sidebar-section-heading" aria-label="Direct messages"><span aria-hidden="true">●</span><span>Direct messages</span></h2>
        <button v-for="conversation in directConversations" :key="conversation.id" :class="{ active: isCurrentConversation(conversation) }" :aria-current="isCurrentConversation(conversation) ? 'page' : undefined" @click="$emit('selectConversation', conversation)">
          <span class="direct-avatar" aria-hidden="true">{{ (directUser(conversation)?.name ?? conversation.name).slice(0, 1) }}</span>
          <span class="conversation-label">{{ directUser(conversation)?.name ?? conversation.name }}</span><small v-if="directUser(conversation)?.kind === 'bot'">bot</small>
        </button>
      </section>
      <section data-testid="group-conversations" aria-labelledby="group-heading">
        <h2 id="group-heading" class="sidebar-section-heading" aria-label="Group chats"><span aria-hidden="true">◇</span><span>Group chats</span></h2>
        <button v-for="conversation in groupConversations" :key="conversation.id" :class="{ active: isCurrentConversation(conversation) }" :aria-current="isCurrentConversation(conversation) ? 'page' : undefined" @click="$emit('selectConversation', conversation)">
          <span class="conversation-hash">#</span><span class="conversation-label">{{ conversation.name }}</span>
        </button>
      </section>
    </nav>
    <div class="profile"><span>{{ principal.name.slice(0, 1) }}</span><div><strong>{{ principal.name }}</strong><small>Human</small></div></div>
  </aside>
</template>
