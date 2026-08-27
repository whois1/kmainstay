<script setup lang="ts">
import { inject, onBeforeUnmount, onMounted, ref } from 'vue'
import BaseDialog from './components/BaseDialog.vue'
import ConversationView from './components/ConversationView.vue'
import OrganisationSettings from './components/OrganisationSettings.vue'
import WorkspaceSidebar from './components/WorkspaceSidebar.vue'
import { useRealtime } from './composables/realtime'
import type { Conversation, EligibleUser, Message, Organisation, Principal, User } from './types'

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
let conversationCreationGeneration = 0
const conversationName = ref('')
const users = ref<User[]>([])
const eligibleUsers = ref<EligibleUser[]>([])
const eligibleEmail = ref('')
const selectedEligibleUserID = ref('')
const showAddExisting = ref(false)
const memberIDs = ref<string[]>([])
let conversationRefreshGeneration = 0
let conversationSelectionGeneration = 0
let navigationIntentGeneration = 0
let messageLoadGeneration = 0
let loadingConversationID: string | null = null
let pendingRealtimeMessages: Message[] = []
const pendingDirectUserIDs = new Set<string>()
const archiveStateGenerations = new Map<string, number>()

const realtime = useRealtime((message) => {
  const conversation = conversations.value.find(({ id }) => id === message.conversation_id)
  if (conversation) {
    archiveStateGenerations.set(conversation.id, (archiveStateGenerations.get(conversation.id) ?? 0) + 1)
    conversation.archived = false
    conversation.latest_sequence = Math.max(conversation.latest_sequence ?? 0, message.sequence)
    conversation.activity_at = message.created_at
    if (conversation.title_automatic && message.body.trim()) {
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
}, Socket, conversationID => { void reconcileDeletedConversation(conversationID) }, refreshConversations)

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
  let selectedMessages: Message[]
  let selectedPageMessageCount = 0
  let selectedPageLatestSequence = readSequence
  try {
    selectedMessages = await request<Message[]>(messageURL)
    selectedPageMessageCount = selectedMessages.length
    selectedPageLatestSequence = selectedMessages.at(-1)?.sequence ?? readSequence
    if (selectionGeneration !== conversationSelectionGeneration || selected.value?.id !== conversation.id || !conversations.value.some(({ id }) => id === conversation.id)) return
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
    throw cause
  }
  if (selectionGeneration !== conversationSelectionGeneration || selected.value?.id !== conversation.id || !conversations.value.some(({ id }) => id === conversation.id)) return
  const realtimeMessages = loadGeneration === messageLoadGeneration ? pendingRealtimeMessages : []
  if (loadGeneration === messageLoadGeneration) {
    loadingConversationID = null
    pendingRealtimeMessages = []
  }
  const currentLatestSequence = conversations.value.find(({ id }) => id === conversation.id)?.latest_sequence ?? latestSequence
  const boundedPageMayHaveGap = selectedPageMessageCount === 100 && selectedPageLatestSequence < currentLatestSequence
  messages.value = boundedPageMayHaveGap ? selectedMessages : mergeMessages(selectedMessages, realtimeMessages)
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
  try {
    const latestMessages = await request<Message[]>(`/api/conversations/${conversation.id}/messages?limit=100`)
    if (selectionGeneration !== conversationSelectionGeneration || selected.value?.id !== conversation.id) return
    const realtimeMessages = loadGeneration === messageLoadGeneration ? pendingRealtimeMessages : []
    messages.value = mergeMessages(latestMessages, realtimeMessages)
    const currentLatestSequence = conversations.value.find(({ id }) => id === conversation.id)?.latest_sequence ?? conversation.latest_sequence ?? 0
    hasNewerMessages.value = (messages.value.at(-1)?.sequence ?? 0) < currentLatestSequence
    realtime.seed(messages.value)
    jumpToLatestVersion.value++
  } catch (cause) {
    if (selectionGeneration === conversationSelectionGeneration && selected.value?.id === conversation.id) error.value = messageOf(cause)
  } finally {
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
  memberIDs.value = []
  showConversation.value = true
}

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
  if (!window.confirm(`Delete ${deleteName}? This removes its messages and nobody will be able to send to it.`)) return
  busy.value = true
  error.value = ''
  try {
    await request<void>(`/api/organisations/${organisation.value.id}/conversations/${conversation.id}`, jsonInit('DELETE'))
    await handleConversationDeleted(conversation.id)
  } catch (cause) { error.value = messageOf(cause) } finally { busy.value = false }
}

async function handleConversationDeleted(conversationID: string) {
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
onBeforeUnmount(realtime.disconnect)
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
    <WorkspaceSidebar :organisation="organisation" :principal="me" :conversations="conversations" :users="users" :selected="selected" :settings-active="activeView === 'settings'" @open-settings="openOrganisation" @new-conversation="openConversationDialog" @new-direct-topic="openDirectTopicDialog" @new-topic="openTopicDialog" @select-direct-user="selectDirectUser" @select-conversation="selectConversation" />
    <ConversationView v-if="activeView === 'chat'" :composer="composer" :images="images" :replying-to="replyingTo" :selected="selected" :messages="messages" :users="users" :current-user-i-d="me.id" :busy="busy" :title-busy="titleSavingConversationIDs.has(selected?.id ?? '')" :error="error" :can-delete="organisation?.role === 'admin'" :has-newer-messages="hasNewerMessages" :jump-to-latest-version="jumpToLatestVersion" @update:composer="updateComposerDraft" @update:images="updateImagesDraft" @update-title="updateConversationTitle" @reply-to="selectReply" @cancel-reply="cancelReply" @archive-conversation="setSelectedConversationArchived(true)" @restore-conversation="setSelectedConversationArchived(false)" @delete-conversation="deleteSelectedConversation" @new-topic="selected && openTopicDialog(selected)" @send-message="sendMessage" @read-through="markReadThrough" @jump-to-latest="jumpToLatest" />
    <OrganisationSettings v-else v-model:eligible-email="eligibleEmail" :organisation="organisation" :users="users" :eligible-users="eligibleUsers" :show-add-existing="showAddExisting" :notice="notice" :error="error" :bot-mutation-i-d="botMutationID" :removing-bot-i-d="removingBotID" @back="activeView = 'chat'" @toggle-add-existing="showAddExisting = !showAddExisting" @search-existing-user="searchExistingUser" @add-existing-user="addExistingUser" @add-bot="addUserStep = 'bot'" @rotate-key="rotateBotKey" @revoke-key="revokeBotKey" @begin-remove-bot="removingBotID = $event" @cancel-remove-bot="removingBotID = ''" @remove-bot="removeBot" />
  </main>

  <BaseDialog v-if="addUserStep" labelledby="bot-dialog-title" :close-disabled="addUserStep === 'key' && !keySaved" :focus-key="addUserStep" @close="closeAddUser">
    <button class="close" aria-label="Close" :disabled="addUserStep === 'key' && !keySaved" @click="closeAddUser">×</button>
    <form v-if="addUserStep === 'bot'" data-testid="create-bot" @submit.prevent="createBot"><p class="dialog-context">New bot</p><h2 id="bot-dialog-title">Name your bot</h2><label>Name<input data-testid="bot-name" v-model="botName" data-dialog-initial-focus required placeholder="e.g. Hector"></label><button>Create bot</button></form>
    <template v-else><p class="dialog-context">{{ keyAction === 'created' ? 'Bot created' : 'Key rotated' }}</p><h2 id="bot-dialog-title">{{ keyAction === 'created' ? 'Copy this key now' : 'Copy the rotated key now' }}</h2><p class="warning">This API key is shown only once. Store it somewhere secure before closing.</p><code class="api-key">{{ apiKey }}</code><button data-testid="copy-key" data-dialog-initial-focus @click="copyKey">{{ copied ? 'Copied' : 'Copy API key' }}</button><p v-if="keyError" class="error" role="alert">{{ keyError }}</p><label class="check key-saved-confirmation"><input data-testid="key-saved" v-model="keySaved" type="checkbox"><span>I saved this key securely</span></label></template>
  </BaseDialog>

  <BaseDialog v-if="showConversation" labelledby="conversation-dialog-title" @close="closeConversationDialog">
    <form data-testid="create-conversation" @submit.prevent="createConversation"><button type="button" class="close" aria-label="Close" @click="closeConversationDialog">×</button><p class="dialog-context">New group chat</p><h2 id="conversation-dialog-title">Create group chat</h2><label>Name<input data-testid="conversation-name" v-model="conversationName" data-dialog-initial-focus required placeholder="e.g. Planning"></label><fieldset><legend>People (select at least two)</legend><label v-for="user in users.filter(u => u.id !== me?.id)" :key="user.id" class="check"><input v-model="memberIDs" type="checkbox" :value="user.id"><span>{{ user.name }} <small>{{ user.kind }}</small></span></label></fieldset><button type="submit" :disabled="creatingConversation || !conversationName.trim() || memberIDs.length < 2">{{ creatingConversation ? 'Creating…' : 'Create group chat' }}</button></form>
  </BaseDialog>
</template>
