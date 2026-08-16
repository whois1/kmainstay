<script setup lang="ts">
import { computed } from 'vue'
import type { EligibleUser, Organisation, User } from '../types'

const props = defineProps<{
  organisation: Organisation | null; users: User[]; eligibleUsers: EligibleUser[]; eligibleEmail: string
  showAddExisting: boolean; notice: string; error: string; botMutationID: string; removingBotID: string
}>()
const emit = defineEmits<{
  back: []; toggleAddExisting: []; searchExistingUser: []; addExistingUser: [userID: string]; addBot: []
  rotateKey: [bot: User]; revokeKey: [bot: User]; beginRemoveBot: [botID: string]; cancelRemoveBot: []; removeBot: [bot: User]
  'update:eligibleEmail': [value: string]
}>()
const people = computed(() => props.users.filter(user => user.kind === 'human'))
const bots = computed(() => props.users.filter(user => user.kind === 'bot'))
</script>

<template>
  <section data-testid="settings-page" class="settings-page" aria-label="Organisation settings">
    <header class="settings-header"><button data-testid="back-to-chat" class="back-button" @click="emit('back')">← Back to chat</button><div><h1>{{ organisation?.name }}</h1><p>Manage who belongs here and how bots authenticate.</p></div></header>
    <div class="settings-content">
      <p v-if="notice" class="notice" role="status">{{ notice }}</p>
      <section data-testid="people-section" class="settings-section">
        <div class="settings-section-heading"><div><h2>People</h2><p>Existing human members and their organisation roles.</p></div><button v-if="organisation?.role === 'admin'" data-testid="add-existing-user" class="secondary" @click="emit('toggleAddExisting')">Add existing user</button></div>
        <form v-if="showAddExisting" data-testid="search-existing-user-form" class="existing-user-form" @submit.prevent="emit('searchExistingUser')">
          <label>Email address<input data-testid="existing-email" :value="eligibleEmail" type="email" required placeholder="person@example.com" @input="emit('update:eligibleEmail', ($event.target as HTMLInputElement).value)"></label>
          <button :disabled="!eligibleEmail.trim()">Search</button>
        </form>
        <article v-for="candidate in eligibleUsers" :key="candidate.id" class="settings-row existing-user-result"><div class="member-identity"><span class="member-avatar">{{ candidate.name.slice(0, 1) }}</span><div><strong>{{ candidate.name }}</strong><small>{{ candidate.email }}</small></div></div><button :data-testid="`add-existing-user-${candidate.id}`" @click="emit('addExistingUser', candidate.id)">Add member</button></article>
        <div class="settings-list"><article v-for="person in people" :key="person.id" class="settings-row"><div class="member-identity"><span class="member-avatar">{{ person.name.slice(0, 1) }}</span><div><strong>{{ person.name }}</strong><small>Human</small></div></div><span class="role-badge">{{ person.role }}</span></article></div>
        <p class="settings-note">Existing accounts can be added as members. Invitations, role changes and human removal wait for ownership transfer and last-admin rules.</p>
      </section>
      <section data-testid="bots-section" class="settings-section">
        <div class="settings-section-heading"><div><h2>Bots</h2><p>Automated members using copy-once API keys.</p></div><button v-if="organisation?.role === 'admin'" data-testid="add-bot" @click="emit('addBot')">Add bot</button></div>
        <div v-if="bots.length" class="settings-list"><article v-for="bot in bots" :key="bot.id" class="settings-row bot-row"><div class="member-identity"><span class="member-avatar bot">{{ bot.name.slice(0, 1) }}</span><div><strong>{{ bot.name }}</strong><small>Bot · {{ bot.role }}</small></div></div><div v-if="organisation?.role === 'admin'" class="key-actions"><button :data-testid="`rotate-key-${bot.id}`" class="secondary" :disabled="!!botMutationID" @click="emit('rotateKey', bot)">Rotate key</button><button :data-testid="`revoke-key-${bot.id}`" class="secondary" :disabled="!!botMutationID" @click="emit('revokeKey', bot)">Revoke key</button><button :data-testid="`remove-bot-${bot.id}`" class="danger-outline" :disabled="!!botMutationID" @click="emit('beginRemoveBot', bot.id)">Remove</button></div><div v-if="removingBotID === bot.id" class="removal-confirmation"><p>Remove <strong>{{ bot.name }}</strong>? Its access to this organisation will stop immediately. If it has no other memberships, its keys stop too. Existing messages stay visible.</p><div><button class="secondary" :disabled="!!botMutationID" @click="emit('cancelRemoveBot')">Cancel</button><button :data-testid="`confirm-remove-${bot.id}`" class="danger" :disabled="!!botMutationID" @click="emit('removeBot', bot)">Remove {{ bot.name }}</button></div></div></article></div>
        <p v-else class="empty-state">No bots have been added.</p>
      </section>
    </div>
    <p v-if="error" class="toast error" role="alert">{{ error }}</p>
  </section>
</template>
