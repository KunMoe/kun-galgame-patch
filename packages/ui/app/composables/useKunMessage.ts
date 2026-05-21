import { render } from 'vue'
import MessageContainer from '../components/kun/alert/MessageContainer.vue'

export type KunMessageType = 'warn' | 'success' | 'error' | 'info'
export type KunMessagePosition =
  | 'top-center'
  | 'top-left'
  | 'top-right'
  | 'bottom-center'
  | 'bottom-left'
  | 'bottom-right'

export interface KunMessageOptions {
  id: string
  message: string
  type: KunMessageType
  duration: number
  richText: boolean
  position: KunMessagePosition
  count: number
}

const messages: Ref<KunMessageOptions[]> = ref([])
let seed = 0
let containerRef: HTMLElement | null = null

export const useKunMessageState = () => ({
  messages: computed(() => messages.value),
  removeMessage: (id: string) => {
    messages.value = messages.value.filter((msg) => msg.id !== id)
  }
})

const initializeContainer = () => {
  if (containerRef) return

  containerRef = document.createElement('div')
  containerRef.id = 'kun-message-container-root'
  document.body.appendChild(containerRef)

  const vNode = h(MessageContainer)

  // CRITICAL: attach the active Nuxt app's appContext to the vNode.
  //
  // Vue's bare `render(vnode, container)` creates an isolated app
  // context. Any descendant component whose setup calls a Nuxt
  // composable — `<KunIcon>` is the live example, it wraps
  // `<NuxtIcon>` from @nuxt/icon whose setup() calls useNuxtApp() —
  // hits `Cannot read properties of null (reading '$nuxt')` because
  // tryUseNuxtApp() returns null in that isolated context.
  //
  // Walking the appContext from the live Nuxt instance preserves
  // plugins (@nuxt/icon, pinia, color-mode, etc.) AND, more
  // importantly, the `$nuxt` injection that those plugins rely on.
  //
  // tryUseNuxtApp can return null if useKunMessage is itself called
  // from a microtask outside a Nuxt context. Consumers must wrap such
  // calls in `nuxtApp.runWithContext(() => useMessage(...))` — when
  // they do, the lookup here succeeds and the container mounts
  // correctly for the rest of the session.
  const nuxtApp = tryUseNuxtApp()
  if (nuxtApp?.vueApp) {
    vNode.appContext = nuxtApp.vueApp._context
  }

  render(vNode, containerRef)
}

export const useKunMessage = (
  messageData: string,
  type: KunMessageType,
  duration = 3000,
  richText = false,
  position = 'top-center' as KunMessagePosition
) => {
  initializeContainer()

  const existingMessage = messages.value.find(
    (m) =>
      m.message === messageData && m.position === position && m.type === type
  )

  if (existingMessage) {
    existingMessage.count++
    existingMessage.duration = duration
  } else {
    seed++
    const id = `message_${seed}`

    const newMessage: KunMessageOptions = {
      id,
      message: messageData,
      type,
      duration,
      richText,
      position,
      count: 1
    }

    if (position.startsWith('top')) {
      messages.value.push(newMessage)
    } else {
      messages.value.unshift(newMessage)
    }
  }
}
