<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Conversation, Organisation, Principal, User } from '../types'

const props = defineProps<{
  organisation: Organisation | null
  principal: Principal
  conversations: Conversation[]
  users: User[]
  selected: Conversation | null
  settingsActive: boolean
  busy?: boolean
  completedArchiveConversationIDs?: string[]
}>()

const emit = defineEmits<{
  openSettings: []
  newConversation: []
  newDirectTopic: [user: User]
  newTopic: [conversation: Conversation]
  selectDirectUser: [user: User]
  selectConversation: [conversation: Conversation]
  archiveConversations: [conversations: Conversation[]]
  deleteConversations: [conversations: Conversation[]]
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
const selectedConversationIDs = ref(new Set<string>())
let selectionAnchorID = ''

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

const groupConversationThreads = computed(() => {
  const threads = new Map<string, Conversation[]>()
  for (const conversation of groupConversations.value) {
    const key = (conversation.member_ids ?? []).filter(userID => userID !== props.principal.id).sort().join('-')
    const topics = threads.get(key) ?? []
    topics.push(conversation)
    threads.set(key, topics)
  }
  return [...threads].map(([key, topics]) => {
    topics.sort(byLatestActivity)
    const memberIDs = new Set(topics[0]?.member_ids?.filter(userID => userID !== props.principal.id))
    const label = props.users.filter(user => memberIDs.has(user.id)).map(user => user.name).join(', ')
    return { key, label, topics }
  }).sort((left, right) => byLatestActivity(left.topics[0], right.topics[0]))
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

function isCurrentGroupThread(topics: Conversation[]) {
  return topics.some(isCurrentConversation)
}

function openLatestDirectTopic(user: User, topics: Conversation[]) {
  if (topics[0]) emit('selectConversation', topics[0])
  else emit('selectDirectUser', user)
}

const selectableConversations = computed(() => [
  ...pinnedConversations.value,
  ...directContacts.value.flatMap(contact => contact.topics),
  ...removedDirectConversations.value,
  ...groupConversationThreads.value.flatMap(thread => thread.topics),
  ...archivedConversations.value,
].filter(conversation => !conversation.is_everyone))
const selectedConversations = computed(() => selectableConversations.value.filter(conversation => selectedConversationIDs.value.has(conversation.id)))
const canDeleteSelectedConversations = computed(() => selectedConversations.value.length > 0 && selectedConversations.value.every(conversation => conversation.visibility === 'organisation' || (conversation.member_ids?.length ?? 0) >= 3))

function selectConversationCheckbox(event: MouseEvent, conversation: Conversation) {
  const checkbox = event.currentTarget as HTMLInputElement
  const nextSelection = new Set(selectedConversationIDs.value)
  if (event.shiftKey && selectionAnchorID) {
    const anchorIndex = selectableConversations.value.findIndex(({ id }) => id === selectionAnchorID)
    const selectedIndex = selectableConversations.value.findIndex(({ id }) => id === conversation.id)
    if (anchorIndex >= 0 && selectedIndex >= 0) {
      const [start, end] = [anchorIndex, selectedIndex].sort((left, right) => left - right)
      for (const selectedConversation of selectableConversations.value.slice(start, end + 1)) nextSelection.add(selectedConversation.id)
    }
  } else if (checkbox.checked) nextSelection.add(conversation.id)
  else nextSelection.delete(conversation.id)
  selectedConversationIDs.value = nextSelection
  selectionAnchorID = conversation.id
}

function archiveSelectedConversations() {
  const selectedActiveConversations = selectableConversations.value.filter(conversation => !conversation.archived && selectedConversationIDs.value.has(conversation.id))
  if (!selectedActiveConversations.length) return
  emit('archiveConversations', selectedActiveConversations)
}

function deleteSelectedConversations() {
  if (!canDeleteSelectedConversations.value) return
  emit('deleteConversations', selectedConversations.value)
}

watch(() => props.conversations.map(conversation => conversation.id), conversationIDs => {
  const availableConversationIDs = new Set(conversationIDs)
  const remainingSelection = new Set([...selectedConversationIDs.value].filter(conversationID => availableConversationIDs.has(conversationID)))
  if (remainingSelection.size !== selectedConversationIDs.value.size) selectedConversationIDs.value = remainingSelection
  if (selectionAnchorID && !availableConversationIDs.has(selectionAnchorID)) selectionAnchorID = ''
})

watch(() => props.completedArchiveConversationIDs, completedConversationIDs => {
  if (!completedConversationIDs?.length) return
  const remainingSelection = new Set(selectedConversationIDs.value)
  for (const conversationID of completedConversationIDs) remainingSelection.delete(conversationID)
  selectedConversationIDs.value = remainingSelection
  if (selectionAnchorID && completedConversationIDs.includes(selectionAnchorID)) selectionAnchorID = ''
})
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
        <div v-for="conversation in pinnedConversations" :key="conversation.id" class="selectable-conversation">
          <input v-if="!conversation.is_everyone" data-testid="conversation-checkbox" class="conversation-selector" type="checkbox" :checked="selectedConversationIDs.has(conversation.id)" :aria-label="`Select ${conversation.name}`" @click="selectConversationCheckbox($event, conversation)">
          <button :class="{ active: isCurrentConversation(conversation) }" :aria-current="isCurrentConversation(conversation) ? 'page' : undefined" @click="$emit('selectConversation', conversation)">
            <span class="conversation-hash">#</span><span class="conversation-label">{{ conversation.name }}</span>
          </button>
        </div>
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
          <div v-for="conversation in contact.topics" :key="conversation.id" class="selectable-conversation">
            <input data-testid="conversation-checkbox" class="conversation-selector" type="checkbox" :checked="selectedConversationIDs.has(conversation.id)" :aria-label="`Select ${directTopicName(conversation)} with ${contact.user.name}`" @click="selectConversationCheckbox($event, conversation)">
            <button class="direct-topic" :class="{ active: isCurrentConversation(conversation) }" :aria-current="isCurrentConversation(conversation) ? 'page' : undefined" @click="$emit('selectConversation', conversation)">
              <span class="topic-branch" aria-hidden="true">↳</span><span class="conversation-label">{{ directTopicName(conversation) }}</span>
            </button>
          </div>
        </div>
        <div v-for="conversation in removedDirectConversations" :key="conversation.id" class="selectable-conversation">
          <input data-testid="conversation-checkbox" class="conversation-selector" type="checkbox" :checked="selectedConversationIDs.has(conversation.id)" aria-label="Select conversation with Removed user" @click="selectConversationCheckbox($event, conversation)">
          <button class="direct-topic removed-direct-topic" :class="{ active: isCurrentConversation(conversation) }" :aria-current="isCurrentConversation(conversation) ? 'page' : undefined" @click="$emit('selectConversation', conversation)">
            <span class="direct-avatar" aria-hidden="true">?</span><span class="conversation-label">Removed user</span>
          </button>
        </div>
      </section>
      <section data-testid="group-conversations" aria-labelledby="group-heading">
        <h2 id="group-heading" class="sidebar-section-heading" aria-label="Group chats"><span aria-hidden="true">◇</span><span>Group chats</span></h2>
        <div v-for="thread in groupConversationThreads" :key="thread.key" class="direct-contact group-thread" :data-testid="`group-thread-${thread.key}`">
          <div class="direct-contact-heading group-thread-heading">
            <button class="direct-contact-button group-thread-button" :class="{ active: isCurrentGroupThread(thread.topics) }" @click="$emit('selectConversation', thread.topics[0])">
              <span class="direct-avatar" aria-hidden="true">{{ thread.label.slice(0, 1) }}</span><span class="conversation-label">{{ thread.label }}</span>
            </button>
            <button :data-testid="`new-group-topic-${thread.key}`" class="topic-add" :aria-label="`New chat with ${thread.label}`" title="New chat with the same people" @click="$emit('newTopic', thread.topics[0])">＋</button>
          </div>
          <div v-for="conversation in thread.topics" :key="conversation.id" class="selectable-conversation">
            <input data-testid="conversation-checkbox" class="conversation-selector" type="checkbox" :checked="selectedConversationIDs.has(conversation.id)" :aria-label="`Select ${conversation.name} with ${thread.label}`" @click="selectConversationCheckbox($event, conversation)">
            <button class="direct-topic group-topic" :class="{ active: isCurrentConversation(conversation) }" :aria-current="isCurrentConversation(conversation) ? 'page' : undefined" @click="$emit('selectConversation', conversation)">
              <span class="topic-branch" aria-hidden="true">↳</span><span class="conversation-label">{{ conversation.name }}</span>
            </button>
          </div>
        </div>
      </section>
      <section v-if="archivedConversations.length" data-testid="archived-conversations" aria-labelledby="archived-heading">
        <h2 id="archived-heading" class="sidebar-section-heading"><span aria-hidden="true">▣</span><span>Archived</span></h2>
        <div v-for="conversation in archivedConversations" :key="conversation.id" class="selectable-conversation">
          <input data-testid="conversation-checkbox" class="conversation-selector" type="checkbox" :checked="selectedConversationIDs.has(conversation.id)" :aria-label="`Select ${conversation.name}`" @click="selectConversationCheckbox($event, conversation)">
          <button :class="{ active: isCurrentConversation(conversation) }" :aria-current="isCurrentConversation(conversation) ? 'page' : undefined" @click="$emit('selectConversation', conversation)">
            <span class="conversation-hash">#</span><span class="conversation-label">{{ conversation.name }}</span>
          </button>
        </div>
      </section>
    </nav>
    <div v-if="selectedConversationIDs.size" data-testid="bulk-conversation-actions" class="bulk-conversation-actions" aria-live="polite">
      <span>{{ selectedConversationIDs.size }} selected</span>
      <button data-testid="bulk-archive-conversations" :disabled="busy" @click="archiveSelectedConversations">Archive</button>
      <button v-if="organisation?.role === 'admin' && canDeleteSelectedConversations" data-testid="bulk-delete-conversations" class="danger-outline" :disabled="busy" @click="deleteSelectedConversations">Delete</button>
    </div>
    <div class="profile"><span>{{ principal.name.slice(0, 1) }}</span><div><strong>{{ principal.name }}</strong><small>Human</small></div></div>
  </aside>
</template>
