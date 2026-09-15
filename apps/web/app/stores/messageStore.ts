import { defineStore } from 'pinia'

// Holds the unread counts keyed by category (mirrors GET /message/unread/counts
// — one entry per user_message type, plus `notice` for their sum and `chat` for
// private/group messages). Shared so the top-bar bell and the /message center
// tabs agree on every badge from a single fetch.
//
// Ephemeral session state — not persisted; the top-bar User.vue refetches it
// on mount and the reading pages refresh it after they mark things read.
export const useMessageStore = defineStore('message', {
  state: (): { unreadCounts: Record<string, number> } => ({
    unreadCounts: {}
  }),
  getters: {
    // `notice` is the aggregate of the notification types; chat is its own
    // table. Both feed the bell's dot, but muted types stay silent.
    hasAnyUnread(state): boolean {
      const muted = useUserStore().user.muted_message_types ?? []
      return Object.entries(state.unreadCounts).some(([type, count]) => {
        if (type === 'notice' || count <= 0) return false
        return !muted.includes(type === 'chat' ? 'pm' : type)
      })
    }
  },
  actions: {
    setCounts(counts: Record<string, number>) {
      this.unreadCounts = counts ?? {}
    },
    clear() {
      this.unreadCounts = {}
    }
  }
})
