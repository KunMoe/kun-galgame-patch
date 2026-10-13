<script setup lang="ts">
import {
  KUN_RATING_ASPECTS,
  KUN_RATING_PLAY_STATE_MAP,
  KUN_RATING_RECOMMEND_COLOR,
  KUN_RATING_RECOMMEND_MAP,
  KUN_RATING_SPOILER_COLOR,
  KUN_RATING_SPOILER_MAP
} from '~/constants/rating'

const props = defineProps<{ rating: GalgameRating }>()

const emit = defineEmits<{ deleted: [number] }>()

const api = useApi()
const userStore = useUserStore()

const isOwn = computed(() => props.rating.user?.id === userStore.user.id)
const displayName = computed(() => props.rating.user?.name ?? '已注销用户')

const spoilerRevealed = ref(false)
const masked = computed(
  () =>
    !!props.rating.short_summary &&
    props.rating.spoiler_level !== 'none' &&
    !spoilerRevealed.value
)

const aspects = computed(() =>
  KUN_RATING_ASPECTS.flatMap((a) => {
    const score = props.rating.aspect_scores[a.key]
    return score === null ? [] : [{ key: a.key, label: a.label, score }]
  })
)

const deleteOpen = ref(false)
const deleting = ref(false)

const confirmDelete = async () => {
  deleting.value = true
  const res = await api.delete(`/patch/rating/${props.rating.id}`)
  deleting.value = false
  if (res.code !== 0) {
    useKunMessage(res.message || '删除失败', 'error')
    return
  }
  deleteOpen.value = false
  useKunMessage('已删除评分', 'success')
  emit('deleted', props.rating.id)
}
</script>

<template>
  <div class="border-default/20 space-y-3 border-b py-4 last:border-b-0">
    <div class="flex items-start justify-between gap-3">
      <div class="flex min-w-0 items-center gap-3">
        <KunAvatar :user="toKunUser(rating.user)" />
        <div class="min-w-0">
          <p class="truncate font-semibold">{{ displayName }}</p>
          <p class="text-default-500 text-xs">
            {{
              formatDate(rating.created, { isPrecise: true, isShowYear: true })
            }}
          </p>
        </div>
      </div>
      <div class="flex shrink-0 items-baseline gap-0.5">
        <span class="text-warning text-2xl leading-none font-bold tabular-nums">
          {{ rating.overall }}
        </span>
        <span class="text-default-500 text-xs">/10</span>
      </div>
    </div>

    <div class="flex flex-wrap items-center gap-2">
      <KunChip
        :color="KUN_RATING_RECOMMEND_COLOR[rating.recommend] ?? 'default'"
        variant="flat"
        size="sm"
      >
        {{ KUN_RATING_RECOMMEND_MAP[rating.recommend] ?? rating.recommend }}
      </KunChip>
      <KunChip
        v-if="rating.play_state"
        color="default"
        variant="flat"
        size="sm"
      >
        {{ KUN_RATING_PLAY_STATE_MAP[rating.play_state] ?? rating.play_state }}
      </KunChip>
      <KunChip
        v-if="rating.spoiler_level !== 'none'"
        :color="KUN_RATING_SPOILER_COLOR[rating.spoiler_level] ?? 'warning'"
        variant="flat"
        size="sm"
      >
        {{
          KUN_RATING_SPOILER_MAP[rating.spoiler_level] ?? rating.spoiler_level
        }}
      </KunChip>
    </div>

    <KunButton
      v-if="masked"
      variant="flat"
      color="default"
      size="sm"
      full-width
      class-name="justify-start"
      @click="spoilerRevealed = true"
    >
      <KunIcon name="lucide:eye-off" class="size-4" />
      这条短评含有剧透，点击查看
    </KunButton>
    <p
      v-else-if="rating.short_summary"
      class="text-default-700 text-sm break-words whitespace-pre-line"
    >
      {{ rating.short_summary }}
    </p>

    <div v-if="aspects.length" class="flex flex-wrap gap-x-4 gap-y-1">
      <span
        v-for="aspect in aspects"
        :key="aspect.key"
        class="text-default-500 text-xs"
      >
        {{ aspect.label }}
        <span class="text-default-700 font-medium tabular-nums">
          {{ aspect.score }}
        </span>
      </span>
    </div>

    <div v-if="rating.can_delete" class="flex justify-end">
      <KunButton
        size="sm"
        variant="light"
        color="danger"
        @click="deleteOpen = true"
      >
        <KunIcon name="lucide:trash-2" class="size-4" />
        删除
      </KunButton>
    </div>

    <KunModal
      v-model="deleteOpen"
      size="sm"
      :is-dismissable="false"
      aria-label="删除评分"
    >
      <div class="space-y-4">
        <h3 class="text-lg font-bold">删除这条评分？</h3>
        <p class="text-default-600 text-sm">
          评分在所有 NextMoe 站点共用，删除后鲲 Galgame
          等站点上也会一起消失，且无法恢复。
        </p>
        <p v-if="!isOwn" class="text-danger text-sm">
          这是其他用户的评分，删除需要 NextMoe 资料库的评分管理权限。
        </p>
        <div class="flex justify-end gap-2">
          <KunButton
            variant="light"
            color="default"
            :disabled="deleting"
            @click="deleteOpen = false"
          >
            取消
          </KunButton>
          <KunButton
            color="danger"
            :loading="deleting"
            :disabled="deleting"
            @click="confirmDelete"
          >
            删除
          </KunButton>
        </div>
      </div>
    </KunModal>
  </div>
</template>
