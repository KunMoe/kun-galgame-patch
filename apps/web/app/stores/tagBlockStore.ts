import { defineStore } from 'pinia'

export interface KunTagBlockState {
  anonymous: KunTagBlockSet
  accountUid: number
  account: KunTagBlockSet
}

export const useTagBlockStore = defineStore('tag-block', {
  state: (): KunTagBlockState => ({
    anonymous: { tree: [], exact: [] },
    accountUid: 0,
    account: { tree: [], exact: [] }
  }),
  persist: {
    key: 'kun-patch-tag-block-store',
    storage: piniaPluginPersistedstate.cookies({
      maxAge: 60 * 60 * 24 * 400,
      sameSite: 'lax'
    })
  }
})
