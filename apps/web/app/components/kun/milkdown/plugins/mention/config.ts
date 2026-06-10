// The @-mention dropdown is rendered by the prosemirror-adapter, outside the
// normal Nuxt setup tree, so it can't reliably reach useRuntimeConfig() at
// runtime. Editor.vue captures the API base in setup (it already does this for
// the upload plugin) and stamps it here so Mention.vue can build the
// /user/search request URL. `$fetch` itself is a global and works fine there.
let apiBase = ''

export const setMentionApiBase = (base: string) => {
  apiBase = base
}

export const getMentionApiBase = () => apiBase
