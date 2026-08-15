<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRealtime } from './composables/realtime'
import { renderMarkdown } from './markdown'
import type { Conversation, Message, Organisation, Principal, User } from './types'

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
const addUserStep = ref<'' | 'kind' | 'bot' | 'key'>('')
const botName = ref('')
const apiKey = ref('')
const copied = ref(false)
const keyAction = ref<'created' | 'rotated'>('created')
const showOrganisation = ref(false)
const notice = ref('')
const showConversation = ref(false)
const conversationName = ref('')
const users = ref<User[]>([])
const memberIDs = ref<string[]>([])

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
    apiKey.value = bot.api_key; keyAction.value = 'created'; copied.value = false; addUserStep.value = 'key'
  } catch (cause) { error.value = messageOf(cause) }
}

async function openOrganisation() {
  if (!organisation.value) return
  users.value = await request<User[]>(`/api/organisations/${organisation.value.id}/users`)
  notice.value = ''
  showOrganisation.value = true
}

async function rotateBotKey(bot: User) {
  try {
    const result = await request<{ api_key: string }>(`/api/bots/${bot.id}/key`, jsonInit('POST'))
    apiKey.value = result.api_key
    keyAction.value = 'rotated'
    copied.value = false
    showOrganisation.value = false
    addUserStep.value = 'key'
  } catch (cause) { error.value = messageOf(cause) }
}

async function revokeBotKey(bot: User) {
  try {
    await request<void>(`/api/bots/${bot.id}/key`, jsonInit('DELETE'))
    notice.value = `${bot.name} key revoked`
  } catch (cause) { error.value = messageOf(cause) }
}

async function copyKey() {
  await navigator.clipboard.writeText(apiKey.value)
  copied.value = true
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
      <button data-testid="organisation" class="brand" @click="openOrganisation"><span class="mark">K</span><span><strong>{{ organisation?.name }}</strong><small>{{ organisation?.role }}</small></span></button>
      <div class="side-heading"><span>Conversations</span><button data-testid="new-conversation" class="icon-button" aria-label="New private conversation" @click="openConversationDialog">＋</button></div>
      <nav><button v-for="conversation in conversations" :key="conversation.id" :class="{ active: selected?.id === conversation.id }" @click="selectConversation(conversation)"><span>#</span>{{ conversation.name }}<small v-if="conversation.visibility === 'members'">private</small></button></nav>
      <button v-if="organisation?.role === 'admin'" data-testid="add-user" class="add-user" @click="addUserStep = 'kind'">＋ Add user</button>
      <div class="profile"><span>{{ me.name.slice(0, 1) }}</span><div><strong>{{ me.name }}</strong><small>Human</small></div></div>
    </aside>
    <section class="conversation">
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
  </main>

  <div v-if="addUserStep" class="scrim" @click.self="addUserStep = ''"><section class="modal card">
    <button class="close" aria-label="Close" @click="addUserStep = ''">×</button>
    <template v-if="addUserStep === 'kind'"><p class="eyebrow">ADD USER</p><h2>Who are you adding?</h2><button data-testid="choose-bot" class="choice" @click="addUserStep = 'bot'"><strong>Bot</strong><span>An automated teammate using an API key</span></button></template>
    <form v-else-if="addUserStep === 'bot'" data-testid="create-bot" @submit.prevent="createBot"><p class="eyebrow">NEW BOT</p><h2>Name your bot</h2><label>Name<input data-testid="bot-name" v-model="botName" autofocus required placeholder="e.g. Hector"></label><button>Create bot</button></form>
    <template v-else><p class="eyebrow">{{ keyAction === 'created' ? 'BOT CREATED' : 'KEY ROTATED' }}</p><h2>{{ keyAction === 'created' ? 'Copy this key now' : 'Copy the rotated key now' }}</h2><p class="warning">This API key is shown only once. Store it somewhere secure before closing.</p><code class="api-key">{{ apiKey }}</code><button data-testid="copy-key" @click="copyKey">{{ copied ? 'Copied' : 'Copy API key' }}</button></template>
  </section></div>

  <div v-if="showOrganisation" class="scrim" @click.self="showOrganisation = false"><section class="modal card organisation-modal"><button class="close" aria-label="Close" @click="showOrganisation = false">×</button><p class="eyebrow">ORGANISATION</p><h2>{{ organisation?.name }}</h2><p class="muted">People and bots in this organisation.</p><p v-if="notice" class="notice" role="status">{{ notice }}</p><div data-testid="organisation-users" class="organisation-users"><article v-for="user in users" :key="user.id" class="organisation-user"><div><strong>{{ user.name }}</strong><span class="role-badge">{{ user.role }}</span><span v-if="user.kind === 'bot'" class="bot-badge">BOT</span></div><div v-if="organisation?.role === 'admin' && user.kind === 'bot'" class="key-actions"><button :data-testid="`rotate-key-${user.id}`" class="secondary" @click="rotateBotKey(user)">Rotate key</button><button :data-testid="`revoke-key-${user.id}`" class="danger" @click="revokeBotKey(user)">Revoke key</button></div></article></div></section></div>

  <div v-if="showConversation" class="scrim" @click.self="showConversation = false"><form class="modal card" data-testid="create-conversation" @submit.prevent="createConversation"><button type="button" class="close" aria-label="Close" @click="showConversation = false">×</button><p class="eyebrow">PRIVATE CONVERSATION</p><h2>Start a conversation</h2><label>Name<input data-testid="conversation-name" v-model="conversationName" required placeholder="e.g. Planning"></label><fieldset><legend>People</legend><label v-for="user in users.filter(u => u.id !== me?.id)" :key="user.id" class="check"><input v-model="memberIDs" type="checkbox" :value="user.id"><span>{{ user.name }} <small>{{ user.kind }}</small></span></label></fieldset><button>Create conversation</button></form></div>
</template>
