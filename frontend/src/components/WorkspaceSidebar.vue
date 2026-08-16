<script setup lang="ts">
import type { Conversation, Organisation, Principal } from '../types'

defineProps<{
  organisation: Organisation | null
  principal: Principal
  conversations: Conversation[]
  selected: Conversation | null
  settingsActive: boolean
}>()

defineEmits<{
  openSettings: []
  newConversation: []
  selectConversation: [conversation: Conversation]
}>()
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
    <div class="side-heading"><span>Conversations</span><button data-testid="new-conversation" class="icon-button" aria-label="New private conversation" @click="$emit('newConversation')">＋</button></div>
    <nav aria-label="Conversations">
      <button v-for="conversation in conversations" :key="conversation.id" :class="{ active: selected?.id === conversation.id }" @click="$emit('selectConversation', conversation)">
        <span class="conversation-hash">#</span><span class="conversation-label">{{ conversation.name }}</span><small v-if="conversation.visibility === 'members'">private</small>
      </button>
    </nav>
    <div class="profile"><span>{{ principal.name.slice(0, 1) }}</span><div><strong>{{ principal.name }}</strong><small>Human</small></div></div>
  </aside>
</template>
