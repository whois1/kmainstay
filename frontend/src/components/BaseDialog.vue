<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'

const props = withDefaults(defineProps<{ labelledby: string; closeDisabled?: boolean; focusKey?: string }>(), {
  closeDisabled: false,
  focusKey: '',
})
const emit = defineEmits<{ close: [] }>()
const dialog = ref<HTMLElement | null>(null)
const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
const focusableSelector = 'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'

function focusableElements() {
  return dialog.value ? Array.from(dialog.value.querySelectorAll<HTMLElement>(focusableSelector)) : []
}

async function focusInitial() {
  await nextTick()
  const target = dialog.value?.querySelector<HTMLElement>('[data-dialog-initial-focus]') ?? focusableElements()[0] ?? dialog.value
  target?.focus()
}

function requestClose() {
  if (!props.closeDisabled) emit('close')
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    if (!props.closeDisabled) {
      event.preventDefault()
      emit('close')
    }
    return
  }
  if (event.key !== 'Tab') return

  const focusable = focusableElements()
  if (!focusable.length) {
    event.preventDefault()
    dialog.value?.focus()
    return
  }
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  const active = document.activeElement
  if (event.shiftKey && (active === first || !dialog.value?.contains(active))) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && (active === last || !dialog.value?.contains(active))) {
    event.preventDefault()
    first.focus()
  }
}

onMounted(focusInitial)
watch(() => props.focusKey, focusInitial, { flush: 'post' })
onBeforeUnmount(() => previousFocus?.focus())
</script>

<template>
  <div class="scrim" @click.self="requestClose">
    <section ref="dialog" class="modal card" role="dialog" aria-modal="true" :aria-labelledby="labelledby" tabindex="-1" @keydown="handleKeydown">
      <slot />
    </section>
  </div>
</template>
