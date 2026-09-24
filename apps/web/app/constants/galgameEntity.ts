import type { KunUIColor } from '@kungal/ui-core'

// catalog's roster_role: main | secondary | appears | unknown. unknown is a
// character reached only through a voice credit, with no roster row to rank it.
export const GALGAME_CHARACTER_KIND_MAP: Record<string, string> = {
  main: '主角',
  secondary: '配角',
  appears: '登场',
  unknown: '未知'
}

export const GALGAME_CHARACTER_KIND_COLOR: Record<string, KunUIColor> = {
  main: 'primary'
}

export const GALGAME_CHARACTER_SPOILER_MAP: Record<number, string> = {
  1: '轻微剧透',
  2: '严重剧透'
}

export const GALGAME_STAFF_GENDER_MAP: Record<number, string> = {
  1: '男',
  2: '女',
  3: '其他'
}

// catalog's label_kind, which is what the 会社 chip carries. Rendering it raw
// puts "Dramatic Create · game_brand" on the page.
export const GALGAME_OFFICIAL_CATEGORY_MAP: Record<string, string> = {
  company: '公司',
  individual: '个人',
  amateur: '业余',
  game_brand: '游戏品牌',
  bunko: '文库',
  publisher: '发行商',
  anime_studio: '动画工作室',
  doujin_circle: '同人社团',
  group: '团体',
  other: '其它'
}

export const GALGAME_RATING_TIER_ORDER = [
  'bad',
  'average',
  'good',
  'masterpiece',
  'god'
] as const
export type GalgameRatingTierKey = (typeof GALGAME_RATING_TIER_ORDER)[number]

interface RatingTierRule {
  // The four lines between the five tiers, ascending, on the SOURCE's own scale
  // — erogamescape's are out of 100, dlsite's out of 5. Two sources are only
  // ever comparable by the tier they land in, never by these numbers.
  cutoffs: readonly [number, number, number, number]
  minVotes: number
}

export const GALGAME_RATING_TIER_MAP: Record<GalgameRatingTierKey, string> = {
  god: '神作',
  masterpiece: '名作',
  good: '良作',
  average: '平作',
  bad: '雷作'
}

export const GALGAME_RATING_TIER_COLOR: Record<
  GalgameRatingTierKey,
  KunUIColor
> = {
  god: 'warning',
  masterpiece: 'success',
  good: 'primary',
  average: 'default',
  bad: 'danger'
}

export const GALGAME_RATING_TIER_DESCRIPTION: Record<
  GalgameRatingTierKey,
  string
> = {
  god: '该来源打分最高的一小撮作品',
  masterpiece: '该来源公认的口碑作',
  good: '该来源评价良好, 值得一玩',
  average: '该来源评价平平, 见仁见智',
  bad: '该来源评价偏低, 入坑需谨慎'
}

interface ExternalRatingMeta {
  label: string
  // The source's own maximum. Catalog ships every source on its native scale
  // and normalizes nothing, so this is the divisor any cross-source comparison
  // has to use before it means anything.
  max: number
  suffix: string
  hint: string
  format: (score: number) => string
  // The bucket keys this source's histogram uses, ascending. The payload is
  // sparse: a key catalog does not send is a real zero, not missing data.
  buckets: number[]
  bucketLabel: (key: number) => string
  // Calibrated per source against the whole catalog corpus rather than
  // converted from one shared curve: the same work sits at bangumi 7.0 and
  // dlsite 4.5, so one set of numbers cannot serve both. DLsite polls
  // self-selected buyers and runs about a tier high, which is why its lines are
  // the tightest.
  tier: RatingTierRule
  link?: (ref: string) => string
}

const pointBuckets = (max: number) =>
  Array.from({ length: max }, (_, index) => index + 1)

const decimal = (score: number) => String(Number(score.toFixed(2)))

export const KUN_EXTERNAL_RATING_ORDER = [
  'vndb',
  'bangumi',
  'erogamescape',
  'dlsite'
] as const

export const KUN_EXTERNAL_RATING_MAP: Record<string, ExternalRatingMeta> = {
  vndb: {
    label: 'VNDB',
    max: 10,
    suffix: '/10',
    hint: 'VNDB 用户评分的加权平均',
    format: decimal,
    buckets: pointBuckets(10),
    bucketLabel: String,
    tier: { cutoffs: [5.5, 6.5, 7.4, 8.0], minVotes: 10 },
    link: (ref) => `https://vndb.org/${ref}`
  },
  bangumi: {
    label: 'Bangumi',
    max: 10,
    suffix: '/10',
    hint: 'Bangumi 用户评分的平均值',
    format: decimal,
    buckets: pointBuckets(10),
    bucketLabel: String,
    // Bangumi's own vote labels sit on exactly these integers: 5 不过不失,
    // 6 还行, 7 推荐, 8 力荐.
    tier: { cutoffs: [5.0, 6.0, 7.0, 8.0], minVotes: 10 },
    link: (ref) => `https://bgm.tv/subject/${ref}`
  },
  erogamescape: {
    label: '批评空间',
    max: 100,
    suffix: '点台',
    hint: '批评空间用户评分的中位数',
    // 批评空间 quotes a score as the band it falls in ("75点台"), so this
    // truncates rather than rounds: 75.8 is still the 75 band.
    format: (score) => String(Math.floor(score)),
    // Its histogram is deciles keyed by the lower bound, where 100 is the single
    // top score rather than a range. Folding those onto a 1-100 point axis would
    // leave 90 empty columns and drop the 0 bucket entirely.
    buckets: [0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100],
    bucketLabel: (key) => (key === 100 ? '100' : `${key}-${key + 9}`),
    // 80点 is the line the community itself quotes, and it stays even though it
    // is stricter here than the other sources' 名作 lines: the famous "80点 =
    // 上位34%" figure describes single VOTES, and per-work medians are far more
    // concentrated — only ~8.5% of works clear 80.
    tier: { cutoffs: [60, 70, 80, 85], minVotes: 10 }
  },
  dlsite: {
    label: 'DLsite',
    max: 5,
    suffix: '/5',
    hint: 'DLsite 购买者的平均星级',
    format: decimal,
    buckets: pointBuckets(5),
    bucketLabel: String,
    tier: { cutoffs: [3.8, 4.2, 4.5, 4.7], minVotes: 10 }
  },
  // catalog carries HowLongToBeat on its 0-100 scale in 5-point buckets, which
  // the out-of-10 fallback below cannot read. Its tier lines have not been
  // calibrated against the corpus like the others', so it carries no badge.
  howlongtobeat: {
    label: 'HowLongToBeat',
    max: 100,
    suffix: '/100',
    hint: 'HowLongToBeat 用户评分',
    format: (score) => String(Math.round(score)),
    buckets: Array.from({ length: 21 }, (_, index) => index * 5),
    bucketLabel: String,
    tier: { cutoffs: [60, 70, 80, 90], minVotes: Number.POSITIVE_INFINITY }
  }
}

export const externalRatingMeta = (source: string): ExternalRatingMeta =>
  KUN_EXTERNAL_RATING_MAP[source] ?? {
    label: source,
    max: 10,
    suffix: '',
    hint: '',
    format: decimal,
    buckets: pointBuckets(10),
    bucketLabel: String,
    tier: { cutoffs: [5.5, 6.5, 7.4, 8.0], minVotes: 10 }
  }

export interface GalgameRatingTierBadge {
  label: string
  color: KunUIColor
  description: string
}

export const ratingTierBadge = (
  meta: ExternalRatingMeta,
  score: number,
  voteCount: number
): GalgameRatingTierBadge | null => {
  if (voteCount < meta.tier.minVotes) {
    return null
  }
  const key =
    GALGAME_RATING_TIER_ORDER[
      meta.tier.cutoffs.filter((cutoff) => score >= cutoff).length
    ]
  if (!key) {
    return null
  }
  return {
    label: GALGAME_RATING_TIER_MAP[key],
    color: GALGAME_RATING_TIER_COLOR[key],
    description: GALGAME_RATING_TIER_DESCRIPTION[key]
  }
}
