// nuxt-umami writes host and website id into the build output. This image is
// configured when the container starts, so an empty build env used to force a
// hardcoded id. The stock script tag stays out until both runtime values are set.
export default defineNuxtPlugin(() => {
  const { umamiHost, umamiId } = useRuntimeConfig().public
  const host = umamiHost.replace(/\/+$/, '')
  const id = umamiId.trim()
  if (!/^https?:\/\//.test(host) || !id) return

  useHead({
    script: [
      {
        key: 'umami',
        src: `${host}/script.js`,
        defer: true,
        'data-website-id': id
      }
    ]
  })
})
