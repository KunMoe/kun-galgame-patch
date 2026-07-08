import { kunMoyuMoe } from '~/config/moyu-moe'
import type {
  ActiveHeadEntry,
  UseHeadOptions,
  UseSeoMetaInput
} from '@unhead/vue'
import type { NuxtApp } from '#app/nuxt'

interface NuxtUseHeadOptions extends UseHeadOptions {
  nuxt?: NuxtApp
}

/**
 * title, description, ogType?, ogImage? required
 */
export const useKunSeoMeta = (
  input: Omit<
    UseSeoMetaInput,
    | 'ogUrl'
    | 'ogTitle'
    | 'ogDescription'
    | 'twitterCard'
    | 'twitterTitle'
    | 'twitterDescription'
    | 'twitterImage'
    | 'twitterImageAlt'
  >,
  options?: NuxtUseHeadOptions,
  // Override the canonical + og:url path (defaults to the current route). Used to
  // consolidate a tabbed route onto ONE canonical URL — e.g. every /patch/:id/*
  // tab points at the 补丁资源下载 tab so the tabs don't compete as duplicates.
  canonicalPath?: string
  // eslint-disable-next-line @typescript-eslint/no-invalid-void-type
): ActiveHeadEntry<UseSeoMetaInput> | void => {
  const title = `${input.title?.toString()} - ${kunMoyuMoe.title}`
  const description = input.description?.toString()
  const route = useRoute()

  const pageUrl = `${kunMoyuMoe.domain.main}${canonicalPath ?? route.path}`
  const image = input.ogImage
    ? input.ogImage
    : kunMoyuMoe.images[0]
      ? kunMoyuMoe.images[0].url
      : '/kungalgame.webp'

  useSeoMeta(
    {
      title,
      description,
      keywords: kunMoyuMoe.keywords.toString(),
      ogUrl: pageUrl,
      ogType: input.ogType || 'website',
      ogTitle: title,
      ogDescription: description,
      ogImage: image,
      ogImageAlt: title,
      twitterCard: 'summary_large_image',
      twitterTitle: title,
      twitterDescription: description,
      twitterImage: image,
      twitterImageAlt: title,
      ...input
    },
    options
  )

  useHead({
    link: [{ rel: 'canonical', href: pageUrl }]
  })
}
