import { SUPPORTED_TYPE_MAP } from '~/constants/resource'
import type { KunSiteConfig } from './config'

export const kunMoyuMoe: KunSiteConfig = {
  title: '鲲 Galgame 补丁 - 开源 Galgame 补丁资源下载站',
  titleShort: '鲲 Galgame 补丁',
  template: '%s - 鲲 Galgame 补丁',
  description:
    '开源, 免费, 零门槛, 纯手写, 最先进的 Galgame 补丁资源下载站, 提供 Windows, 安卓, KRKR, Tyranor 等各类平台的 Galgame 补丁资源下载。永远免费！',
  keywords: [
    'Galgame',
    '资源',
    '下载',
    '补丁',
    '网站',
    '免费',
    '开源',
    'Nuxt',
    // Exclude the R18 类别 label (r18: 'R18 成人内容补丁') from site-wide SEO
    // keywords — adult-content terms shouldn't ride on every page's <meta>.
    ...Object.entries(SUPPORTED_TYPE_MAP)
      .filter(([key]) => key !== 'r18')
      .map(([, label]) => label)
  ],
  canonical: 'https://www.moyu.moe',
  author: [
    { name: '鲲', url: 'https://soft.moe' },
    { name: '鲲 Galgame', url: 'https://nav.kungal.org' },
    { name: '鲲 Galgame 论坛', url: 'https://www.kungal.com' }
  ],
  creator: {
    name: '鲲 Galgame',
    mention: '@kungalgame',
    url: 'https://nav.kungal.org'
  },
  publisher: {
    name: '鲲 Galgame',
    mention: '@kungalgame',
    url: 'https://nav.kungal.org'
  },
  domain: {
    main: 'https://www.moyu.moe',
    // The shared image_service CDN (kun-galgame-infra's KUN_IMAGE_PUBLIC_BASE_URL).
    // MUST equal the backend's KUN_IMAGE_CDN_BASE so both sides build identical
    // {imageBed}/aa/bb/<hash>[_variant].webp URLs (image_service object-key layout).
    imageBed: 'https://image.kungal.iloveren.link',
    storage: 'https://oss.moyu.moe',
    kungal: 'https://www.kungal.com',
    telegram_group: 'https://t.me/kungalgame',
    cluster: 'https://nav.kungal.org'
  },
  og: {
    title: '鲲 Galgame 补丁 - 开源 Galgame 补丁资源下载站',
    description:
      '开源, 免费, 零门槛的 Galgame 补丁资源下载站, 提供 Windows, 安卓, KRKR, Tyranor 等各类平台的 Galgame 补丁资源下载。最先进的 Galgame 补丁资源站！永远免费！',
    image: 'https://moyu.moe/kungalgame.webp',
    url: 'https://www.moyu.moe'
  },
  images: [
    {
      url: 'https://moyu.moe/kungalgame.webp',
      width: 1920,
      height: 1080,
      alt: '鲲 Galgame 补丁 - 开源 Galgame 补丁资源下载站'
    }
  ],
  ad: [
    {
      name: 'AIEro',
      url: 'https://umami.kungal.org/q/tuMHj3J6B'
    }
  ]
}
