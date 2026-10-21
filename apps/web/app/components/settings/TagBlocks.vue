<script setup lang="ts">
import { KUN_TAG_BLOCKS_MAX } from '~/constants/tagBlock'

const { isSignedIn, blocks, count, modeOf, block, unblock, refresh } =
  useTagBlocks()
const names = useEntityNames()

const blockedIds = computed(() =>
  [...blocks.value.tree, ...blocks.value.exact].sort((a, b) =>
    names.labelOf('tag', a).localeCompare(names.labelOf('tag', b), 'zh-Hans-CN')
  )
)

const resolveNames = () => names.resolve({ company: [], tag: blockedIds.value })

onMounted(async () => {
  if (isSignedIn.value) await refresh()
  resolveNames()
})
watch(blockedIds, resolveNames)

const busy = ref(0)
const run = async (
  id: number,
  action: () => Promise<boolean>,
  done: string
) => {
  busy.value = id
  const ok = await action()
  busy.value = 0
  if (ok) useKunMessage(done, 'success')
}

const nameOf = (id: number) => names.labelOf('tag', id)
const descendantsOf = (id: number) =>
  names.itemOf('tag', id)?.descendant_count ?? 0

const onBlock = (item: SearchEntityItem, mode: KunTagBlockMode) => {
  names.remember(item)
  const n = descendantsOf(item.id)
  const done =
    mode === 'tree' && n
      ? `已屏蔽「${nameOf(item.id)}」及其 ${n} 个子标签`
      : `已屏蔽「${nameOf(item.id)}」`
  run(item.id, () => block(item.id, mode), done)
}

const onUnblock = (id: number) =>
  run(id, () => unblock(id), `已取消屏蔽「${nameOf(id)}」`)

const onMode = (id: number, mode: KunTagBlockMode) => {
  if (modeOf(id) === mode) return
  const done =
    mode === 'tree'
      ? `「${nameOf(id)}」改为连同子标签一起屏蔽`
      : `「${nameOf(id)}」改为只屏蔽这一个标签`
  run(id, () => block(id, mode), done)
}
</script>

<template>
  <KunCard :bordered="true">
    <template #header>
      <div class="flex items-center justify-between gap-3 px-1 pt-1">
        <h2 class="text-xl font-medium">屏蔽标签</h2>
        <KunChip
          size="xs"
          variant="flat"
          :color="count >= KUN_TAG_BLOCKS_MAX ? 'danger' : 'default'"
        >
          <span class="tabular-nums">
            {{ count }} / {{ KUN_TAG_BLOCKS_MAX }}
          </span>
        </KunChip>
      </div>
    </template>
    <div class="space-y-5">
      <p class="text-default-500 text-xs">
        带有被屏蔽标签的 Galgame 不会出现在首页、Galgame 列表、鲲 Galgame
        论坛数据、搜索、 发售月表、排行以及标签 / 会社 / 系列 / 角色 / Staff
        页中，它们的补丁资源也不会出现在资源列表里。屏蔽有子标签的标签时，可以连同子标签一起屏蔽，也可以只屏蔽它本身。
        {{
          isSignedIn
            ? '该设置保存在 NextMoe 账号上，全站通用'
            : '未登录时只保存在当前浏览器'
        }}
      </p>

      <SettingsTagBlocksSearch :busy="busy" @block="onBlock" />

      <div class="space-y-2">
        <p class="text-default-600 text-sm font-medium">已屏蔽的标签</p>
        <SettingsTagBlocksRow
          v-for="id in blockedIds"
          :id="id"
          :key="id"
          :item="names.itemOf('tag', id)"
          :mode="modeOf(id) ?? 'tree'"
          :busy="busy === id"
          @unblock="onUnblock(id)"
          @mode="(mode: KunTagBlockMode) => onMode(id, mode)"
        />
        <p v-if="!blockedIds.length" class="text-default-400 text-sm">
          还没有屏蔽任何标签
        </p>
      </div>
    </div>
  </KunCard>
</template>
