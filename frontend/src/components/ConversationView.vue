<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import { renderMarkdown } from '../markdown'
import type { Conversation, Message } from '../types'

const props = defineProps<{ selected: Conversation | null; messages: Message[]; composer: string; busy: boolean; error: string; canDelete: boolean; hasNewerMessages?: boolean; jumpToLatestVersion?: number }>()
const emit = defineEmits<{ deleteConversation: []; sendMessage: []; readThrough: [sequence: number]; jumpToLatest: []; 'update:composer': [value: string] }>()

const firstUnreadMessageID = ref<string | null>(null)
const messageList = ref<HTMLElement | null>(null)
const showJumpToBottom = ref(false)
let capturedConversationID: string | null = null
let initiallyPositioned = false
let positionedMessageCount = 0

watch(() => [props.selected?.id, props.selected?.read_sequence, props.messages] as const, async ([conversationID, , messages]) => {
  if (conversationID !== capturedConversationID) {
    capturedConversationID = conversationID ?? null
    firstUnreadMessageID.value = null
    initiallyPositioned = false
    positionedMessageCount = 0
    showJumpToBottom.value = false
  }
  if (conversationID && messages.length > 0 && !initiallyPositioned) {
    firstUnreadMessageID.value = messages.find(message => message.sequence > (props.selected?.read_sequence ?? 0))?.id ?? ''
    initiallyPositioned = true
    positionedMessageCount = messages.length
    await nextTick()
    if (props.selected?.id !== conversationID) return
    if (firstUnreadMessageID.value) {
      messageList.value?.querySelector<HTMLElement>('[data-testid="new-messages-divider"]')?.scrollIntoView?.({ block: 'start' })
      showJumpToBottom.value = Boolean(props.hasNewerMessages) || !isNearBottom()
    } else if (messageList.value) {
      messageList.value.scrollTop = messageList.value.scrollHeight
    }
    emit('readThrough', messages.at(-1)?.sequence ?? 0)
  }
}, { immediate: true })

watch(() => props.messages.length, async (length) => {
  if (!initiallyPositioned || length <= positionedMessageCount) return
  positionedMessageCount = length
  const shouldFollow = isNearBottom()
  if (!shouldFollow) {
    showJumpToBottom.value = true
    return
  }
  await nextTick()
  scrollToBottom()
  emitLatestReadThrough()
})

watch(() => props.hasNewerMessages, hasNewer => {
  if (hasNewer) showJumpToBottom.value = true
})

watch(() => props.jumpToLatestVersion, async (version, previousVersion) => {
  if (!version || version === previousVersion) return
  await nextTick()
  scrollToBottom()
  emitLatestReadThrough()
})

function isNearBottom() {
  const element = messageList.value
  return element ? element.scrollHeight - element.scrollTop - element.clientHeight <= 80 : true
}

function scrollToBottom() {
  const element = messageList.value
  if (!element) return
  if (typeof element.scrollTo === 'function') element.scrollTo({ top: element.scrollHeight })
  else element.scrollTop = element.scrollHeight
  showJumpToBottom.value = false
}

function emitLatestReadThrough() {
  const latestSequence = props.messages.at(-1)?.sequence
  if (latestSequence !== undefined) emit('readThrough', latestSequence)
}

function jumpToBottom() {
  if (props.hasNewerMessages) {
    emit('jumpToLatest')
    return
  }
  scrollToBottom()
  emitLatestReadThrough()
}

function handleScroll() {
  if (props.hasNewerMessages) {
    showJumpToBottom.value = true
    if (isNearBottom()) emitLatestReadThrough()
    return
  }
  if (!isNearBottom()) {
    showJumpToBottom.value = props.messages.length > 0
    return
  }
  showJumpToBottom.value = false
  emitLatestReadThrough()
}
</script>

<template>
  <section class="conversation" aria-label="Conversation">
    <header v-if="selected">
      <div><h1># {{ selected.name }}</h1><p>{{ selected.visibility === 'members' ? 'Private conversation' : 'Everyone in the organisation' }}</p></div>
      <button v-if="canDelete" data-testid="delete-conversation" class="danger-outline" :disabled="busy" aria-label="Delete conversation" @click="$emit('deleteConversation')"><span class="delete-label">Delete conversation</span><span class="delete-icon" aria-hidden="true">Delete</span></button>
    </header>
    <div v-else class="empty-state"><h1>No conversations</h1><p>Create one from the conversations list.</p></div>
    <div ref="messageList" class="message-list" aria-live="polite" @scroll="handleScroll">
      <template v-for="message in messages" :key="message.id">
      <div v-if="message.id === firstUnreadMessageID" data-testid="new-messages-divider" class="new-messages-divider" role="separator" aria-label="New messages"><span>New messages</span></div>
      <article data-testid="message" :data-message-id="message.id">
        <div class="avatar" :class="{ bot: message.author_kind === 'bot' }">{{ message.author_name.slice(0, 1) }}</div>
        <div><div class="message-meta"><strong>{{ message.author_name }}</strong><span v-if="message.author_kind === 'bot'" class="bot-badge">BOT</span><time>{{ new Date(message.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) }}</time></div><div class="markdown" v-html="renderMarkdown(message.body)" /></div>
      </article>
      </template>
    </div>
    <button v-if="showJumpToBottom" data-testid="jump-to-bottom" class="jump-to-bottom" type="button" aria-label="Jump to latest message" @click="jumpToBottom">↓</button>
    <form v-if="selected" data-testid="composer" class="composer" @submit.prevent="$emit('sendMessage')">
      <textarea :value="composer" :placeholder="`Message #${selected.name}`" rows="1" aria-label="Message" @input="$emit('update:composer', ($event.target as HTMLTextAreaElement).value)" @keydown.enter.exact.prevent="$emit('sendMessage')" />
      <button :disabled="busy || !composer.trim()" aria-label="Send message">Send</button><small>Markdown supported · Enter to send</small>
    </form>
    <p v-if="error" class="toast error" role="alert">{{ error }}</p>
  </section>
</template>
