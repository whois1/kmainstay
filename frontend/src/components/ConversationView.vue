<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { renderMarkdown } from '../markdown'
import type { Conversation, Message, User } from '../types'

const props = defineProps<{ selected: Conversation | null; messages: Message[]; composer: string; images?: File[]; busy: boolean; titleBusy?: boolean; error: string; canDelete: boolean; users?: User[]; currentUserID?: string; hasNewerMessages?: boolean; jumpToLatestVersion?: number; replyingTo?: Message | null }>()
const emit = defineEmits<{ deleteConversation: []; archiveConversation: []; restoreConversation: []; newTopic: []; updateTitle: [name: string]; replyTo: [message: Message]; cancelReply: []; sendMessage: []; readThrough: [sequence: number]; jumpToLatest: []; 'update:composer': [value: string]; 'update:images': [value: File[]] }>()

const firstUnreadMessageID = ref<string | null>(null)
const messageList = ref<HTMLElement | null>(null)
const showJumpToBottom = ref(false)
const textarea = ref<HTMLTextAreaElement | null>(null)
const draft = ref(props.composer)
const mentionStart = ref<number | null>(null)
const mentionQuery = ref('')
const activeSuggestion = ref(0)
const pickerOpen = ref(false)
const selectedImages = ref<File[]>(props.images ?? [])
const imageError = ref('')
const previewURLs = ref<string[]>([])
const editingTitle = ref(false)
const titleDraft = ref('')
const isDraft = computed(() => props.selected?.id.startsWith('draft:') ?? false)
const suggestions = computed(() => (props.users ?? []).filter(user => user.id !== props.currentUserID && user.name.toLocaleLowerCase().includes(mentionQuery.value.toLocaleLowerCase())))
const pickerVisible = computed(() => pickerOpen.value && suggestions.value.length > 0)
const activeOptionID = computed(() => pickerVisible.value ? `mention-option-${suggestions.value[activeSuggestion.value]?.id}` : undefined)
const directUser = computed(() => {
  if (props.selected?.visibility !== 'members' || props.selected.member_ids?.length !== 2) return null
  const otherUserID = props.selected.member_ids.find(id => id !== props.currentUserID)
  return (props.users ?? []).find(user => user.id === otherUserID) ?? null
})
const removedDirectConversation = computed(() => props.selected?.visibility === 'members' && props.selected.member_ids?.length === 1 && props.selected.member_ids[0] === props.currentUserID)
const reservedDirectConversation = computed(() => {
  if (!props.selected) return false
  const legacyPairName = `direct:${[...(props.selected.member_ids ?? [])].sort().join(':')}`
  return props.selected.name === `direct:${props.selected.id}` || props.selected.name === legacyPairName
})
const conversationDisplayName = computed(() => {
  if (removedDirectConversation.value) return 'Removed user'
  if (directUser.value && reservedDirectConversation.value) return 'General'
  return props.selected?.name ?? ''
})
const conversationContext = computed(() => {
  if (removedDirectConversation.value) return 'This person is no longer available'
  if (directUser.value) return `With ${directUser.value.name}`
  return props.selected?.visibility === 'members' ? 'Private conversation' : 'Everyone in the organisation'
})
const directBot = computed(() => {
  return directUser.value?.kind === 'bot' ? directUser.value : null
})
let capturedConversationID: string | null = null
let initiallyPositioned = false
let positionedMessageCount = 0

watch(() => props.composer, value => { draft.value = value })
watch(() => props.images, value => {
  const images = value ?? []
  if (images.length !== selectedImages.value.length || images.some((image, index) => image !== selectedImages.value[index])) selectedImages.value = [...images]
})
watch(selectedImages, updatePreviews, { immediate: true })
watch(() => props.selected?.id, async () => {
  closeMentionPicker()
  imageError.value = ''
  editingTitle.value = false
  clearImages()
  await nextTick()
  if (isDraft.value) textarea.value?.focus()
})

function beginTitleEdit() {
  titleDraft.value = conversationDisplayName.value
  editingTitle.value = true
}

function submitTitle() {
  const name = titleDraft.value.trim()
  if (!name) return
  emit('updateTitle', name)
  editingTitle.value = false
}

function selectImage(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  input.value = ''
  selectImageFiles(files)
}

function selectImageFiles(files: File[]) {
  imageError.value = ''
  if (!files.length) return
  if (selectedImages.value.length + files.length > 10) {
    imageError.value = 'Add no more than 10 images.'
    return
  }
  if (files.some(file => !['image/jpeg', 'image/png'].includes(file.type) || file.size > 10 * 1024 * 1024)) {
    imageError.value = 'Choose JPEG or PNG images up to 10 MB each.'
    return
  }
  if ([...selectedImages.value, ...files].reduce((total, file) => total + file.size, 0) > 20 * 1024 * 1024) {
    imageError.value = 'Images must be no more than 20 MB total.'
    return
  }
  selectedImages.value = [...selectedImages.value, ...files]
  emit('update:images', selectedImages.value)
}

function pasteImage(event: ClipboardEvent) {
  const files = Array.from(event.clipboardData?.files ?? [])
  if (!files.length) return
  event.preventDefault()
  if (props.busy || removedDirectConversation.value) return
  selectImageFiles(files)
}

function dropImage(event: DragEvent) {
  const files = Array.from(event.dataTransfer?.files ?? [])
  if (!files.length) return
  event.preventDefault()
  if (props.busy || removedDirectConversation.value) return
  selectImageFiles(files)
}

function allowImageDrop(event: DragEvent) {
  if (event.dataTransfer?.types.includes('Files')) event.preventDefault()
}

function removeImage(index: number) {
  selectedImages.value = selectedImages.value.filter((_, imageIndex) => imageIndex !== index)
  imageError.value = ''
  emit('update:images', selectedImages.value)
}

function clearImages() {
  if (!selectedImages.value.length && !previewURLs.value.length) return
  selectedImages.value = []
  imageError.value = ''
  emit('update:images', [])
}

function updatePreviews() {
  if (typeof URL.revokeObjectURL === 'function') previewURLs.value.forEach(url => URL.revokeObjectURL(url))
  previewURLs.value = typeof URL.createObjectURL === 'function' ? selectedImages.value.map(image => URL.createObjectURL(image)) : []
}

onBeforeUnmount(() => {
  if (typeof URL.revokeObjectURL === 'function') previewURLs.value.forEach(url => URL.revokeObjectURL(url))
})

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
      <div class="conversation-header-copy">
        <form v-if="editingTitle" data-testid="conversation-title-form" class="conversation-title-form" @submit.prevent="submitTitle"><input data-testid="conversation-title-input" v-model="titleDraft" aria-label="Conversation title" required><button type="submit">Save</button><button type="button" class="secondary" @click="editingTitle = false">Cancel</button></form>
        <div v-else class="conversation-title-row"><h1>{{ directUser || removedDirectConversation ? '' : '# ' }}{{ conversationDisplayName }}</h1><button v-if="!removedDirectConversation && !isDraft" data-testid="edit-conversation-title" class="title-edit" type="button" aria-label="Edit conversation title" :disabled="titleBusy" @click="beginTitleEdit">{{ titleBusy ? 'Saving…' : 'Edit' }}</button></div>
        <p>{{ conversationContext }}</p>
      </div>
      <div v-if="!isDraft" class="conversation-header-actions"><button v-if="selected.visibility === 'members' && !removedDirectConversation && !selected.archived" data-testid="new-topic" class="secondary" aria-label="New chat with the same people" title="New chat with the same people" @click="$emit('newTopic')">＋ New chat</button><button v-if="selected.archived" data-testid="restore-conversation" class="secondary" :disabled="busy" @click="$emit('restoreConversation')">Restore</button><button v-else data-testid="archive-conversation" class="secondary" :disabled="busy" @click="$emit('archiveConversation')">Archive</button><button v-if="canDelete" data-testid="delete-conversation" class="danger-outline" :disabled="busy" aria-label="Delete conversation" @click="$emit('deleteConversation')"><span class="delete-label">Delete conversation</span><span class="delete-icon" aria-hidden="true">Delete</span></button></div>
    </header>
    <div v-else class="empty-state"><h1>No conversations</h1><p>Create one from the conversations list.</p></div>
    <div class="message-list-shell">
      <div ref="messageList" class="message-list" :class="{ 'has-jump-control': showJumpToBottom }" aria-live="polite" @scroll="handleScroll">
        <template v-for="message in messages" :key="message.id">
        <div v-if="message.id === firstUnreadMessageID" data-testid="new-messages-divider" class="new-messages-divider" role="separator" aria-label="New messages"><span>New messages</span></div>
        <article data-testid="message" :data-message-id="message.id">
          <div class="avatar" :class="{ bot: message.author_kind === 'bot' }">{{ message.author_name.slice(0, 1) }}</div>
          <div><div class="message-meta"><strong>{{ message.author_name }}</strong><span v-if="message.author_kind === 'bot'" class="bot-badge">BOT</span><time>{{ new Date(message.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) }}</time><button data-testid="reply-message" class="message-reply-action" type="button" @click="$emit('replyTo', message)">Reply</button></div><div v-if="message.reply_to" data-testid="message-reply" class="message-reply"><strong>{{ message.reply_to.author_name }}</strong><span>{{ message.reply_to.body }}</span></div><div v-for="attachment in message.attachments ?? []" :key="attachment.id" class="message-image-frame"><img data-testid="message-image" class="message-image" :src="attachment.content_url" :alt="attachment.original_filename" loading="lazy" :width="attachment.width" :height="attachment.height" /></div><div v-if="message.body.trim()" class="markdown" v-html="renderMarkdown(message.body, message.mentions)" /></div>
        </article>
        </template>
      </div>
      <button v-if="showJumpToBottom" data-testid="jump-to-bottom" class="jump-to-bottom" type="button" aria-label="Jump to latest message" @click="jumpToBottom">↓</button>
    </div>
    <form v-if="selected" data-testid="composer" class="composer" @submit.prevent="$emit('sendMessage')" @dragover="allowImageDrop" @drop="dropImage">
      <div v-if="replyingTo" data-testid="reply-preview" class="reply-preview"><div><strong>Replying to {{ replyingTo.author_name }}</strong><span>{{ replyingTo.body }}</span></div><button type="button" aria-label="Cancel reply" @click="$emit('cancelReply')">×</button></div>
      <div class="composer-input"><textarea ref="textarea" :value="draft" :disabled="removedDirectConversation || selected.archived" :placeholder="selected.archived ? 'Restore this conversation to reply' : removedDirectConversation ? 'Conversation unavailable' : directUser ? `Message ${conversationDisplayName}` : `Message #${conversationDisplayName}`" rows="1" role="combobox" aria-label="Message" aria-autocomplete="list" aria-haspopup="listbox" :aria-expanded="pickerVisible" :aria-controls="pickerVisible ? 'mention-suggestions' : undefined" :aria-activedescendant="activeOptionID" @input="updateComposer" @click="updateMentionPicker($event.target as HTMLTextAreaElement)" @keydown="handleComposerKeydown" @paste="pasteImage" />
        <ul v-if="pickerVisible" id="mention-suggestions" class="mention-picker" role="listbox" aria-label="Mention a person or bot">
          <li v-for="(user, index) in suggestions" :id="`mention-option-${user.id}`" :key="user.id" role="option" :aria-selected="index === activeSuggestion" @mousedown.prevent @click="selectMention(user)"><span>{{ user.name }}</span><small>{{ user.kind }}</small></li>
        </ul>
      </div>
      <div v-for="(selectedImage, index) in selectedImages" :key="`${selectedImage.name}-${selectedImage.size}-${selectedImage.lastModified}-${index}`" data-testid="selected-image" class="selected-image"><img v-if="previewURLs[index]" :src="previewURLs[index]" alt="Selected image preview" /><span>{{ selectedImage.name }}</span><button data-testid="remove-image" type="button" :aria-label="`Remove ${selectedImage.name}`" @click="removeImage(index)">Remove</button></div>
      <div class="composer-actions"><label class="image-picker" :class="{ disabled: removedDirectConversation || selected.archived || busy }">Add images<input type="file" accept="image/jpeg,image/png" multiple :disabled="removedDirectConversation || selected.archived || busy" @change="selectImage" /></label><button :disabled="removedDirectConversation || selected.archived || busy || (!draft.trim() && !selectedImages.length)" aria-label="Send message">Send</button></div>
      <small>{{ selected.archived ? 'Restore this conversation to reply' : removedDirectConversation ? 'This conversation is unavailable' : 'Paste, drop or add up to 10 JPEG/PNG images · 20 MB total · Enter to send' }}</small>
      <small v-if="imageError" class="image-error" role="alert">{{ imageError }}</small>
      <small v-if="(users ?? []).some(user => user.kind === 'bot')" data-testid="bot-guidance">{{ directBot ? `${directBot.name} responds automatically in this private chat.` : 'Bots respond when you @mention them.' }}</small>
    </form>
    <p v-if="error" class="toast error" role="alert">{{ error }}</p>
  </section>
</template>
