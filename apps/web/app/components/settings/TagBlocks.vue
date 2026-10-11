<script setup lang="ts">
import { KUN_TAG_BLOCKS_MAX } from '~/constants/tagBlock'

const { isSignedIn, blocks, count, modeOf, block, unblock, refresh } =
  useTagBlocks()
const names = useEntityNames()

const blockedIds = computed(() =>
  [...blocks.value.tree, ...blocks.value.exact].sort((a, b) => a - b)
)
const blockedItems = computed(() => names.itemsOf('tag', blockedIds.value))

const resolveNames = () => names.resolve({ company: [], tag: blockedIds.value })

onMounted(async () => {
  if (isSignedIn.value) await refresh()
  resolveNames()
})
watch(blockedIds, resolveNames)

const busy = ref(0)
const run = async (id: number, action: () => Promise<boolean>) => {
  busy.value = id
  await action()
  busy.value = 0
}

const onToggle = (item: SearchEntityItem) => {
  names.remember(item)
  run(item.id, () => (modeOf(item.id) ? unblock(item.id) : block(item.id)))
}
const onMode = (id: number, withChildren: boolean) =>
  run(id, () => block(id, withChildren ? 'tree' : 'exact'))
</script>

<template>
  <KunCard :bordered="true">
    <template #header>
      <h2 class="px-1 pt-1 text-xl font-medium">屏蔽标签</h2>
    </template>
    <div class="space-y-4">
      <p class="text-default-500 text-xs">
        带有被屏蔽标签的 Galgame 不会出现在首页、Galgame 列表、资料库、搜索、
        发售月表、排行以及标签 / 会社 / 系列 / 角色 / Staff
        页中，它们的补丁资源也不会出现在资源列表里。「含子标签」会连同更细分的标签一起屏蔽。
        {{
          isSignedIn
            ? '该设置保存在 NextMoe 账号上，全站通用'
            : '未登录时只保存在当前浏览器'
        }}
      </p>

      <div class="flex flex-wrap items-center justify-between gap-3">
        <FilterEntityMenu
          family="tag"
          label="添加屏蔽"
          icon="lucide:ban"
          placeholder="搜索要屏蔽的标签"
          :selected-ids="blockedIds"
          :selected-items="blockedItems"
          :max="KUN_TAG_BLOCKS_MAX"
          multiple
          @toggle="onToggle"
        />
        <span class="text-default-500 text-xs tabular-nums">
          已屏蔽 {{ count }} / {{ KUN_TAG_BLOCKS_MAX }}
        </span>
      </div>

      <ul v-if="blockedIds.length" class="space-y-2">
        <li
          v-for="id in blockedIds"
          :key="id"
          class="border-default-200 flex items-center justify-between gap-3 rounded-lg border px-3 py-2"
        >
          <KunLink
            :to="`/galgame/tag/${id}`"
            color="default"
            underline="hover"
            class-name="min-w-0 truncate text-sm"
          >
            {{ names.labelOf('tag', id) }}
          </KunLink>
          <div class="flex shrink-0 items-center gap-2">
            <KunSwitch
              :model-value="modeOf(id) === 'tree'"
              :disabled="busy === id"
              label="含子标签"
              size="sm"
              @update:model-value="(v: boolean) => onMode(id, v)"
            />
            <KunButton
              size="sm"
              variant="light"
              is-icon-only
              :disabled="busy === id"
              aria-label="取消屏蔽"
              @click="run(id, () => unblock(id))"
            >
              <KunIcon name="lucide:x" />
            </KunButton>
          </div>
        </li>
      </ul>
      <p v-else class="text-default-400 text-sm">还没有屏蔽任何标签</p>
    </div>
  </KunCard>
</template>
