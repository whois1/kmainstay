<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRealtime } from './composables/realtime'
import { renderMarkdown } from './markdown'
import type { Conversation, EligibleUser, Message, Organisation, Principal, User } from './types'

type Fetcher = typeof fetch
const fetcher = inject<Fetcher>('fetcher', fetch)
const Socket = inject<typeof WebSocket>('socketFactory', WebSocket)
const me = ref<Principal | null>(null)
const organisation = ref<Organisation | null>(null)
const conversations = ref<Conversation[]>([])
const selected = ref<Conversation | null>(null)
const messages = ref<Message[]>([])
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
const people = computed(() => users.value.filter(user => user.kind === 'human'))
const bots = computed(() => users.value.filter(user => user.kind === 'bot'))

const realtime = useRealtime((message) => {
  if (message.conversation_id === selected.value?.id && !messages.value.some(({ id }) => id === message.id)) {
    messages.value.push(message)
  }
}, Socket)

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
  selected.value = conversation
  messages.value = await request<Message[]>(`/api/conversations/${conversation.id}/messages?limit=100`)
  realtime.seed(messages.value)
  activeView.value = 'chat'
}

async function sendMessage() {
  if (!composer.value.trim() || !selected.value || busy.value) return
  const body = composer.value
  composer.value = ''; busy.value = true; error.value = ''
  try {
    const message = await request<Message>(`/api/conversations/${selected.value.id}/messages`, jsonInit('POST', { body, client_id: crypto.randomUUID() }))
    if (!messages.value.some(({ id }) => id === message.id)) messages.value.push(message)
  } catch (cause) { composer.value = body; error.value = messageOf(cause) } finally { busy.value = false }
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

async function addExistingUser() {
  if (!organisation.value || !selectedEligibleUserID.value) return
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
  if (botMutationID.value) return
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

function jsonInit(method: string, body?: unknown): RequestInit {
  return { method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) }
}
function messageOf(cause: unknown) { return cause instanceof Error ? cause.message : 'Something went wrong' }
const title = computed(() => selected.value?.name ?? 'Choose a conversation')
onMounted(initialise)
onBeforeUnmount(realtime.disconnect)
</script>

<template>
  <main v-if="!me" class="login-shell">
    <form class="card login-card" @submit.prevent="login">
      <p class="eyebrow">K—MAINSTAY</p><h1>Welcome back</h1><p class="muted">Sign in to your workspace.</p>
      <label>Email<input v-model="email" type="email" autocomplete="email" required></label>
      <label>Password<input v-model="password" type="password" autocomplete="current-password" required></label>
      <p v-if="error" class="error" role="alert">{{ error }}</p><button :disabled="busy">Sign in</button>
    </form>
  </main>
  <main v-else class="workspace">
    <aside>
      <div data-testid="organisation-brand" class="brand"><span class="mark">K</span><span class="brand-copy"><strong>{{ organisation?.name }}</strong><small>{{ organisation?.role }}</small></span><button data-testid="organisation-settings" class="brand-settings" :class="{ active: activeView === 'settings' }" aria-label="Organisation settings" title="Organisation settings" @click="openOrganisation"><svg aria-hidden="true" viewBox="0 0 24 24"><path d="M19.43 12.98c.04-.32.07-.65.07-.98s-.03-.66-.07-.98l2.11-1.65c.19-.15.24-.42.12-.64l-2-3.46a.5.5 0 0 0-.6-.22l-2.49 1a7.3 7.3 0 0 0-1.69-.98l-.38-2.65A.5.5 0 0 0 14 2h-4a.5.5 0 0 0-.49.42l-.38 2.65c-.61.25-1.17.58-1.69.98l-2.49-1a.5.5 0 0 0-.6.22l-2 3.46a.5.5 0 0 0 .12.64l2.11 1.65a7.4 7.4 0 0 0 0 1.96l-2.11 1.65a.5.5 0 0 0-.12.64l2 3.46a.5.5 0 0 0 .6.22l2.49-1c.52.4 1.08.73 1.69.98l.38 2.65A.5.5 0 0 0 10 22h4a.5.5 0 0 0 .49-.42l.38-2.65c.61-.25 1.17-.58 1.69-.98l2.49 1a.5.5 0 0 0 .6-.22l2-3.46a.5.5 0 0 0-.12-.64l-2.11-1.65ZM12 15.5A3.5 3.5 0 1 1 12 8a3.5 3.5 0 0 1 0 7.5Z"/></svg></button></div>
      <div class="side-heading"><span>Conversations</span><button data-testid="new-conversation" class="icon-button" aria-label="New private conversation" @click="openConversationDialog">＋</button></div>
      <nav><button v-for="conversation in conversations" :key="conversation.id" :class="{ active: selected?.id === conversation.id }" @click="selectConversation(conversation)"><span class="conversation-hash">#</span><span class="conversation-label">{{ conversation.name }}</span><small v-if="conversation.visibility === 'members'">private</small></button></nav>
      <div class="profile"><span>{{ me.name.slice(0, 1) }}</span><div><strong>{{ me.name }}</strong><small>Human</small></div></div>
    </aside>
    <section v-if="activeView === 'chat'" class="conversation">
      <header><div><h1># {{ title }}</h1><p>{{ selected?.visibility === 'members' ? 'Private conversation' : 'Everyone in the organisation' }}</p></div></header>
      <div class="message-list" aria-live="polite">
        <article v-for="message in messages" :key="message.id" data-testid="message">
          <div class="avatar" :class="{ bot: message.author_kind === 'bot' }">{{ message.author_name.slice(0, 1) }}</div>
          <div><div class="message-meta"><strong>{{ message.author_name }}</strong><span v-if="message.author_kind === 'bot'" class="bot-badge">BOT</span><time>{{ new Date(message.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) }}</time></div><div class="markdown" v-html="renderMarkdown(message.body)" /></div>
        </article>
      </div>
      <form v-if="selected" data-testid="composer" class="composer" @submit.prevent="sendMessage"><textarea v-model="composer" :placeholder="`Message #${selected.name}`" rows="1" @keydown.enter.exact.prevent="sendMessage" /><button :disabled="busy || !composer.trim()" aria-label="Send message">↑</button><small>Markdown supported · Enter to send</small></form>
      <p v-if="error" class="toast error" role="alert">{{ error }}</p>
    </section>
    <section v-else data-testid="settings-page" class="settings-page">
      <header class="settings-header"><button data-testid="back-to-chat" class="back-button" @click="activeView = 'chat'">← Back to chat</button><div><p class="eyebrow">ORGANISATION SETTINGS</p><h1>{{ organisation?.name }}</h1><p>Manage who belongs here and how bots authenticate.</p></div></header>
      <div class="settings-content">
        <p v-if="notice" class="notice" role="status">{{ notice }}</p>
        <section data-testid="people-section" class="settings-section card">
          <div class="settings-section-heading"><div><h2>People</h2><p>Existing human members and their organisation roles.</p></div><button v-if="organisation?.role === 'admin'" data-testid="add-existing-user" class="secondary" @click="showAddExisting = !showAddExisting">＋ Add existing user</button></div>
          <form v-if="showAddExisting" data-testid="search-existing-user-form" class="existing-user-form" @submit.prevent="searchExistingUser">
            <label>Email address<input data-testid="existing-email" v-model="eligibleEmail" type="email" required placeholder="person@example.com"></label>
            <button :disabled="!eligibleEmail.trim()">Search</button>
          </form>
          <article v-for="candidate in eligibleUsers" :key="candidate.id" class="settings-row existing-user-result"><div class="member-identity"><span class="member-avatar">{{ candidate.name.slice(0, 1) }}</span><div><strong>{{ candidate.name }}</strong><small>{{ candidate.email }}</small></div></div><button :data-testid="`add-existing-user-${candidate.id}`" @click="addExistingUser">Add member</button></article>
          <div class="settings-list"><article v-for="person in people" :key="person.id" class="settings-row"><div class="member-identity"><span class="member-avatar">{{ person.name.slice(0, 1) }}</span><div><strong>{{ person.name }}</strong><small>Human</small></div></div><span class="role-badge">{{ person.role }}</span></article></div>
          <p class="settings-note">Existing accounts can be added as members. Invitations, role changes and human removal wait for ownership transfer and last-admin rules.</p>
        </section>
        <section data-testid="bots-section" class="settings-section card"><div class="settings-section-heading"><div><h2>Bots</h2><p>Automated members using copy-once API keys.</p></div><button v-if="organisation?.role === 'admin'" data-testid="add-bot" @click="addUserStep = 'bot'">＋ Add bot</button></div><div v-if="bots.length" class="settings-list"><article v-for="bot in bots" :key="bot.id" class="settings-row bot-row"><div class="member-identity"><span class="member-avatar bot">{{ bot.name.slice(0, 1) }}</span><div><strong>{{ bot.name }}</strong><small>Bot · {{ bot.role }}</small></div></div><div v-if="organisation?.role === 'admin'" class="key-actions"><button :data-testid="`rotate-key-${bot.id}`" class="secondary" :disabled="!!botMutationID" @click="rotateBotKey(bot)">Rotate key</button><button :data-testid="`revoke-key-${bot.id}`" class="secondary" :disabled="!!botMutationID" @click="revokeBotKey(bot)">Revoke key</button><button :data-testid="`remove-bot-${bot.id}`" class="danger-outline" :disabled="!!botMutationID" @click="removingBotID = bot.id">Remove</button></div><div v-if="removingBotID === bot.id" class="removal-confirmation"><p>Remove <strong>{{ bot.name }}</strong>? Its access to this organisation will stop immediately. If it has no other memberships, its keys stop too. Existing messages stay visible.</p><div><button class="secondary" :disabled="!!botMutationID" @click="removingBotID = ''">Cancel</button><button :data-testid="`confirm-remove-${bot.id}`" class="danger" :disabled="!!botMutationID" @click="removeBot(bot)">Remove {{ bot.name }}</button></div></div></article></div><p v-else class="empty-state">No bots have been added.</p></section>
      </div>
      <p v-if="error" class="toast error" role="alert">{{ error }}</p>
    </section>
  </main>

  <div v-if="addUserStep" class="scrim" @click.self="closeAddUser"><section class="modal card">
    <button class="close" aria-label="Close" :disabled="addUserStep === 'key' && !keySaved" @click="closeAddUser">×</button>
    <form v-if="addUserStep === 'bot'" data-testid="create-bot" @submit.prevent="createBot"><p class="eyebrow">NEW BOT</p><h2>Name your bot</h2><label>Name<input data-testid="bot-name" v-model="botName" autofocus required placeholder="e.g. Hector"></label><button>Create bot</button></form>
    <template v-else><p class="eyebrow">{{ keyAction === 'created' ? 'BOT CREATED' : 'KEY ROTATED' }}</p><h2>{{ keyAction === 'created' ? 'Copy this key now' : 'Copy the rotated key now' }}</h2><p class="warning">This API key is shown only once. Store it somewhere secure before closing.</p><code class="api-key">{{ apiKey }}</code><button data-testid="copy-key" @click="copyKey">{{ copied ? 'Copied' : 'Copy API key' }}</button><p v-if="keyError" class="error" role="alert">{{ keyError }}</p><label class="check key-saved-confirmation"><input data-testid="key-saved" v-model="keySaved" type="checkbox"><span>I saved this key securely</span></label></template>
  </section></div>

  <div v-if="showConversation" class="scrim" @click.self="showConversation = false"><form class="modal card" data-testid="create-conversation" @submit.prevent="createConversation"><button type="button" class="close" aria-label="Close" @click="showConversation = false">×</button><p class="eyebrow">PRIVATE CONVERSATION</p><h2>Start a conversation</h2><label>Name<input data-testid="conversation-name" v-model="conversationName" required placeholder="e.g. Planning"></label><fieldset><legend>People</legend><label v-for="user in users.filter(u => u.id !== me?.id)" :key="user.id" class="check"><input v-model="memberIDs" type="checkbox" :value="user.id"><span>{{ user.name }} <small>{{ user.kind }}</small></span></label></fieldset><button>Create conversation</button></form></div>
</template>
