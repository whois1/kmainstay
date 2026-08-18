<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { renderMarkdown } from '../markdown'
import type { Conversation, Message, User } from '../types'

const props = defineProps<{ selected: Conversation | null; messages: Message[]; composer: string; busy: boolean; error: string; canDelete: boolean; users?: User[]; currentUserID?: string; hasNewerMessages?: boolean; jumpToLatestVersion?: number }>()
const emit = defineEmits<{ deleteConversation: []; sendMessage: []; readThrough: [sequence: number]; jumpToLatest: []; 'update:composer': [value: string] }>()

const firstUnreadMessageID = ref<string | null>(null)
const messageList = ref<HTMLElement | null>(null)
const showJumpToBottom = ref(false)
const textarea = ref<HTMLTextAreaElement | null>(null)
const draft = ref(props.composer)
const mentionStart = ref<number | null>(null)
const mentionQuery = ref('')
const activeSuggestion = ref(0)
const pickerOpen = ref(false)
const suggestions = computed(() => (props.users ?? []).filter(user => user.id !== props.currentUserID && user.name.toLocaleLowerCase().includes(mentionQuery.value.toLocaleLowerCase())))
const pickerVisible = computed(() => pickerOpen.value && suggestions.value.length > 0)
const activeOptionID = computed(() => pickerVisible.value ? `mention-option-${suggestions.value[activeSuggestion.value]?.id}` : undefined)
const directUser = computed(() => {
  if (props.selected?.visibility !== 'members' || props.selected.member_ids?.length !== 2) return null
  const otherUserID = props.selected.member_ids.find(id => id !== props.currentUserID)
  return (props.users ?? []).find(user => user.id === otherUserID) ?? null
})
const removedDirectConversation = computed(() => props.selected?.visibility === 'members' && props.selected.member_ids?.length === 1 && props.selected.member_ids[0] === props.currentUserID)
const conversationDisplayName = computed(() => removedDirectConversation.value ? 'Removed user' : directUser.value?.name ?? props.selected?.name ?? '')
const directBot = computed(() => {
  return directUser.value?.kind === 'bot' ? directUser.value : null
})
let capturedConversationID: string | null = null
let initiallyPositioned = false
let positionedMessageCount = 0

watch(() => props.composer, value => { draft.value = value })
watch(() => props.selected?.id, () => closeMentionPicker())

function updateComposer(event: Event) {
  const element = event.target as HTMLTextAreaElement
  draft.value = element.value
  emit('update:composer', element.value)
  updateMentionPicker(element)
}

function updateMentionPicker(element: HTMLTextAreaElement) {
  const caret = element.selectionStart
  const beforeCaret = element.value.slice(0, caret)
  const at = beforeCaret.lastIndexOf('@')
  if (at < 0 || (at > 0 && /[\p{L}\p{N}\p{M}_]/u.test(beforeCaret[at - 1])) || /[\n\r]/.test(beforeCaret.slice(at + 1))) { closeMentionPicker(); return }
  mentionStart.value = at
  mentionQuery.value = beforeCaret.slice(at + 1)
  activeSuggestion.value = 0
  pickerOpen.value = true
}

function handleComposerKeydown(event: KeyboardEvent) {
	const plainEnter = event.key === 'Enter' && !event.shiftKey && !event.ctrlKey && !event.altKey && !event.metaKey && !event.isComposing
  if (!pickerOpen.value) {
    if (plainEnter) { event.preventDefault(); emit('sendMessage') }
    return
  }
  if (event.key === 'Escape') { event.preventDefault(); closeMentionPicker(); return }
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    if (!suggestions.value.length) return
    const change = event.key === 'ArrowDown' ? 1 : -1
    activeSuggestion.value = (activeSuggestion.value + change + suggestions.value.length) % suggestions.value.length
    return
  }
  if (plainEnter && suggestions.value[activeSuggestion.value]) { event.preventDefault(); selectMention(suggestions.value[activeSuggestion.value]); return }
  if (plainEnter) { event.preventDefault(); emit('sendMessage') }
}

async function selectMention(user: User) {
  if (mentionStart.value === null || !textarea.value) return
  const caret = textarea.value.selectionStart
  const inserted = `@${user.name} `
  const value = draft.value.slice(0, mentionStart.value) + inserted + draft.value.slice(caret)
  const nextCaret = mentionStart.value + inserted.length
  draft.value = value
  emit('update:composer', value)
  closeMentionPicker()
  await nextTick()
  textarea.value?.focus()
  textarea.value?.setSelectionRange(nextCaret, nextCaret)
}

function closeMentionPicker() { pickerOpen.value = false; mentionStart.value = null; mentionQuery.value = ''; activeSuggestion.value = 0 }

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
      <div><h1>{{ directUser || removedDirectConversation ? '' : '# ' }}{{ conversationDisplayName }}</h1><p>{{ removedDirectConversation ? 'This person is no longer available' : selected.visibility === 'members' ? 'Private conversation' : 'Everyone in the organisation' }}</p></div>
      <button v-if="canDelete" data-testid="delete-conversation" class="danger-outline" :disabled="busy" aria-label="Delete conversation" @click="$emit('deleteConversation')"><span class="delete-label">Delete conversation</span><span class="delete-icon" aria-hidden="true">Delete</span></button>
    </header>
    <div v-else class="empty-state"><h1>No conversations</h1><p>Create one from the conversations list.</p></div>
    <div ref="messageList" class="message-list" aria-live="polite" @scroll="handleScroll">
      <template v-for="message in messages" :key="message.id">
      <div v-if="message.id === firstUnreadMessageID" data-testid="new-messages-divider" class="new-messages-divider" role="separator" aria-label="New messages"><span>New messages</span></div>
      <article data-testid="message" :data-message-id="message.id">
        <div class="avatar" :class="{ bot: message.author_kind === 'bot' }">{{ message.author_name.slice(0, 1) }}</div>
        <div><div class="message-meta"><strong>{{ message.author_name }}</strong><span v-if="message.author_kind === 'bot'" class="bot-badge">BOT</span><time>{{ new Date(message.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) }}</time></div><div class="markdown" v-html="renderMarkdown(message.body, message.mentions)" /></div>
      </article>
      </template>
    </div>
    <button v-if="showJumpToBottom" data-testid="jump-to-bottom" class="jump-to-bottom" type="button" aria-label="Jump to latest message" @click="jumpToBottom">↓</button>
    <form v-if="selected" data-testid="composer" class="composer" @submit.prevent="$emit('sendMessage')">
      <div class="composer-input"><textarea ref="textarea" :value="draft" :disabled="removedDirectConversation" :placeholder="removedDirectConversation ? 'Conversation unavailable' : directUser ? `Message ${conversationDisplayName}` : `Message #${conversationDisplayName}`" rows="1" role="combobox" aria-label="Message" aria-autocomplete="list" aria-haspopup="listbox" :aria-expanded="pickerVisible" :aria-controls="pickerVisible ? 'mention-suggestions' : undefined" :aria-activedescendant="activeOptionID" @input="updateComposer" @click="updateMentionPicker($event.target as HTMLTextAreaElement)" @keydown="handleComposerKeydown" />
        <ul v-if="pickerVisible" id="mention-suggestions" class="mention-picker" role="listbox" aria-label="Mention a person or bot">
          <li v-for="(user, index) in suggestions" :id="`mention-option-${user.id}`" :key="user.id" role="option" :aria-selected="index === activeSuggestion" @mousedown.prevent @click="selectMention(user)"><span>{{ user.name }}</span><small>{{ user.kind }}</small></li>
        </ul>
      </div>
      <button :disabled="removedDirectConversation || busy || !draft.trim()" aria-label="Send message">Send</button><small>{{ removedDirectConversation ? 'This conversation is unavailable' : 'Markdown supported · Enter to send' }}</small>
      <small v-if="(users ?? []).some(user => user.kind === 'bot')" data-testid="bot-guidance">{{ directBot ? `${directBot.name} responds automatically in this private chat.` : 'Bots respond when you @mention them.' }}</small>
    </form>
    <p v-if="error" class="toast error" role="alert">{{ error }}</p>
  </section>
</template>
