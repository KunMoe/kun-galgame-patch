import { defineStore } from 'pinia'

// Holds the set of unread notification *types* (mirrors GET /message/unread —
// user_message types only, NOT chat). Shared so the top-bar bell and the
// /message/notice page agree: the bell reads it to show the "new" dot, and the
// notice page clears it after marking everything read on entry.
//
// Ephemeral session state — not persisted; the top-bar User.vue refetches it
// on mount.
export const useMessageStore = defineStore('message', {
  state: (): { unreadTypes: string[] } => ({
    unreadTypes: []
  }),
  actions: {
    setUnread(types: string[]) {
      this.unreadTypes = types ?? []
    },
    clear() {
      this.unreadTypes = []
    }
  }
})
