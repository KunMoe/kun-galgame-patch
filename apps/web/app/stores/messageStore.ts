import { defineStore } from 'pinia'

// Holds the set of unread notification *types* (mirrors GET /message/unread —
// user_message types only, NOT chat). Shared so the top-bar bell and the
// /message/notice page agree: the bell reads it to show the "new" dot, and the
// notice page clears it after marking everything read on entry.
//
// Ephemeral session state — not persisted; the top-bar User.vue refetches it
// on mount.
// commentUnread is a SECOND source for the same dot: the comment walls a reader
// follows live in the community primitive, which counts their unread posts
// itself (GET /community/unread/count). It is not a user_message type, so it
// cannot be muted through muted_message_types and is held apart from them.
export const useMessageStore = defineStore('message', {
  state: (): { unreadTypes: string[]; commentUnread: number } => ({
    unreadTypes: [],
    commentUnread: 0
  }),
  actions: {
    setUnread(types: string[]) {
      this.unreadTypes = types ?? []
    },
    setCommentUnread(total: number) {
      this.commentUnread = total ?? 0
    },
    // Only the user_message side. Marking notifications read does not touch a
    // comment wall's read state — that receipt is per thread and is made by
    // opening the wall, not by visiting the notice page.
    clear() {
      this.unreadTypes = []
    }
  }
})
