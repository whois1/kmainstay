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
const conversationName = ref('')
const users = ref<User[]>([])
const eligibleUsers = ref<EligibleUser[]>([])
const eligibleEmail = ref('')
const selectedEligibleUserID = ref('')
const showAddExisting = ref(false)
const memberIDs = ref<string[]>([])
let conversationRefreshGeneration = 0
let conversationSelectionGeneration = 0

const realtime = useRealtime((message) => {
  const conversation = conversations.value.find(({ id }) => id === message.conversation_id)
  if (conversation) conversation.latest_sequence = Math.max(conversation.latest_sequence ?? 0, message.sequence)
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
  conversations.value = await request<Conversation[]>(`/api/organisations/${organisation.value.id}/conversations`)
  if (conversations.value[0]) await selectConversation(conversations.value[0])
  realtime.connect()
}

async function login() {
  error.value = ''; busy.value = true
  try {
    me.value = await request<Principal>('/api/session', jsonInit('POST', { email: email.value, password: password.value }))
    const organisations = await request<Organisation[]>('/api/organisations')
    organisation.value = organisations[0] ?? null
    if (organisation.value) {
      conversations.value = await request<Conversation[]>(`/api/organisations/${organisation.value.id}/conversations`)
      if (conversations.value[0]) await selectConversation(conversations.value[0])
      realtime.connect()
    }
  } catch (cause) { error.value = messageOf(cause) } finally { busy.value = false }
}

async function selectConversation(conversation: Conversation) {
  const selectionGeneration = ++conversationSelectionGeneration
  selected.value = conversation
  messages.value = []
  hasNewerMessages.value = false
  const readSequence = conversation.read_sequence ?? 0
  const latestSequence = conversation.latest_sequence ?? readSequence
  const messageURL = latestSequence > readSequence
    ? `/api/conversations/${conversation.id}/messages?limit=100&after_sequence=${readSequence}`
    : `/api/conversations/${conversation.id}/messages?limit=100`
  const selectedMessages = await request<Message[]>(messageURL)
  if (selectionGeneration !== conversationSelectionGeneration || selected.value?.id !== conversation.id || !conversations.value.some(({ id }) => id === conversation.id)) return
  messages.value = selectedMessages
  hasNewerMessages.value = Boolean(selectedMessages.length && selectedMessages.at(-1)!.sequence < latestSequence)
  realtime.seed(messages.value)
  activeView.value = 'chat'
}

async function jumpToLatest() {
  const conversation = selected.value
  if (!conversation || !hasNewerMessages.value) return
  const selectionGeneration = conversationSelectionGeneration
  try {
    const latestMessages = await request<Message[]>(`/api/conversations/${conversation.id}/messages?limit=100`)
    if (selectionGeneration !== conversationSelectionGeneration || selected.value?.id !== conversation.id) return
    messages.value = latestMessages
    hasNewerMessages.value = false
    realtime.seed(messages.value)
    jumpToLatestVersion.value++
  } catch (cause) {
    if (selectionGeneration === conversationSelectionGeneration && selected.value?.id === conversation.id) error.value = messageOf(cause)
  }
}

async function sendMessage() {
  if (!composer.value.trim() || !selected.value || busy.value) return
  const body = composer.value
  const conversationID = selected.value.id
  const selectionGeneration = conversationSelectionGeneration
  const isCurrentConversation = () => selectionGeneration === conversationSelectionGeneration && selected.value?.id === conversationID && conversations.value.some(({ id }) => id === conversationID)
  composer.value = ''; busy.value = true; error.value = ''
  try {
    const message = await request<Message>(`/api/conversations/${conversationID}/messages`, jsonInit('POST', { body, client_id: crypto.randomUUID() }))
    if (isCurrentConversation()) {
      selected.value!.read_sequence = Math.max(selected.value!.read_sequence ?? 0, message.sequence)
      selected.value!.latest_sequence = Math.max(selected.value!.latest_sequence ?? 0, message.sequence)
      if (hasNewerMessages.value) await jumpToLatest()
      else if (!messages.value.some(({ id }) => id === message.id)) messages.value.push(message)
    }
  } catch (cause) {
    if (isCurrentConversation()) {
      composer.value = body
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

async function openConversationDialog() {
  if (!organisation.value) return
  users.value = await request<User[]>(`/api/organisations/${organisation.value.id}/users`)
  showConversation.value = true
}

async function createConversation() {
  if (!organisation.value || !conversationName.value.trim()) return
  try {
    const conversation = await request<Conversation>(`/api/organisations/${organisation.value.id}/conversations`, jsonInit('POST', { name: conversationName.value, visibility: 'members', member_ids: memberIDs.value }))
    conversations.value.push(conversation); showConversation.value = false; conversationName.value = ''; memberIDs.value = []
    await selectConversation(conversation)
  } catch (cause) { error.value = messageOf(cause) }
}

async function deleteSelectedConversation() {
  if (!organisation.value || organisation.value.role !== 'admin' || !selected.value || busy.value) return
  const conversation = selected.value
  if (!window.confirm(`Delete #${conversation.name}? This removes its messages and nobody will be able to send to it.`)) return
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
    if (conversations.value[0]) await selectConversation(conversations.value[0])
  } catch (cause) { error.value = messageOf(cause) }
}

async function reconcileDeletedConversation(conversationID: string) {
  await handleConversationDeleted(conversationID)
  await refreshConversations()
}

async function refreshConversations() {
  if (!organisation.value) return
  const refreshGeneration = ++conversationRefreshGeneration
  try {
    const currentConversations = await request<Conversation[]>(`/api/organisations/${organisation.value.id}/conversations`)
    if (refreshGeneration !== conversationRefreshGeneration) return
    const refreshedSelected = currentConversations.find(conversation => conversation.id === selected.value?.id)
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
    if (currentConversations[0]) await selectConversation(currentConversations[0])
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
    <WorkspaceSidebar :organisation="organisation" :principal="me" :conversations="conversations" :selected="selected" :settings-active="activeView === 'settings'" @open-settings="openOrganisation" @new-conversation="openConversationDialog" @select-conversation="selectConversation" />
    <ConversationView v-if="activeView === 'chat'" v-model:composer="composer" :selected="selected" :messages="messages" :busy="busy" :error="error" :can-delete="organisation?.role === 'admin'" :has-newer-messages="hasNewerMessages" :jump-to-latest-version="jumpToLatestVersion" @delete-conversation="deleteSelectedConversation" @send-message="sendMessage" @read-through="markReadThrough" @jump-to-latest="jumpToLatest" />
    <OrganisationSettings v-else v-model:eligible-email="eligibleEmail" :organisation="organisation" :users="users" :eligible-users="eligibleUsers" :show-add-existing="showAddExisting" :notice="notice" :error="error" :bot-mutation-i-d="botMutationID" :removing-bot-i-d="removingBotID" @back="activeView = 'chat'" @toggle-add-existing="showAddExisting = !showAddExisting" @search-existing-user="searchExistingUser" @add-existing-user="addExistingUser" @add-bot="addUserStep = 'bot'" @rotate-key="rotateBotKey" @revoke-key="revokeBotKey" @begin-remove-bot="removingBotID = $event" @cancel-remove-bot="removingBotID = ''" @remove-bot="removeBot" />
  </main>

  <BaseDialog v-if="addUserStep" labelledby="bot-dialog-title" :close-disabled="addUserStep === 'key' && !keySaved" :focus-key="addUserStep" @close="closeAddUser">
    <button class="close" aria-label="Close" :disabled="addUserStep === 'key' && !keySaved" @click="closeAddUser">×</button>
    <form v-if="addUserStep === 'bot'" data-testid="create-bot" @submit.prevent="createBot"><p class="dialog-context">New bot</p><h2 id="bot-dialog-title">Name your bot</h2><label>Name<input data-testid="bot-name" v-model="botName" data-dialog-initial-focus required placeholder="e.g. Hector"></label><button>Create bot</button></form>
    <template v-else><p class="dialog-context">{{ keyAction === 'created' ? 'Bot created' : 'Key rotated' }}</p><h2 id="bot-dialog-title">{{ keyAction === 'created' ? 'Copy this key now' : 'Copy the rotated key now' }}</h2><p class="warning">This API key is shown only once. Store it somewhere secure before closing.</p><code class="api-key">{{ apiKey }}</code><button data-testid="copy-key" data-dialog-initial-focus @click="copyKey">{{ copied ? 'Copied' : 'Copy API key' }}</button><p v-if="keyError" class="error" role="alert">{{ keyError }}</p><label class="check key-saved-confirmation"><input data-testid="key-saved" v-model="keySaved" type="checkbox"><span>I saved this key securely</span></label></template>
  </BaseDialog>

  <BaseDialog v-if="showConversation" labelledby="conversation-dialog-title" @close="showConversation = false">
    <form data-testid="create-conversation" @submit.prevent="createConversation"><button type="button" class="close" aria-label="Close" @click="showConversation = false">×</button><p class="dialog-context">Private conversation</p><h2 id="conversation-dialog-title">Start a conversation</h2><label>Name<input data-testid="conversation-name" v-model="conversationName" data-dialog-initial-focus required placeholder="e.g. Planning"></label><fieldset><legend>People</legend><label v-for="user in users.filter(u => u.id !== me?.id)" :key="user.id" class="check"><input v-model="memberIDs" type="checkbox" :value="user.id"><span>{{ user.name }} <small>{{ user.kind }}</small></span></label></fieldset><button>Create conversation</button></form>
  </BaseDialog>
</template>
