<script setup lang="ts">
// Every page number this site used before the id realignment lands here, and
// nowhere else.
//
// /patch/<n> and /galgame/<n> are two different namespaces for the same kind of
// number: 9,519 of the old page ids are also a live catalog work id, so
// /patch/4107 and /galgame/4107 are different games. That is why this route
// resolves ONLY out of patch_redirect and 404s on a miss — inferring
// "not in the ledger, so it must be the same number" would quietly serve the
// wrong game for the 8,484 pages whose old number now belongs to another.
const route = useRoute()
const api = useApi()

// A catch-all hands `slug` over as an array of segments, and String() joins it
// with a comma: "999001,resource" is NaN as an id, so /patch/<n>/resource and
// /patch/<n>/comment fell through to the not-found branch and answered 200
// while only the bare /patch/<n> redirected.
const segments = (
  Array.isArray(route.params.slug) ? route.params.slug : [route.params.slug]
)
  .map(String)
  .filter(Boolean)

const [rawID, segment] = segments
const legacyID = Number(rawID)

const TAB_OF_SEGMENT: Record<string, string> = {
  resource: 'resource',
  comment: 'comment'
}

const target = await (async () => {
  if (!Number.isInteger(legacyID) || legacyID < 1) return ''
  const res = await api.get<{ moved_to: number }>(`/patch/legacy/${legacyID}`)
  if (res.code !== 0 || !res.data?.moved_to) return ''

  const id = res.data.moved_to
  if (segment === 'edit') return `/galgame/${id}/edit`
  const tab = TAB_OF_SEGMENT[segment ?? '']
  return tab ? `/galgame/${id}?tab=${tab}` : `/galgame/${id}`
})()

if (!target) {
  throw createError({
    statusCode: 404,
    statusMessage: '这个页面的地址已经变了，而且没有找到它的新位置',
    fatal: true
  })
}

useKunDisableSeo('页面已迁移')

await navigateTo(target, { redirectCode: 301, replace: true })
</script>

<template>
  <div class="py-16">
    <KunLoading description="正在跳转到新地址" />
  </div>
</template>
