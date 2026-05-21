import { render, h, ref } from 'vue'
import Alert from '../components/kun/alert/Alert.vue'

export interface KunAlertOptions {
  title?: string
  message?: string
  showCancel?: boolean
}

interface KunAlertState {
  show: boolean
  title: string
  message: string
  showCancel: boolean
}

const state = ref<KunAlertState>({
  show: false,
  title: '',
  message: '',
  showCancel: true
})

let resolver: ((ok: boolean) => void) | null = null
let containerEl: HTMLElement | null = null

export const useKunAlertState = () => ({
  state,
  handleConfirm: () => {
    state.value.show = false
    resolver?.(true)
    resolver = null
  },
  handleCancel: () => {
    state.value.show = false
    resolver?.(false)
    resolver = null
  }
})

const initializeContainer = () => {
  if (containerEl) return
  if (!import.meta.client) return

  containerEl = document.createElement('div')
  containerEl.id = 'kun-alert-root'
  document.body.appendChild(containerEl)

  const vNode = h(Alert)
  const nuxtApp = useNuxtApp()
  vNode.appContext = nuxtApp.vueApp._context
  render(vNode, containerEl)
}

export const useKunAlert = (opts: KunAlertOptions = {}): Promise<boolean> => {
  initializeContainer()
  return new Promise((resolve) => {
    resolver = resolve
    state.value = {
      show: true,
      title: opts.title ?? '',
      message: opts.message ?? '',
      showCancel: opts.showCancel ?? true
    }
  })
}
