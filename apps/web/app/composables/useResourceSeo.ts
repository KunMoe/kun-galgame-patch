// Everything the resource detail page tells a crawler, lifted out of the page
// (which is already large) into one place.
//
// Follows the house pattern set by patch/[id].vue: hand-built ld+json as a plain
// object, JSON.stringify'd into a <script>, with every empty field dropped so a
// crawler never sees a blank property. Deliberately no schema-dts dependency —
// kungal uses one, moyu does not, and one page is not worth the divergence.
//
// The guiding rule, same as patch/[id].vue: stay HONEST to the data moyu
// actually has. No aggregateRating is emitted, because resources are not rated —
// only liked. Inventing a rating is what earns a structured-data manual action,
// and a star that isn't backed by anything is worse than no star.
import {
  SUPPORTED_RESOURCE_LINK_MAP,
  SUPPORTED_TYPE_MAP,
  SUPPORTED_LANGUAGE_MAP,
  SUPPORTED_PLATFORM_MAP
} from '~/constants/resource'
import { kunMoyuMoe } from '~/config/moyu-moe'

interface Options {
  // The composed <title> ({game}{platform}{language}{model}{type}资源下载).
  // Built by the page because its two halves also render the visible <h1>.
  title: Ref<string>
  // Approved root-comment count, for commentCount / the interaction counter.
  commentCount: Ref<number>
}

const BCP47 = new Set(['zh-Hans', 'zh-Hant', 'ja', 'en'])

export const useResourceSeo = (
  // Nullable AND undefined-able: useAsyncData's data is `T | null | undefined`
  // until it settles.
  detail: Ref<PatchResourceDetail | null | undefined>,
  { title, commentCount }: Options
) => {
  const route = useRoute()

  const resource = computed(() => detail.value?.resource ?? null)
  const patch = computed(() => detail.value?.patch ?? null)
  const patchName = computed(() =>
    patch.value ? getPreferredLanguageText(patch.value.name) : ''
  )

  const labels = (keys: string[] | undefined, map: Record<string, string>) =>
    (keys ?? []).map((k) => map[k] ?? k).filter(Boolean)

  const noteText = computed(() =>
    resource.value?.note_html
      ? resource.value.note_html.replace(/<[^>]+>/g, '').trim().slice(0, 140)
      : ''
  )

  // The uploader's own note is the most specific text this page has, so it wins.
  // The fallback is assembled from moyu's own attributes so it stays unique per
  // resource rather than one template repeated across thousands of pages.
  const description = computed(() => {
    if (noteText.value) return noteText.value
    const r = resource.value
    if (!r) return `${patchName.value} 的补丁资源下载`
    const attrs = [
      labels(r.platform, SUPPORTED_PLATFORM_MAP).join('、'),
      labels(r.language, SUPPORTED_LANGUAGE_MAP).join('、'),
      labels(r.type, SUPPORTED_TYPE_MAP).join('、')
    ]
      .filter(Boolean)
      .join(' · ')
    const storage =
      SUPPORTED_RESOURCE_LINK_MAP[r.storage] ?? r.storage ?? ''
    return (
      `《${patchName.value}》补丁资源免费下载` +
      (attrs ? `：${attrs}` : '') +
      (r.size ? `，文件 ${r.size}` : '') +
      (storage ? `，${storage}` : '') +
      '，支持 aria2 多线程加速下载与 BLAKE3 完整性校验。'
    )
  })

  const apply = () => {
    const r = resource.value
    const p = patch.value

    // NSFW owning game, or nothing loaded → no indexable metadata at all. This
    // page exposes the game's name and the uploader's note, so it must not feed
    // OpenGraph or rich results for an adult title.
    if (!detail.value || !r || !p || p.content_limit !== 'sfw') {
      useKunDisableSeo(title.value || '补丁资源')
      return
    }

    const canonicalUrl = `${kunMoyuMoe.domain.main}${route.path}`
    const gameUrl = `${kunMoyuMoe.domain.main}/galgame/${r.galgame_id}`
    // Un-varianted (full size) for social cards: bannerSrc on the page is the
    // 460x259 `mini` thumbnail, which is well under the 1200x630 that Twitter /
    // OpenGraph want for a large card.
    const cover = resolveBannerUrl(p)

    const published = new Date(r.created).toISOString()
    const modified = new Date(
      (r.update_time as string) || r.created
    ).toISOString()

    const platformLabels = labels(r.platform, SUPPORTED_PLATFORM_MAP)
    const typeLabels = labels(r.type, SUPPORTED_TYPE_MAP)
    const languageLabels = labels(r.language, SUPPORTED_LANGUAGE_MAP)
    // inLanguage wants language CODES, not the Chinese display labels — the raw
    // keys already are BCP-47 ('zh-Hans' / 'ja' / …). 'other' is not a language.
    const languageCodes = (r.language ?? []).filter((l) => BCP47.has(l))

    const keywords = [
      patchName.value,
      ...typeLabels,
      ...languageLabels,
      ...platformLabels,
      r.model_name || ''
    ].filter(Boolean)

    const alternateNames = Object.values(p.name).filter(
      (n): n is string => !!n && n !== patchName.value
    )

    useKunSeoMeta({
      title: title.value,
      description: description.value,
      ogType: 'article',
      ...(cover && { ogImage: cover }),
      ...(r.user?.id && {
        articleAuthor: [`${kunMoyuMoe.domain.main}/user/${r.user.id}/resource`]
      }),
      articlePublishedTime: published,
      articleModifiedTime: modified
    })

    // A patch is a piece of software you install, so SoftwareApplication is the
    // type that actually fits — VideoGame is the GAME, which lives on the wiki
    // and is modelled here only as the thing this patch is `about`.
    //
    // COMPUTED, not a plain object: the comment count comes from a fetch that is
    // still in flight when this runs, so a snapshot taken here would ship
    // commentCount: 0 on every resource that has comments. useHead re-serializes
    // when it settles.
    const softwareLd = computed<Record<string, unknown>>(() => ({
      '@context': 'https://schema.org',
      '@type': 'SoftwareApplication',
      name: r.name || `${patchName.value} 的补丁资源`,
      url: canonicalUrl,
      applicationCategory: 'GameApplication',
      description: description.value,
      datePublished: published,
      dateModified: modified,
      // Free, and say so in the two ways consumers look for it.
      isAccessibleForFree: true,
      offers: {
        '@type': 'Offer',
        price: 0,
        priceCurrency: 'CNY',
        availability: 'https://schema.org/InStock',
        url: canonicalUrl
      },
      // Deliberately NO downloadUrl: the file link is only revealed on this
      // page, and handing crawlers the raw blob URL would route people straight
      // past the notes, the change history and the problem reports.
      ...(cover && { image: cover }),
      ...(r.size && { fileSize: r.size }),
      ...(platformLabels.length && { operatingSystem: platformLabels }),
      ...(languageCodes.length && { inLanguage: languageCodes }),
      ...(keywords.length && { keywords: keywords.join(', ') }),
      // A patch is useless without the game it patches — state the dependency.
      ...(patchName.value && {
        softwareRequirements: `《${patchName.value}》游戏本体`,
        about: {
          '@type': 'VideoGame',
          name: patchName.value,
          url: gameUrl,
          ...(alternateNames.length && { alternateName: alternateNames }),
          ...(cover && { image: cover })
        }
      }),
      ...(r.user?.name && {
        author: {
          '@type': 'Person',
          name: r.user.name,
          ...(r.user.id && {
            url: `${kunMoyuMoe.domain.main}/user/${r.user.id}/resource`
          })
        }
      }),
      // The page's own engagement. DownloadAction is the one that actually
      // describes what this page is for.
      interactionStatistic: [
        {
          '@type': 'InteractionCounter',
          interactionType: { '@type': 'DownloadAction' },
          userInteractionCount: r.download
        },
        {
          '@type': 'InteractionCounter',
          interactionType: { '@type': 'LikeAction' },
          userInteractionCount: r.like_count
        },
        {
          '@type': 'InteractionCounter',
          interactionType: { '@type': 'CommentAction' },
          userInteractionCount: commentCount.value
        }
      ],
      commentCount: commentCount.value,
      discussionUrl: canonicalUrl
    }))

    const breadcrumbLd: Record<string, unknown> = {
      '@context': 'https://schema.org',
      '@type': 'BreadcrumbList',
      itemListElement: [
        {
          '@type': 'ListItem',
          position: 1,
          name: '首页',
          item: kunMoyuMoe.domain.main
        },
        {
          '@type': 'ListItem',
          position: 2,
          name: 'Galgame 补丁',
          item: `${kunMoyuMoe.domain.main}/galgame`
        },
        // Position 3 uses the game page's canonical /galgame/:id, never a
        // ?tab= variant — a breadcrumb pointing at one just asks the crawler to
        // reconcile two URLs for the same page.
        {
          '@type': 'ListItem',
          position: 3,
          name: patchName.value,
          item: gameUrl
        },
        {
          '@type': 'ListItem',
          position: 4,
          name: r.name || '补丁资源',
          item: canonicalUrl
        }
      ]
    }

    useHead({
      script: [
        {
          id: 'schema-org-software-application',
          type: 'application/ld+json',
          innerHTML: computed(() => JSON.stringify(softwareLd.value))
        },
        {
          id: 'schema-org-breadcrumb',
          type: 'application/ld+json',
          innerHTML: JSON.stringify(breadcrumbLd)
        }
      ]
    })
  }

  apply()
}
