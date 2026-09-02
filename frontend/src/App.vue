<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import BaseDialog from './components/BaseDialog.vue'
import ConversationView from './components/ConversationView.vue'
import OrganisationSettings from './components/OrganisationSettings.vue'
import WorkspaceSidebar from './components/WorkspaceSidebar.vue'
import { useRealtime } from './composables/realtime'
import type { Conversation, ConversationActivity, EligibleUser, Message, Organisation, Principal, User } from './types'

document.documentElement.dataset.theme = 'dark'
document.documentElement.style.colorScheme = 'dark'

type Fetcher = typeof fetch
const fetcher = inject<Fetcher>('fetcher', fetch)
const Socket = inject<typeof WebSocket>('socketFactory', WebSocket)
const me = ref<Principal | null>(null)
const organisation = ref<Organisation | null>(null)
const conversations = ref<Conversation[]>([])
const selected = ref<Conversation | null>(null)
const messages = ref<Message[]>([])
const hasNewerMessages = ref(false)
const jumpToLatestVersion = ref(0)
const email = ref('')
const password = ref('')
const composer = ref('')
const images = ref<File[]>([])
const replyingTo = ref<Message | null>(null)
const pendingMessageClientID = ref('')
const error = ref('')
const busy = ref(false)
const activeView = ref<'chat' | 'settings'>('chat')
const addUserStep = ref<'' | 'bot' | 'key'>('')
const botName = ref('')
const apiKey = ref('')
const copied = ref(false)
const keySaved = ref(false)
const keyError = ref('')
const keyAction = ref<'created' | 'rotated'>('created')
const botMutationID = ref('')
const notice = ref('')
const removingBotID = ref('')
const showConversation = ref(false)
const creatingConversation = ref(false)
const titleSavingConversationIDs = ref(new Set<string>())
const completedArchiveConversationIDs = ref<string[]>([])
let conversationCreationGeneration = 0
const conversationName = ref('')
const users = ref<User[]>([])
const eligibleUsers = ref<EligibleUser[]>([])
const eligibleEmail = ref('')
const selectedEligibleUserID = ref('')
const showAddExisting = ref(false)
const memberIDs = ref<string[]>([])
const conversationNameEdited = ref(false)
let conversationRefreshGeneration = 0
let conversationSelectionGeneration = 0
let navigationIntentGeneration = 0
let messageLoadGeneration = 0
let loadingConversationID: string | null = null
let pendingRealtimeMessages: Message[] = []
let realtimeUpdateGeneration = 0
const realtimeUpdatesByMessageID = new Map<string, { eventSequence: number; generation: number; message: Message }>()
type HistoryLoad = { conversationID: string; generationBeforeRequest: number }
const historyLoadsByConversationID = new Map<string, Set<HistoryLoad>>()
const pendingLocalEditConversationIDsByMessageID = new Map<string, string>()
const pendingDirectUserIDs = new Set<string>()
const pendingAutomaticTitleConversationIDs = new Set<string>()
const archiveStateGenerations = new Map<string, number>()
const activities = ref<Record<string, ConversationActivity>>({})
const activityTimers = new Map<string, ReturnType<typeof setTimeout>>()
const selectedActivities = computed(() => Object.values(activities.value).filter(activity => activity.conversation_id === selected.value?.id))

function replaceMessageAndReplyPreviews(updated: Message) {
  messages.value = messages.value.map(message => message.id === updated.id ? updated : message.reply_to?.id === updated.id ? { ...message, reply_to: { ...message.reply_to, body: updated.body } } : message)
  pendingRealtimeMessages = pendingRealtimeMessages.map(message => message.id === updated.id ? updated : message.reply_to?.id === updated.id ? { ...message, reply_to: { ...message.reply_to, body: updated.body } } : message)
  if (replyingTo.value?.id === updated.id) replyingTo.value = updated
}

function activityKey(conversationID: string, userID: string) {
  return `${conversationID}\u0000${userID}`
}

function clearActivity(conversationID: string, userID: string) {
  const key = activityKey(conversationID, userID)
  const timer = activityTimers.get(key)
  if (timer) clearTimeout(timer)
  activityTimers.delete(key)
  if (!(key in activities.value)) return
  const next = { ...activities.value }
  delete next[key]
  activities.value = next
}

function updateActivity(activity: ConversationActivity) {
  const expiresAt = Date.parse(activity.expires_at)
  clearActivity(activity.conversation_id, activity.user_id)
  if (!activity.active || !Number.isFinite(expiresAt) || expiresAt <= Date.now()) return
  const key = activityKey(activity.conversation_id, activity.user_id)
  activities.value = { ...activities.value, [key]: activity }
  activityTimers.set(key, setTimeout(() => clearActivity(activity.conversation_id, activity.user_id), expiresAt - Date.now()))
}

const realtime = useRealtime((message) => {
  if (message.author_kind === 'bot') clearActivity(message.conversation_id, message.author_id)
  const conversation = conversations.value.find(({ id }) => id === message.conversation_id)
  if (conversation) {
    archiveStateGenerations.set(conversation.id, (archiveStateGenerations.get(conversation.id) ?? 0) + 1)
    conversation.archived = false
    conversation.latest_sequence = Math.max(conversation.latest_sequence ?? 0, message.sequence)
    conversation.activity_at = message.created_at
    if (conversation.title_automatic && message.body.trim()) {
      void reconcileAutomaticConversationTitle(conversation.id)
      conversation.name = message.body.trim()
      conversation.title_automatic = false
    }
  }
  if (message.conversation_id === loadingConversationID) {
    if (!pendingRealtimeMessages.some(({ id }) => id === message.id)) pendingRealtimeMessages.push(message)
    return
  }
  if (message.conversation_id === selected.value?.id && !hasNewerMessages.value && !messages.value.some(({ id }) => id === message.id)) {
    messages.value.push(message)
  }
}, Socket, conversationID => { void reconcileDeletedConversation(conversationID) }, refreshConversations, updateActivity, (updated, eventSequence) => {
  const current = realtimeUpdatesByMessageID.get(updated.id)
  if (current && current.eventSequence >= eventSequence) return
  const generation = ++realtimeUpdateGeneration
  if (pendingLocalEditConversationIDsByMessageID.has(updated.id) || realtimeUpdateNeededByHistoryLoad(updated.conversation_id, generation)) {
    realtimeUpdatesByMessageID.set(updated.id, { eventSequence, generation, message: updated })
  }
  replaceMessageAndReplyPreviews(updated)
})

function realtimeUpdateNeededByHistoryLoad(conversationID: string, generation: number) {
  return [...(historyLoadsByConversationID.get(conversationID) ?? [])].some(load => generation > load.generationBeforeRequest)
}

function beginHistoryLoad(conversationID: string): HistoryLoad {
  const load = { conversationID, generationBeforeRequest: realtimeUpdateGeneration }
  const loads = historyLoadsByConversationID.get(conversationID) ?? new Set<HistoryLoad>()
  loads.add(load)
  historyLoadsByConversationID.set(conversationID, loads)
  return load
}

function finishHistoryLoad(load: HistoryLoad) {
  const loads = historyLoadsByConversationID.get(load.conversationID)
  loads?.delete(load)
  if (!loads?.size) historyLoadsByConversationID.delete(load.conversationID)
  pruneUnneededRealtimeUpdates()
}

function pruneUnneededRealtimeUpdates() {
  for (const [messageID, update] of realtimeUpdatesByMessageID) {
    if (!pendingLocalEditConversationIDsByMessageID.has(messageID) && !realtimeUpdateNeededByHistoryLoad(update.message.conversation_id, update.generation)) {
      realtimeUpdatesByMessageID.delete(messageID)
    }
  }
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetcher(url, init)
  if (!response.ok) {
    const body = await response.json().catch(() => ({})) as { error?: string }
    throw new Error(body.error || `Request failed (${response.status})`)
  }
  return response.status === 204 ? undefined as T : response.json() as Promise<T>
}

async function initialise() {
  try {
    me.value = await request<Principal>('/api/me')
  } catch { return }
  const organisations = await request<Organisation[]>('/api/organisations')
  organisation.value = organisations[0] ?? null
  if (!organisation.value) return
  conversations.value = await request<Conversation[]>(`/api/organisations/${organisation.value.id}/conversations?include_archived=true`)
  const firstActive = conversations.value.find(conversation => !conversation.archived)
  if (firstActive) await selectConversation(firstActive)
  try { users.value = await request<User[]>(`/api/organisations/${organisation.value.id}/users`) } catch { /* Settings can retry a transient roster failure. */ }
  realtime.connect()
}

async function login() {
  error.value = ''; busy.value = true
  try {
    me.value = await request<Principal>('/api/session', jsonInit('POST', { email: email.value, password: password.value }))
    const organisations = await request<Organisation[]>('/api/organisations')
    organisation.value = organisations[0] ?? null
    if (organisation.value) {
      conversations.value = await request<Conversation[]>(`/api/organisations/${organisation.value.id}/conversations?include_archived=true`)
      const firstActive = conversations.value.find(conversation => !conversation.archived)
      if (firstActive) await selectConversation(firstActive)
      try { users.value = await request<User[]>(`/api/organisations/${organisation.value.id}/users`) } catch { /* Settings can retry a transient roster failure. */ }
      realtime.connect()
    }
  } catch (cause) { error.value = messageOf(cause) } finally { busy.value = false }
}

async function selectConversation(conversation: Conversation) {
  navigationIntentGeneration++
  const selectionGeneration = ++conversationSelectionGeneration
  const loadGeneration = ++messageLoadGeneration
  selected.value = conversation
  replyingTo.value = null
  images.value = []
  pendingMessageClientID.value = ''
  messages.value = []
  hasNewerMessages.value = false
  loadingConversationID = conversation.id
  pendingRealtimeMessages = []
  const readSequence = conversation.read_sequence ?? 0
  const latestSequence = conversation.latest_sequence ?? readSequence
  const messageURL = latestSequence > readSequence
    ? `/api/conversations/${conversation.id}/messages?limit=100&after_sequence=${readSequence}`
    : `/api/conversations/${conversation.id}/messages?limit=100`
  const historyLoad = beginHistoryLoad(conversation.id)
  let selectedMessages: Message[]
  let selectedPageMessageCount = 0
  let selectedPageLatestSequence = readSequence
  try {
    selectedMessages = await request<Message[]>(messageURL)
    selectedPageMessageCount = selectedMessages.length
    selectedPageLatestSequence = selectedMessages.at(-1)?.sequence ?? readSequence
    if (selectionGeneration !== conversationSelectionGeneration || selected.value?.id !== conversation.id || !conversations.value.some(({ id }) => id === conversation.id)) {
      finishHistoryLoad(historyLoad)
      return
    }
    const firstUnreadMessageID = latestSequence > readSequence ? selectedMessages[0]?.id : undefined
    if (firstUnreadMessageID) {
      try {
        const recentHistory = await request<Message[]>(`/api/conversations/${conversation.id}/messages?limit=100&before=${encodeURIComponent(firstUnreadMessageID)}`)
        selectedMessages = mergeMessages(recentHistory, selectedMessages)
      } catch {
        // Keep the successfully loaded unread page when optional history is unavailable.
      }
    }
  } catch (cause) {
    if (loadGeneration === messageLoadGeneration) {
      loadingConversationID = null
      pendingRealtimeMessages = []
    }
    finishHistoryLoad(historyLoad)
    throw cause
  }
  if (selectionGeneration !== conversationSelectionGeneration || selected.value?.id !== conversation.id || !conversations.value.some(({ id }) => id === conversation.id)) {
    finishHistoryLoad(historyLoad)
    return
  }
  const realtimeMessages = loadGeneration === messageLoadGeneration ? pendingRealtimeMessages : []
  if (loadGeneration === messageLoadGeneration) {
    loadingConversationID = null
    pendingRealtimeMessages = []
  }
  const currentLatestSequence = conversations.value.find(({ id }) => id === conversation.id)?.latest_sequence ?? latestSequence
  const boundedPageMayHaveGap = selectedPageMessageCount === 100 && selectedPageLatestSequence < currentLatestSequence
  messages.value = applyRealtimeUpdates(boundedPageMayHaveGap ? selectedMessages : mergeMessages(selectedMessages, realtimeMessages), historyLoad)
  finishHistoryLoad(historyLoad)
  hasNewerMessages.value = (messages.value.at(-1)?.sequence ?? readSequence) < currentLatestSequence
  realtime.seed(messages.value)
  activeView.value = 'chat'
}

async function jumpToLatest() {
  const conversation = selected.value
  if (!conversation || !hasNewerMessages.value) return
  const selectionGeneration = conversationSelectionGeneration
  const loadGeneration = ++messageLoadGeneration
  loadingConversationID = conversation.id
  pendingRealtimeMessages = []
  const historyLoad = beginHistoryLoad(conversation.id)
  try {
    const latestMessages = await request<Message[]>(`/api/conversations/${conversation.id}/messages?limit=100`)
    if (selectionGeneration !== conversationSelectionGeneration || selected.value?.id !== conversation.id) return
    const realtimeMessages = loadGeneration === messageLoadGeneration ? pendingRealtimeMessages : []
    messages.value = applyRealtimeUpdates(mergeMessages(latestMessages, realtimeMessages), historyLoad)
    const currentLatestSequence = conversations.value.find(({ id }) => id === conversation.id)?.latest_sequence ?? conversation.latest_sequence ?? 0
    hasNewerMessages.value = (messages.value.at(-1)?.sequence ?? 0) < currentLatestSequence
    realtime.seed(messages.value)
    jumpToLatestVersion.value++
  } catch (cause) {
    if (selectionGeneration === conversationSelectionGeneration && selected.value?.id === conversation.id) error.value = messageOf(cause)
  } finally {
    finishHistoryLoad(historyLoad)
    if (loadGeneration === messageLoadGeneration) {
      loadingConversationID = null
      pendingRealtimeMessages = []
    }
  }
}

function mergeMessages(loadedMessages: Message[], realtimeMessages: Message[]) {
  const messagesByID = new Map(loadedMessages.map(message => [message.id, message]))
  for (const message of realtimeMessages) messagesByID.set(message.id, message)
  return [...messagesByID.values()].sort((left, right) => left.sequence - right.sequence)
}

function applyRealtimeUpdates(loadedMessages: Message[], historyLoad: HistoryLoad) {
  let updatedMessages = loadedMessages
  for (const { generation, message: updated } of realtimeUpdatesByMessageID.values()) {
    if (updated.conversation_id !== historyLoad.conversationID || generation <= historyLoad.generationBeforeRequest) continue
    updatedMessages = updatedMessages.map(message => message.id === updated.id ? updated : message.reply_to?.id === updated.id ? { ...message, reply_to: { ...message.reply_to, body: updated.body } } : message)
  }
  return updatedMessages
}

function pruneRealtimeUpdatesForConversation(conversationID: string) {
  historyLoadsByConversationID.delete(conversationID)
  for (const [messageID, pendingConversationID] of pendingLocalEditConversationIDsByMessageID) {
    if (pendingConversationID === conversationID) pendingLocalEditConversationIDsByMessageID.delete(messageID)
  }
  for (const [messageID, update] of realtimeUpdatesByMessageID) {
    if (update.message.conversation_id === conversationID) realtimeUpdatesByMessageID.delete(messageID)
  }
}

function updateComposerDraft(value: string) {
  if (value !== composer.value) pendingMessageClientID.value = ''
  composer.value = value
}

function updateImagesDraft(value: File[]) {
  if (value.length !== images.value.length || value.some((image, index) => image !== images.value[index])) pendingMessageClientID.value = ''
  images.value = value
}

function selectReply(message: Message) {
  if (message.id !== replyingTo.value?.id) pendingMessageClientID.value = ''
  replyingTo.value = message
}

function cancelReply() {
  if (replyingTo.value) pendingMessageClientID.value = ''
  replyingTo.value = null
}

async function sendMessage() {
  if ((!composer.value.trim() && !images.value.length) || !selected.value || selected.value.archived || busy.value) return
  const body = composer.value
  const selectedImages = images.value
  const selectedReply = replyingTo.value
  let conversationID = selected.value.id
  const selectionGeneration = conversationSelectionGeneration
  const draftConversation = conversationID.startsWith('draft:') ? selected.value : null
  const isCurrentConversation = () => selectionGeneration === conversationSelectionGeneration && selected.value?.id === conversationID && conversations.value.some(({ id }) => id === conversationID)
  const isCurrentSendTarget = () => isCurrentConversation() || Boolean(draftConversation && selectionGeneration === conversationSelectionGeneration && selected.value?.id === draftConversation.id)
  const clientID = pendingMessageClientID.value || crypto.randomUUID()
  composer.value = ''; images.value = []; busy.value = true; error.value = ''
  try {
    if (draftConversation) {
      const conversation = await request<Conversation>(`/api/organisations/${organisation.value!.id}/conversations`, jsonInit('POST', {
        name: draftConversation.name,
        visibility: 'members',
        member_ids: draftConversation.member_ids?.filter(userID => userID !== me.value?.id) ?? [],
        automatic_title: true,
        client_id: draftConversation.id.slice('draft:'.length),
      }))
      conversationRefreshGeneration++
      const existingConversation = conversations.value.find(({ id }) => id === conversation.id)
      if (existingConversation) Object.assign(existingConversation, conversation)
      else conversations.value.push(conversation)
      const persistedConversation = existingConversation ?? conversation
      if (selectionGeneration === conversationSelectionGeneration && selected.value?.id === draftConversation.id) selected.value = persistedConversation
      conversationID = persistedConversation.id
    }
    let init: RequestInit
    if (selectedImages.length) {
      const form = new FormData()
      form.set('body', body)
      form.set('client_id', clientID)
      if (selectedReply) form.set('reply_to_message_id', selectedReply.id)
      selectedImages.forEach(image => form.append('image', image))
      init = { method: 'POST', body: form }
    } else {
      init = jsonInit('POST', { body, client_id: clientID, reply_to_message_id: selectedReply?.id })
    }
    const message = await request<Message>(`/api/conversations/${conversationID}/messages`, init)
    if (pendingMessageClientID.value === clientID) pendingMessageClientID.value = ''
    if (replyingTo.value?.id === selectedReply?.id) replyingTo.value = null
    if (isCurrentConversation()) {
      selected.value!.archived = false
      selected.value!.read_sequence = Math.max(selected.value!.read_sequence ?? 0, message.sequence)
      selected.value!.latest_sequence = Math.max(selected.value!.latest_sequence ?? 0, message.sequence)
      selected.value!.activity_at = message.created_at
      if (selected.value!.title_automatic && message.body.trim()) {
        void reconcileAutomaticConversationTitle(selected.value!.id)
        selected.value!.name = message.body.trim()
        selected.value!.title_automatic = false
      }
      if (hasNewerMessages.value) await jumpToLatest()
      else if (!messages.value.some(({ id }) => id === message.id)) messages.value.push(message)
    }
  } catch (cause) {
    if (isCurrentSendTarget()) {
      pendingMessageClientID.value = clientID
      composer.value = body
      images.value = selectedImages
      error.value = messageOf(cause)
    }
  } finally { busy.value = false }
}

async function editMessage(messageID: string, body: string) {
  if (!selected.value || busy.value) return
  const conversationID = selected.value.id
  const realtimeUpdateGenerationBeforeRequest = realtimeUpdateGeneration
  pendingLocalEditConversationIDsByMessageID.set(messageID, conversationID)
  busy.value = true
  error.value = ''
  try {
    const updated = await request<Message>(`/api/conversations/${conversationID}/messages/${messageID}`, jsonInit('PUT', { body }))
    if (selected.value?.id === conversationID && (realtimeUpdatesByMessageID.get(messageID)?.generation ?? 0) <= realtimeUpdateGenerationBeforeRequest) {
      replaceMessageAndReplyPreviews(updated)
    }
  } catch (cause) {
    if (selected.value?.id === conversationID) error.value = messageOf(cause)
  } finally {
    pendingLocalEditConversationIDsByMessageID.delete(messageID)
    pruneUnneededRealtimeUpdates()
    busy.value = false
  }
}

async function markReadThrough(sequence: number) {
  const conversation = selected.value
  if (!conversation || sequence <= (conversation.read_sequence ?? 0)) return
  try {
    const persisted = await request<{ sequence: number }>(`/api/conversations/${conversation.id}/read`, jsonInit('PUT', { sequence }))
    const currentConversation = conversations.value.find(({ id }) => id === conversation.id)
    if (currentConversation) currentConversation.read_sequence = Math.max(currentConversation.read_sequence ?? 0, persisted.sequence)
    if (selected.value?.id === conversation.id) selected.value.read_sequence = Math.max(selected.value.read_sequence ?? 0, persisted.sequence)
  } catch (cause) {
    if (selected.value?.id === conversation.id) error.value = messageOf(cause)
  }
}

async function createBot() {
  if (!organisation.value || !botName.value.trim()) return
  try {
    const bot = await request<User & { api_key: string }>(`/api/organisations/${organisation.value.id}/bots`, jsonInit('POST', { name: botName.value }))
    users.value.push(bot)
    apiKey.value = bot.api_key; keyAction.value = 'created'; copied.value = false; keySaved.value = false; keyError.value = ''; addUserStep.value = 'key'; botName.value = ''
  } catch (cause) { error.value = messageOf(cause) }
}

async function openOrganisation() {
  if (!organisation.value) return
  navigationIntentGeneration++
  error.value = ''
  try {
    users.value = await request<User[]>(`/api/organisations/${organisation.value.id}/users`)
    eligibleUsers.value = []
    eligibleEmail.value = ''
    notice.value = ''
    removingBotID.value = ''
    showAddExisting.value = false
    activeView.value = 'settings'
  } catch (cause) { error.value = messageOf(cause) }
}

async function searchExistingUser() {
  if (!organisation.value || !eligibleEmail.value.trim()) return
  error.value = ''
  notice.value = ''
  try {
    eligibleUsers.value = await request<EligibleUser[]>(`/api/organisations/${organisation.value.id}/eligible-users?email=${encodeURIComponent(eligibleEmail.value.trim())}`)
    selectedEligibleUserID.value = eligibleUsers.value[0]?.id ?? ''
    if (!eligibleUsers.value.length) notice.value = 'No eligible user found with that email'
  } catch (cause) { error.value = messageOf(cause) }
}

async function addExistingUser(userID = selectedEligibleUserID.value) {
  if (!organisation.value || !userID) return
  selectedEligibleUserID.value = userID
  try {
    const user = await request<User>(`/api/organisations/${organisation.value.id}/users`, jsonInit('POST', { user_id: selectedEligibleUserID.value }))
    users.value.push(user)
    eligibleUsers.value = eligibleUsers.value.filter(candidate => candidate.id !== user.id)
    selectedEligibleUserID.value = ''
    eligibleEmail.value = ''
    showAddExisting.value = false
    notice.value = `${user.name} added as a member`
  } catch (cause) { error.value = messageOf(cause) }
}

async function rotateBotKey(bot: User) {
  if (botMutationID.value || !window.confirm(`Rotate ${bot.name}'s key? Its current key will stop working immediately.`)) return
  botMutationID.value = bot.id
  error.value = ''
  try {
    const result = await request<{ api_key: string }>(`/api/bots/${bot.id}/key`, jsonInit('POST'))
    apiKey.value = result.api_key
    keyAction.value = 'rotated'
    copied.value = false
    keySaved.value = false
    keyError.value = ''
    addUserStep.value = 'key'
  } catch (cause) { error.value = messageOf(cause) } finally { botMutationID.value = '' }
}

async function revokeBotKey(bot: User) {
  if (botMutationID.value || !window.confirm(`Revoke ${bot.name}'s key? The bot will be disconnected immediately and will need a new key to reconnect.`)) return
  botMutationID.value = bot.id
  try {
    await request<void>(`/api/bots/${bot.id}/key`, jsonInit('DELETE'))
    notice.value = `${bot.name} key revoked`
  } catch (cause) { error.value = messageOf(cause) } finally { botMutationID.value = '' }
}

async function removeBot(bot: User) {
  if (!organisation.value || botMutationID.value) return
  botMutationID.value = bot.id
  try {
    await request<void>(`/api/organisations/${organisation.value.id}/bots/${bot.id}`, jsonInit('DELETE'))
    users.value = users.value.filter(user => user.id !== bot.id)
    await refreshConversations()
    removingBotID.value = ''
    notice.value = `${bot.name} removed`
  } catch (cause) { error.value = messageOf(cause) } finally { botMutationID.value = '' }
}

async function copyKey() {
  try {
    await navigator.clipboard.writeText(apiKey.value)
    copied.value = true
    keyError.value = ''
  } catch {
    copied.value = false
    keyError.value = 'Copy failed. Select the visible key and copy it manually.'
  }
}

function closeAddUser() {
  if (addUserStep.value === 'key' && !keySaved.value) return
  addUserStep.value = ''
}

function beginConversationDialog() {
  conversationCreationGeneration++
  creatingConversation.value = false
}

function closeConversationDialog() {
  beginConversationDialog()
  showConversation.value = false
}

async function openConversationDialog() {
  if (!organisation.value) return
  const dialogIntentGeneration = ++navigationIntentGeneration
  const conversationUsers = await request<User[]>(`/api/organisations/${organisation.value.id}/users`)
  if (dialogIntentGeneration !== navigationIntentGeneration) return
  users.value = conversationUsers
  beginConversationDialog()
  conversationName.value = ''
  conversationNameEdited.value = false
  memberIDs.value = []
  showConversation.value = true
}

watch(memberIDs, selectedMemberIDs => {
  if (conversationNameEdited.value) return
  const selectedMemberIDSet = new Set(selectedMemberIDs)
  conversationName.value = users.value.filter(user => selectedMemberIDSet.has(user.id)).map(user => user.name).join(', ')
})

function openDirectTopicDialog(user: User) {
  navigationIntentGeneration++
  startTopicDraft(`New ${user.name} session`, [user.id])
}

function openTopicDialog(conversation: Conversation) {
  if (!me.value || conversation.visibility !== 'members') return
  navigationIntentGeneration++
  const participantIDs = (conversation.member_ids ?? []).filter(userID => userID !== me.value!.id)
  startTopicDraft(`New ${conversation.name} session`, participantIDs)
}

function startTopicDraft(name: string, participantIDs: string[]) {
  if (!me.value) return
  conversationSelectionGeneration++
  messageLoadGeneration++
  loadingConversationID = null
  pendingRealtimeMessages = []
  selected.value = {
    id: `draft:${crypto.randomUUID()}`,
    name,
    visibility: 'members',
    member_ids: [me.value.id, ...participantIDs],
    title_automatic: true,
  }
  messages.value = []
  hasNewerMessages.value = false
  replyingTo.value = null
  composer.value = ''
  images.value = []
  pendingMessageClientID.value = ''
  error.value = ''
  activeView.value = 'chat'
}

async function selectDirectUser(user: User) {
  if (!organisation.value || pendingDirectUserIDs.has(user.id)) return
  const directIntentGeneration = ++navigationIntentGeneration
  const existingConversation = conversations.value
    .filter(conversation =>
      conversation.visibility === 'members' &&
      conversation.member_ids?.length === 2 &&
      conversation.member_ids.includes(me.value!.id) &&
      conversation.member_ids.includes(user.id),
    )
    .sort((left, right) =>
      (right.latest_sequence ?? 0) - (left.latest_sequence ?? 0)
      || conversations.value.indexOf(right) - conversations.value.indexOf(left),
    )[0]
  if (existingConversation) {
    await selectConversation(existingConversation)
    return
  }
  pendingDirectUserIDs.add(user.id)
  error.value = ''
  try {
    const conversation = await request<Conversation>(`/api/organisations/${organisation.value.id}/direct-conversations/${user.id}`, jsonInit('POST'))
    if (!conversations.value.some(({ id }) => id === conversation.id)) conversations.value.push(conversation)
    if (directIntentGeneration === navigationIntentGeneration) await selectConversation(conversation)
  } catch (cause) {
    if (directIntentGeneration === navigationIntentGeneration) error.value = messageOf(cause)
  } finally {
    pendingDirectUserIDs.delete(user.id)
  }
}

async function createConversation() {
  const name = conversationName.value.trim()
  if (!organisation.value || !name || memberIDs.value.length < 2 || creatingConversation.value) return
  const creationGeneration = conversationCreationGeneration
  const organisationID = organisation.value.id
  const submittedMemberIDs = [...memberIDs.value]
  creatingConversation.value = true
  try {
    const conversation = await request<Conversation>(`/api/organisations/${organisationID}/conversations`, jsonInit('POST', { name, visibility: 'members', member_ids: submittedMemberIDs }))
    if (!conversations.value.some(({ id }) => id === conversation.id)) conversations.value.push(conversation)
    if (creationGeneration !== conversationCreationGeneration) return
    showConversation.value = false
    conversationName.value = ''
    memberIDs.value = []
    await selectConversation(conversation)
  } catch (cause) {
    if (creationGeneration === conversationCreationGeneration) error.value = messageOf(cause)
  } finally {
    if (creationGeneration === conversationCreationGeneration) creatingConversation.value = false
  }
}

async function updateConversationTitle(name: string) {
  const conversation = selected.value
  if (!conversation || titleSavingConversationIDs.value.has(conversation.id)) return
  titleSavingConversationIDs.value = new Set(titleSavingConversationIDs.value).add(conversation.id)
  try {
    const updated = await request<Pick<Conversation, 'name' | 'title_automatic'>>(`/api/conversations/${conversation.id}/title`, jsonInit('PUT', { name }))
    conversationRefreshGeneration++
    const current = conversations.value.find(({ id }) => id === conversation.id)
    if (current) {
      current.name = updated.name
      current.title_automatic = false
    }
    if (selected.value?.id === conversation.id) {
      selected.value.name = updated.name
      selected.value.title_automatic = false
    }
  } catch (cause) {
    if (selected.value?.id === conversation.id) error.value = messageOf(cause)
  } finally {
    const savingConversationIDs = new Set(titleSavingConversationIDs.value)
    savingConversationIDs.delete(conversation.id)
    titleSavingConversationIDs.value = savingConversationIDs
  }
}

async function setSelectedConversationArchived(archived: boolean) {
  const conversation = selected.value
  if (!conversation || conversation.id.startsWith('draft:') || busy.value) return
  busy.value = true
  error.value = ''
  try {
    await request<void>(`/api/conversations/${conversation.id}/archive`, jsonInit(archived ? 'PUT' : 'DELETE'))
    await reconcileConversationArchiveState(conversation.id)
  } catch (cause) {
    if (selected.value?.id === conversation.id) error.value = messageOf(cause)
  } finally {
    busy.value = false
  }
}

async function archiveConversationsInBulk(selectedConversations: Conversation[]) {
  if (busy.value) return
  const activeConversations = selectedConversations.filter(conversation => !conversation.archived && !conversation.id.startsWith('draft:'))
  if (!activeConversations.length) return
  busy.value = true
  error.value = ''
  const results = await Promise.allSettled(activeConversations.map(conversation =>
    request<void>(`/api/conversations/${conversation.id}/archive`, jsonInit('PUT')),
  ))
  await refreshConversations()
  completedArchiveConversationIDs.value = activeConversations.filter((_conversation, index) => results[index].status === 'fulfilled').map(conversation => conversation.id)
  const failure = results.find(result => result.status === 'rejected')
  if (failure?.status === 'rejected') error.value = messageOf(failure.reason)
  busy.value = false
}

async function reconcileConversationArchiveState(conversationID: string) {
  if (!organisation.value) return
  const archiveStateGeneration = archiveStateGenerations.get(conversationID) ?? 0
  const currentConversations = await request<Conversation[]>(`/api/organisations/${organisation.value.id}/conversations?include_archived=true`)
  if ((archiveStateGenerations.get(conversationID) ?? 0) !== archiveStateGeneration) return
  const authoritativeConversation = currentConversations.find(conversation => conversation.id === conversationID)
  if (!authoritativeConversation) return
  archiveStateGenerations.set(conversationID, archiveStateGeneration + 1)
  const listedConversation = conversations.value.find(conversation => conversation.id === conversationID)
  if (listedConversation) listedConversation.archived = authoritativeConversation.archived
  if (selected.value?.id === conversationID) selected.value.archived = authoritativeConversation.archived
}

async function deleteSelectedConversation() {
  if (!organisation.value || organisation.value.role !== 'admin' || !selected.value || busy.value) return
  const conversation = selected.value
  const otherUserID = conversation.visibility === 'members' && conversation.member_ids?.length === 2
    ? conversation.member_ids.find(userID => userID !== me.value?.id)
    : undefined
  const otherUser = users.value.find(user => user.id === otherUserID)
  const removedDirectConversation = conversation.visibility === 'members' && conversation.member_ids?.length === 1 && conversation.member_ids[0] === me.value?.id
  const legacyPairName = `direct:${[...(conversation.member_ids ?? [])].sort().join(':')}`
  const directTopic = conversation.name === `direct:${conversation.id}` || conversation.name === legacyPairName ? 'General' : conversation.name
  const deleteName = otherUser ? `"${directTopic}" with ${otherUser.name}` : removedDirectConversation ? 'conversation with Removed user' : `#${conversation.name}`
  if (!window.confirm(`Delete ${deleteName} for everyone? This permanently removes all messages in this conversation.`)) return
  busy.value = true
  error.value = ''
  try {
    await request<void>(`/api/organisations/${organisation.value.id}/conversations/${conversation.id}`, jsonInit('DELETE'))
    await handleConversationDeleted(conversation.id)
  } catch (cause) { error.value = messageOf(cause) } finally { busy.value = false }
}

async function deleteConversationsInBulk(selectedConversations: Conversation[]) {
  if (!organisation.value || organisation.value.role !== 'admin' || busy.value || !selectedConversations.length) return
  if (!window.confirm(`Delete ${selectedConversations.length} conversations? This permanently removes their messages for everyone.`)) return
  busy.value = true
  error.value = ''
  const organisationID = organisation.value.id
  const results = await Promise.allSettled(selectedConversations.map(conversation =>
    request<void>(`/api/organisations/${organisationID}/conversations/${conversation.id}`, jsonInit('DELETE')),
  ))
  const deletedConversationIDs = new Set(selectedConversations.filter((_conversation, index) => results[index].status === 'fulfilled').map(conversation => conversation.id))
  for (const conversationID of deletedConversationIDs) pruneRealtimeUpdatesForConversation(conversationID)
  conversations.value = conversations.value.filter(conversation => !deletedConversationIDs.has(conversation.id))
  if (selected.value && deletedConversationIDs.has(selected.value.id)) {
    conversationSelectionGeneration++
    selected.value = null
    messages.value = []
    const firstActive = conversations.value.find(conversation => !conversation.archived)
    if (firstActive) {
      try { await selectConversation(firstActive) } catch (cause) { error.value = messageOf(cause) }
    }
  }
  const failure = results.find(result => result.status === 'rejected')
  if (failure?.status === 'rejected') error.value = messageOf(failure.reason)
  busy.value = false
}

async function handleConversationDeleted(conversationID: string) {
  pruneRealtimeUpdatesForConversation(conversationID)
  if (!conversations.value.some(conversation => conversation.id === conversationID)) return
  conversations.value = conversations.value.filter(conversation => conversation.id !== conversationID)
  if (selected.value?.id !== conversationID) return
  conversationSelectionGeneration++
  selected.value = null
  messages.value = []
  try {
    const firstActive = conversations.value.find(conversation => !conversation.archived)
    if (firstActive) await selectConversation(firstActive)
  } catch (cause) { error.value = messageOf(cause) }
}

async function reconcileDeletedConversation(conversationID: string) {
  await handleConversationDeleted(conversationID)
  await refreshConversations()
}

async function reconcileAutomaticConversationTitle(conversationID: string) {
  if (pendingAutomaticTitleConversationIDs.has(conversationID)) return
  pendingAutomaticTitleConversationIDs.add(conversationID)
  try {
    await refreshConversations()
  } finally {
    pendingAutomaticTitleConversationIDs.delete(conversationID)
  }
}

async function refreshConversations() {
  if (!organisation.value) return
  const refreshGeneration = ++conversationRefreshGeneration
  const archiveStateGenerationsAtStart = new Map(archiveStateGenerations)
  try {
    const currentConversations = await request<Conversation[]>(`/api/organisations/${organisation.value.id}/conversations?include_archived=true`)
    if (refreshGeneration !== conversationRefreshGeneration) return
    const refreshedSelected = currentConversations.find(conversation => conversation.id === selected.value?.id)
    for (const refreshedConversation of currentConversations) {
      const localConversation = conversations.value.find(conversation => conversation.id === refreshedConversation.id)
      if (!localConversation) continue
      if ((archiveStateGenerations.get(refreshedConversation.id) ?? 0) !== (archiveStateGenerationsAtStart.get(refreshedConversation.id) ?? 0)) {
        refreshedConversation.archived = localConversation.archived
      }
      refreshedConversation.read_sequence = Math.max(refreshedConversation.read_sequence ?? 0, localConversation.read_sequence ?? 0)
      if ((localConversation.latest_sequence ?? 0) > (refreshedConversation.latest_sequence ?? 0)) {
        refreshedConversation.latest_sequence = localConversation.latest_sequence
        refreshedConversation.activity_at = localConversation.activity_at
        refreshedConversation.name = localConversation.name
        refreshedConversation.title_automatic = localConversation.title_automatic
      }
    }
    if (selected.value?.id.startsWith('draft:')) {
      conversations.value = currentConversations
      return
    }
    if (refreshedSelected && selected.value) {
      refreshedSelected.read_sequence = Math.max(refreshedSelected.read_sequence ?? 0, selected.value.read_sequence ?? 0)
      refreshedSelected.latest_sequence = Math.max(refreshedSelected.latest_sequence ?? 0, selected.value.latest_sequence ?? 0)
    }
    conversations.value = currentConversations
    if (refreshedSelected) {
      selected.value = refreshedSelected
      return
    }
    conversationSelectionGeneration++
    selected.value = null
    messages.value = []
    const firstActive = conversations.value.find(conversation => !conversation.archived)
    if (firstActive) await selectConversation(firstActive)
  } catch (cause) {
    if (refreshGeneration === conversationRefreshGeneration) error.value = messageOf(cause)
  }
}

function jsonInit(method: string, body?: unknown): RequestInit {
  return { method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) }
}
function messageOf(cause: unknown) { return cause instanceof Error ? cause.message : 'Something went wrong' }
onMounted(initialise)
onBeforeUnmount(() => {
  realtime.disconnect()
  for (const timer of activityTimers.values()) clearTimeout(timer)
  activityTimers.clear()
})
</script>

<template>
  <main v-if="!me" class="login-shell">
    <form class="card login-card" @submit.prevent="login">
      <p class="product-name">K-Mainstay</p><h1>Welcome back</h1><p class="muted">Sign in to your workspace.</p>
      <label>Email<input v-model="email" type="email" autocomplete="email" required></label>
      <label>Password<input v-model="password" type="password" autocomplete="current-password" required></label>
      <p v-if="error" class="error" role="alert">{{ error }}</p><button :disabled="busy">Sign in</button>
    </form>
  </main>
  <main v-else class="workspace">
    <WorkspaceSidebar :organisation="organisation" :principal="me" :conversations="conversations" :users="users" :selected="selected" :settings-active="activeView === 'settings'" :busy="busy" :completed-archive-conversation-i-ds="completedArchiveConversationIDs" @open-settings="openOrganisation" @new-conversation="openConversationDialog" @new-direct-topic="openDirectTopicDialog" @new-topic="openTopicDialog" @select-direct-user="selectDirectUser" @select-conversation="selectConversation" @archive-conversations="archiveConversationsInBulk" @delete-conversations="deleteConversationsInBulk" />
    <ConversationView v-if="activeView === 'chat'" :composer="composer" :images="images" :activities="selectedActivities" :replying-to="replyingTo" :selected="selected" :messages="messages" :users="users" :current-user-i-d="me.id" :busy="busy" :title-busy="titleSavingConversationIDs.has(selected?.id ?? '')" :error="error" :can-delete="organisation?.role === 'admin' && !selected?.is_everyone && (selected?.visibility === 'organisation' || (selected?.member_ids?.length ?? 0) >= 3)" :has-newer-messages="hasNewerMessages" :jump-to-latest-version="jumpToLatestVersion" @update:composer="updateComposerDraft" @update:images="updateImagesDraft" @update-title="updateConversationTitle" @edit-message="editMessage" @reply-to="selectReply" @cancel-reply="cancelReply" @archive-conversation="setSelectedConversationArchived(true)" @restore-conversation="setSelectedConversationArchived(false)" @delete-conversation="deleteSelectedConversation" @new-topic="selected && openTopicDialog(selected)" @send-message="sendMessage" @read-through="markReadThrough" @jump-to-latest="jumpToLatest" />
    <OrganisationSettings v-else v-model:eligible-email="eligibleEmail" :organisation="organisation" :users="users" :eligible-users="eligibleUsers" :show-add-existing="showAddExisting" :notice="notice" :error="error" :bot-mutation-i-d="botMutationID" :removing-bot-i-d="removingBotID" @back="activeView = 'chat'" @toggle-add-existing="showAddExisting = !showAddExisting" @search-existing-user="searchExistingUser" @add-existing-user="addExistingUser" @add-bot="addUserStep = 'bot'" @rotate-key="rotateBotKey" @revoke-key="revokeBotKey" @begin-remove-bot="removingBotID = $event" @cancel-remove-bot="removingBotID = ''" @remove-bot="removeBot" />
  </main>

  <BaseDialog v-if="addUserStep" labelledby="bot-dialog-title" :close-disabled="addUserStep === 'key' && !keySaved" :focus-key="addUserStep" @close="closeAddUser">
    <button class="close" aria-label="Close" :disabled="addUserStep === 'key' && !keySaved" @click="closeAddUser">×</button>
    <form v-if="addUserStep === 'bot'" data-testid="create-bot" @submit.prevent="createBot"><p class="dialog-context">New bot</p><h2 id="bot-dialog-title">Name your bot</h2><label>Name<input data-testid="bot-name" v-model="botName" data-dialog-initial-focus required placeholder="e.g. Hector"></label><button>Create bot</button></form>
    <template v-else><p class="dialog-context">{{ keyAction === 'created' ? 'Bot created' : 'Key rotated' }}</p><h2 id="bot-dialog-title">{{ keyAction === 'created' ? 'Copy this key now' : 'Copy the rotated key now' }}</h2><p class="warning">This API key is shown only once. Store it somewhere secure before closing.</p><code class="api-key">{{ apiKey }}</code><button data-testid="copy-key" data-dialog-initial-focus @click="copyKey">{{ copied ? 'Copied' : 'Copy API key' }}</button><p v-if="keyError" class="error" role="alert">{{ keyError }}</p><label class="check key-saved-confirmation"><input data-testid="key-saved" v-model="keySaved" type="checkbox"><span>I saved this key securely</span></label></template>
  </BaseDialog>

  <BaseDialog v-if="showConversation" labelledby="conversation-dialog-title" @close="closeConversationDialog">
    <div class="dialog-header"><div><p class="dialog-context">New group chat</p><h2 id="conversation-dialog-title">Create group chat</h2></div><button data-testid="close-conversation-dialog" type="button" class="close" aria-label="Close" @click="closeConversationDialog">×</button></div>
    <form data-testid="create-conversation" class="group-chat-form" autocomplete="off" data-1p-ignore="true" data-lpignore="true" data-bwignore="true" @submit.prevent="createConversation"><label>Name<input data-testid="conversation-name" v-model="conversationName" name="group_chat_title" autocomplete="off" data-1p-ignore="true" data-lpignore="true" data-bwignore="true" data-dialog-initial-focus required placeholder="e.g. Planning" @input="conversationNameEdited = true"></label><fieldset><legend>People (select at least two)</legend><label v-for="user in users.filter(u => u.id !== me?.id)" :key="user.id" class="check"><input v-model="memberIDs" type="checkbox" :value="user.id"><span>{{ user.name }} <small>{{ user.kind }}</small></span></label></fieldset><button type="submit" :disabled="creatingConversation || !conversationName.trim() || memberIDs.length < 2">{{ creatingConversation ? 'Creating…' : 'Create group chat' }}</button></form>
  </BaseDialog>
</template>
