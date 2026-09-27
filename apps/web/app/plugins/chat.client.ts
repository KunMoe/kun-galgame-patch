import { Centrifuge, UnauthorizedError } from 'centrifuge'

export default defineNuxtPlugin(() => {
  if (!useRuntimeConfig().public.chatEnabled) return
  const userStore = useUserStore()
  const chat = useChatStore()
  const api = useApi()
  let client: Centrifuge | null = null

  const token = async () => {
    const r = await api.post<ChatRealtimeToken>('/im/realtime-token')
    if (r.code === 0) return r.data
    if (r.code === 40313 || r.code === 40100 || r.code === 40101) {
      throw new UnauthorizedError(r.message)
    }
    throw new Error(r.message)
  }

  const connect = async (uid: number) => {
    await chat.start(String(uid))
    if (chat.status !== 'ready') return
    const first = await token().catch(() => null)
    if (!first || userStore.user.id !== uid) return
    client = new Centrifuge(first.url, {
      token: first.token,
      getToken: async () => (await token()).token
    })
    client.on('publication', (ctx) => chat.push(ctx.data as ChatPush))
    client.on('connected', () => void chat.sync())
    client.connect()
  }

  const disconnect = () => {
    client?.disconnect()
    client = null
    chat.stop()
  }

  // Started before hydration, the chat state rendered on the server and the
  // client's disagree.
  onNuxtReady(() => {
    watch(
      () => userStore.user.id,
      (id) => {
        disconnect()
        if (id > 0) void connect(id)
      },
      { immediate: true }
    )
  })

  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible' && chat.status === 'ready') {
      void chat.sync()
    }
  })
})
