<script setup lang="ts">
import {
  KUN_EXTERNAL_RATING_ORDER,
  externalRatingMeta,
  ratingTierBadge
} from '~/constants/galgameEntity'

const props = defineProps<{
  ratings: PatchDetailRating[]
  vndbId?: string
  bangumiId?: number | null
  erogamescapeId?: number | null
}>()

const refOf = (source: string) => {
  const ids: Record<string, string | number | null | undefined> = {
    vndb: props.vndbId,
    bangumi: props.bangumiId,
    erogamescape: props.erogamescapeId
  }
  return ids[source] ? String(ids[source]) : ''
}

const cards = computed(() => {
  const rank = (source: string) => {
    const at = KUN_EXTERNAL_RATING_ORDER.indexOf(
      source as (typeof KUN_EXTERNAL_RATING_ORDER)[number]
    )
    return at < 0 ? KUN_EXTERNAL_RATING_ORDER.length : at
  }

  return [...props.ratings]
    .sort((a, b) => rank(a.source) - rank(b.source))
    .map((row) => {
      const meta = externalRatingMeta(row.source)
      const ref = refOf(row.source)
      const counts = new Map(
        (row.distribution ?? []).map((b) => [b.score, b.count])
      )
      const peak = Math.max(1, ...counts.values())

      return {
        source: row.source,
        label: meta.label,
        hint: meta.hint,
        score: meta.format(row.score),
        suffix: meta.suffix,
        // Every source runs its own scale, so the bar is share-of-max within
        // this source only — never a number to compare across two cards.
        fill: `${Math.min(100, Math.round((row.score / meta.max) * 100))}%`,
        votes: row.vote_count.toLocaleString('en-US'),
        tier: ratingTierBadge(meta, row.score, row.vote_count),
        rank: row.rank,
        href: ref ? (meta.link?.(ref) ?? '') : '',
        bars: counts.size
          ? meta.buckets.map((key) => ({
              key,
              label: meta.bucketLabel(key),
              count: counts.get(key) ?? 0,
              height: `${Math.round(((counts.get(key) ?? 0) / peak) * 100)}%`
            }))
          : []
      }
    })
})
</script>

<template>
  <section v-if="cards.length" class="space-y-4">
    <KunHeader
      name="外部评分"
      description="各站点各自的刻度, 分级按各来源自己的标准独立计算, 不跨站点换算"
      scale="h2"
    />

    <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
      <div
        v-for="card in cards"
        :key="card.source"
        class="bg-default-100 space-y-3 rounded-xl p-4"
      >
        <div class="flex items-center justify-between gap-2">
          <div class="flex min-w-0 items-center gap-1.5">
            <KunTooltip :text="card.hint">
              <span class="text-default-600 text-sm font-medium">
                {{ card.label }}
              </span>
            </KunTooltip>
            <KunTooltip v-if="card.tier" :text="card.tier.description">
              <KunChip :color="card.tier.color" variant="flat" size="xs">
                {{ card.tier.label }}
              </KunChip>
            </KunTooltip>
          </div>
          <a
            v-if="card.href"
            :href="card.href"
            target="_blank"
            rel="noopener noreferrer"
            class="text-default-400 hover:text-primary"
            :aria-label="`在 ${card.label} 查看`"
          >
            <KunIcon name="lucide:external-link" class="size-4" />
          </a>
        </div>

        <div class="flex items-baseline justify-between gap-2">
          <span class="flex items-baseline gap-0.5">
            <span class="text-3xl leading-none font-semibold tabular-nums">
              {{ card.score }}
            </span>
            <span class="text-default-500 text-sm leading-none font-medium">
              {{ card.suffix }}
            </span>
          </span>
          <span class="text-default-500 text-xs tabular-nums">
            {{ card.votes }} 人
          </span>
        </div>

        <div class="bg-default-200 h-1.5 overflow-hidden rounded-full">
          <div
            class="bg-primary h-full rounded-full"
            :style="{ width: card.fill }"
          />
        </div>

        <div v-if="card.bars.length" class="flex h-14 items-end gap-px">
          <KunTooltip
            v-for="bar in card.bars"
            :key="bar.key"
            :text="`${bar.label}: ${bar.count} 人`"
            class-name="h-full flex-1"
          >
            <div class="flex h-full w-full items-end">
              <div
                :class="
                  cn(
                    'w-full rounded-t-sm',
                    bar.count ? 'bg-primary/70' : 'bg-default-200'
                  )
                "
                :style="{ height: bar.count ? bar.height : '2px' }"
              />
            </div>
          </KunTooltip>
        </div>

        <p v-if="card.rank" class="text-default-500 text-xs">
          站内排名 #{{ card.rank }}
        </p>
      </div>
    </div>
  </section>
</template>
