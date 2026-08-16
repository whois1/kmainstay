<script setup lang="ts">
import { renderMarkdown } from '../markdown'
import type { Conversation, Message } from '../types'

defineProps<{ selected: Conversation | null; messages: Message[]; composer: string; busy: boolean; error: string; canDelete: boolean }>()
defineEmits<{ deleteConversation: []; sendMessage: []; 'update:composer': [value: string] }>()
</script>

<template>
  <section class="conversation" aria-label="Conversation">
    <header v-if="selected">
      <div><h1># {{ selected.name }}</h1><p>{{ selected.visibility === 'members' ? 'Private conversation' : 'Everyone in the organisation' }}</p></div>
      <button v-if="canDelete" data-testid="delete-conversation" class="danger-outline" :disabled="busy" aria-label="Delete conversation" @click="$emit('deleteConversation')"><span class="delete-label">Delete conversation</span><span class="delete-icon" aria-hidden="true">Delete</span></button>
    </header>
    <div v-else class="empty-state"><h1>No conversations</h1><p>Create one from the conversations list.</p></div>
    <div class="message-list" aria-live="polite">
      <article v-for="message in messages" :key="message.id" data-testid="message">
        <div class="avatar" :class="{ bot: message.author_kind === 'bot' }">{{ message.author_name.slice(0, 1) }}</div>
        <div><div class="message-meta"><strong>{{ message.author_name }}</strong><span v-if="message.author_kind === 'bot'" class="bot-badge">BOT</span><time>{{ new Date(message.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) }}</time></div><div class="markdown" v-html="renderMarkdown(message.body)" /></div>
      </article>
    </div>
    <form v-if="selected" data-testid="composer" class="composer" @submit.prevent="$emit('sendMessage')">
      <textarea :value="composer" :placeholder="`Message #${selected.name}`" rows="1" aria-label="Message" @input="$emit('update:composer', ($event.target as HTMLTextAreaElement).value)" @keydown.enter.exact.prevent="$emit('sendMessage')" />
      <button :disabled="busy || !composer.trim()" aria-label="Send message">Send</button><small>Markdown supported · Enter to send</small>
    </form>
    <p v-if="error" class="toast error" role="alert">{{ error }}</p>
  </section>
</template>
