<script setup lang="ts">
import {
  KUN_PLAYTIME_HOURS_MAX,
  KUN_PLAYTIME_MINUTES_FLOOR,
  KUN_RATING_ASPECTS,
  KUN_RATING_PLAY_STATE_DONE,
  KUN_RATING_PLAY_STATE_OPTIONS,
  KUN_RATING_RECOMMEND_OPTIONS,
  KUN_RATING_SPOILER_OPTIONS,
  KUN_RATING_SUMMARY_MAX,
  type KunRatingPlayState,
  type KunRatingRecommend,
  type KunRatingSpoiler
} from '~/constants/rating'

const props = defineProps<{
  workId: number
  workName?: string
}>()

const emit = defineEmits<{ saved: [GalgameWorkRecord] }>()

const open = defineModel<boolean>({ required: true })

const api = useApi()
const { bump } = useRatingRefresh(() => props.workId)

const loading = ref(false)
const pending = ref(false)
const deleting = ref(false)
const loaded = ref<GalgameWorkRecord | null>(null)

const status = ref<KunRatingPlayState | ''>('')
const hours = ref<number | null>(null)
const loadedHours = ref<number | null>(null)

const overall = ref(0)
const recommend = ref<KunRatingRecommend>('neutral')
const spoilerLevel = ref<KunRatingSpoiler>('none')
const shortSummary = ref('')
const aspectsOpen = ref(false)

const emptyAspects = (): GalgameRatingAspects => ({
  art: null,
  story: null,
  music: null,
  character: null,
  route: null,
  system: null,
  voice: null,
  replay_value: null
})
const aspects = ref<GalgameRatingAspects>(emptyAspects())

const isWish = computed(() => status.value === 'wish')
const hadRating = computed(() => !!loaded.value?.rating)

const stateOptions = computed(() =>
  loaded.value?.play_state === KUN_RATING_PLAY_STATE_DONE.value
    ? [KUN_RATING_PLAY_STATE_DONE, ...KUN_RATING_PLAY_STATE_OPTIONS]
    : KUN_RATING_PLAY_STATE_OPTIONS
)

// The field shows hours to one decimal; an untouched field sends the minutes
// it was filled from, or every save would round the reader's playtime.
const minutes = computed(() =>
  hours.value === loadedHours.value
    ? (loaded.value?.minutes ?? 0)
    : Math.round((hours.value ?? 0) * 60)
)

const fillRating = (r: GalgameRating | null) => {
  overall.value = r?.overall ?? 0
  recommend.value = (r?.recommend as KunRatingRecommend) ?? 'neutral'
  spoilerLevel.value = (r?.spoiler_level as KunRatingSpoiler) ?? 'none'
  shortSummary.value = r?.short_summary ?? ''
  aspects.value = r ? { ...r.aspect_scores } : emptyAspects()
  aspectsOpen.value = Object.values(aspects.value).some((v) => v !== null)
}

const fill = (record: GalgameWorkRecord) => {
  loaded.value = record
  const known = [KUN_RATING_PLAY_STATE_DONE, ...KUN_RATING_PLAY_STATE_OPTIONS]
  status.value = known.some((o) => o.value === record.play_state)
    ? (record.play_state as KunRatingPlayState)
    : ''
  hours.value =
    record.minutes > 0 ? Math.round((record.minutes / 60) * 10) / 10 : null
  loadedHours.value = hours.value
  fillRating(record.rating)
}

watch(open, async (isOpen) => {
  if (!isOpen) return
  loading.value = true
  const res = await api.get<GalgameWorkRecord>(`/patch/${props.workId}/record`)
  loading.value = false
  if (res.code !== 0) {
    useKunMessage(res.message || '读取评分失败', 'error')
    open.value = false
    return
  }
  fill(res.data)
})

const setAspect = (key: keyof GalgameRatingAspects, value: number) => {
  aspects.value = { ...aspects.value, [key]: value > 0 ? value : null }
}

const body = computed<GalgameWorkRecordInput>(() => {
  const out: GalgameWorkRecordInput = {}
  if (status.value) out.play_state = status.value
  if (isWish.value) return out
  if (minutes.value > 0) out.minutes = minutes.value
  if (overall.value > 0) {
    out.rating = {
      overall: overall.value,
      recommend: recommend.value,
      spoiler_level: spoilerLevel.value,
      short_summary: shortSummary.value,
      aspect_scores: aspectsOpen.value ? aspects.value : null
    }
  }
  return out
})

// PUT replaces the whole record: whatever the form no longer sends is deleted
// upstream, so say so before it happens.
const losingRating = computed(() => hadRating.value && !body.value.rating)
const losingPlaytime = computed(
  () => (loaded.value?.minutes ?? 0) > 0 && body.value.minutes === undefined
)

const SHARED_RATING_WARNING =
  '评分在所有 NextMoe 站点共用，删除后鲲 Galgame 等站点上也会一起消失，且无法恢复。'

const invalid = (): string => {
  if (isWish.value) return ''
  if (minutes.value > 0 && minutes.value < KUN_PLAYTIME_MINUTES_FLOOR) {
    return '不足 10 分钟的时长不计入统计，请填 0 或更长的时长'
  }
  if (minutes.value > KUN_PLAYTIME_HOURS_MAX * 60) {
    return `单部作品最多记录 ${KUN_PLAYTIME_HOURS_MAX} 小时`
  }
  const aspectsFilled =
    aspectsOpen.value && Object.values(aspects.value).some((v) => v !== null)
  if (overall.value < 1 && (shortSummary.value.trim() || aspectsFilled)) {
    return '请先给出总体评分'
  }
  return ''
}

const save = async () => {
  if (pending.value || loading.value || deleting.value) return
  const problem = invalid()
  if (problem) {
    useKunMessage(problem, 'warn')
    return
  }
  if (losingRating.value) {
    const ok = await useKunAlert({
      title: '删除评分',
      message: `想玩的作品还不能评分，保存后会删除你的评分。${SHARED_RATING_WARNING}`,
      type: 'danger',
      confirmText: '保存并删除评分',
      confirmColor: 'danger'
    })
    if (!ok) return
  }
  pending.value = true
  const res = await api.put<GalgameWorkRecord>(
    `/patch/${props.workId}/record`,
    { ...body.value }
  )
  pending.value = false
  if (res.code !== 0) {
    useKunMessage(res.message || '保存失败', 'error')
    return
  }
  bump()
  emit('saved', res.data)
  open.value = false
  useKunMessage('已保存', 'success')
}

const deleteRating = async () => {
  const rating = loaded.value?.rating
  if (!rating || pending.value || deleting.value) return
  const ok = await useKunAlert({
    title: '删除评分',
    message: `${SHARED_RATING_WARNING}游玩状态和时长会保留。`,
    type: 'danger',
    confirmText: '删除评分',
    confirmColor: 'danger'
  })
  if (!ok) return
  deleting.value = true
  const res = await api.delete(`/patch/rating/${rating.id}`)
  deleting.value = false
  if (res.code !== 0) {
    useKunMessage(res.message || '删除失败', 'error')
    return
  }
  loaded.value = { ...loaded.value!, rating: null }
  fillRating(null)
  bump()
  useKunMessage('已删除评分', 'success')
}
</script>

<template>
  <KunModal v-model="open" size="lg" :is-dismissable="false" aria-label="评分">
    <div class="space-y-5">
      <div class="space-y-1">
        <h3 class="text-lg font-bold">评分</h3>
        <p v-if="workName" class="text-default-500 line-clamp-1 text-sm">
          {{ workName }}
        </p>
        <p class="text-default-500 text-xs">
          评分、游玩状态与时长在所有 NextMoe 站点共用，在这里写的和鲲 Galgame
          等站点上的是同一份
        </p>
      </div>

      <KunLoading v-if="loading" />

      <template v-else>
        <div class="space-y-4">
          <p v-if="isWish" class="text-warning text-sm">想玩的作品还不能评分</p>

          <div class="space-y-2">
            <span class="text-default-600 text-sm">总体评分</span>
            <div class="flex flex-wrap items-center gap-4">
              <KunRating
                v-model="overall"
                :max="10"
                :disabled="isWish"
                aria-label="总体评分"
              />
              <span class="text-warning text-3xl font-bold tabular-nums">
                {{ overall || '-' }}
              </span>
              <KunButton
                v-if="overall && !hadRating && !isWish"
                size="sm"
                variant="light"
                color="default"
                @click="overall = 0"
              >
                不打分
              </KunButton>
            </div>
          </div>

          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <KunSelect
              v-model="recommend"
              :options="KUN_RATING_RECOMMEND_OPTIONS"
              :disabled="isWish"
              label="推荐程度"
            />
            <KunSelect
              v-model="spoilerLevel"
              :options="KUN_RATING_SPOILER_OPTIONS"
              :disabled="isWish"
              label="剧透程度"
            />
          </div>

          <KunTextarea
            v-model="shortSummary"
            label="短评（可选，纯文本，请勿包含 R18 内容）"
            description="短评会公开显示在所有 NextMoe 站点上"
            :rows="4"
            :maxlength="KUN_RATING_SUMMARY_MAX"
            :disabled="isWish"
            show-char-count
            auto-grow
          />

          <div class="space-y-3">
            <KunButton
              size="sm"
              variant="flat"
              :disabled="isWish"
              @click="aspectsOpen = !aspectsOpen"
            >
              {{ aspectsOpen ? '不填分项评分' : '填写分项评分（可选）' }}
            </KunButton>
            <div v-if="aspectsOpen" class="space-y-2">
              <div
                v-for="aspect in KUN_RATING_ASPECTS"
                :key="aspect.key"
                class="flex items-center gap-3"
              >
                <span class="text-default-600 w-8 shrink-0 text-sm">
                  {{ aspect.label }}
                </span>
                <KunRating
                  :model-value="aspects[aspect.key] ?? 0"
                  :max="10"
                  size="sm"
                  :disabled="isWish"
                  :aria-label="aspect.label"
                  @update:model-value="(v) => setAspect(aspect.key, v)"
                />
                <span class="text-default-500 w-5 text-sm tabular-nums">
                  {{ aspects[aspect.key] ?? '-' }}
                </span>
              </div>
            </div>
          </div>
        </div>

        <div class="bg-default-100 space-y-3 rounded-xl p-4">
          <div class="flex items-center justify-between gap-2">
            <span class="font-medium">游玩记录</span>
            <KunButton
              v-if="status"
              size="sm"
              variant="light"
              color="default"
              @click="status = ''"
            >
              清除状态
            </KunButton>
          </div>
          <KunRadioGroup
            v-model="status"
            :options="stateOptions"
            variant="pill"
            orientation="horizontal"
            size="sm"
            aria-label="游玩状态"
            class-name="flex-wrap"
          />
          <KunNumberInput
            v-if="!isWish"
            v-model="hours"
            label="游玩时长（小时，可选）"
            description="显示的是你在所有 NextMoe 应用里记录过的最长时长，保存后作为本站的记录"
            :min="0"
            :max="KUN_PLAYTIME_HOURS_MAX"
            :step="0.5"
            :precision="1"
            placeholder="例如 12.5"
          />
        </div>

        <div
          v-if="losingRating || losingPlaytime"
          class="text-danger space-y-1 text-sm"
        >
          <p v-if="losingRating">
            保存后会删除你的评分，所有 NextMoe 站点上都会一起消失。
          </p>
          <p v-if="losingPlaytime">
            保存后会撤回本站记录的游玩时长，其它应用记录的不受影响。
          </p>
        </div>

        <div class="flex flex-wrap items-center justify-between gap-2">
          <KunButton
            v-if="hadRating"
            variant="light"
            color="danger"
            :loading="deleting"
            :disabled="deleting || pending"
            @click="deleteRating"
          >
            <KunIcon name="lucide:trash-2" class="size-4" />
            删除评分
          </KunButton>
          <div class="ml-auto flex items-center gap-2">
            <KunButton variant="light" color="default" @click="open = false">
              取消
            </KunButton>
            <KunButton
              :color="losingRating ? 'danger' : 'primary'"
              :loading="pending"
              :disabled="pending || deleting"
              @click="save"
            >
              保存
            </KunButton>
          </div>
        </div>
      </template>
    </div>
  </KunModal>
</template>
