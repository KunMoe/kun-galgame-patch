<script setup lang="ts">
import { GALGAME_STAFF_GENDER_MAP } from '~/constants/galgameEntity'
import { imageServiceUrl } from '~/shared/utils/resolveBannerUrl'

defineOptions({ name: 'staff-detail' })

const route = useRoute()
const api = useApi()

const staffID = computed(() => Number(route.params.id))

const VERDICT_NOT_FOUND = 40400

const { data, pending } = await useAsyncData(
  () => `staff-detail-${staffID.value}`,
  async () => {
    const res = await api.get<PatchStaffDetail>(
      `/galgame/staff/${staffID.value}`
    )
    return { code: res.code, detail: res.code === 0 ? res.data : null }
  },
  { watch: [staffID] }
)

const notFound = () => kunPageError(404, '制作人员不存在')

if (data.value?.code === VERDICT_NOT_FOUND) throw notFound()
watch(data, (v) => {
  if (v?.code === VERDICT_NOT_FOUND) showError(notFound())
})

const staff = computed(() => data.value?.detail ?? null)

const works = useEntityWorks(
  () => `/galgame/staff/${staffID.value}/works`,
  staff
)

const name = computed(() => getPreferredLanguageText(staff.value?.name))
const secondaryName = computed(() =>
  getSecondaryLanguageText(staff.value?.name, name.value)
)

const photoSrc = computed(() => imageServiceUrl(staff.value?.photo_hash ?? ''))

const intro = computed(() => pickPreferredLanguageRow(staff.value?.intros))

const aliases = computed(() =>
  (staff.value?.aliases ?? []).filter(
    (alias) => alias !== name.value && alias !== secondaryName.value
  )
)

// birth_y is often absent where the month and day are known, so the two halves
// render independently rather than as one date.
const birthday = computed(() => {
  const d = staff.value
  if (!d) return ''
  const md = d.birth_m && d.birth_d ? `${d.birth_m} 月 ${d.birth_d} 日` : ''
  const year = d.birth_y ? `${d.birth_y} 年` : ''
  return [year, md].filter(Boolean).join(' ')
})

const facts = computed(() => {
  const rows: string[] = []
  const gender = staff.value?.gender
  if (gender && GALGAME_STAFF_GENDER_MAP[gender]) {
    rows.push(GALGAME_STAFF_GENDER_MAP[gender])
  }
  if (birthday.value) rows.push(`生日 ${birthday.value}`)
  return rows
})

const roleText = (work: PatchEntityWork) =>
  (work.roles ?? [])
    .map((r) =>
      r.character ? `${r.role_name}（${r.character}）` : r.role_name
    )
    .join(' · ')

if (staff.value) {
  useKunSeoMeta({
    title: `制作人员 · ${name.value}`,
    description: `${name.value} 参与制作的 Galgame 作品与资料`
  })
} else {
  useKunDisableSeo('制作人员资料')
}
</script>

<template>
  <div class="container mx-auto my-6">
    <KunLoading v-if="pending && !staff" description="加载中..." />

    <KunNull v-else-if="!staff" description="制作人员加载失败，请稍后重试" />

    <template v-else>
      <section class="flex items-start gap-4">
        <KunImage
          v-if="photoSrc"
          :src="photoSrc"
          :alt="name"
          loading="eager"
          aspect-ratio="1/1"
          object-fit="cover"
          class-name="w-24 shrink-0 overflow-hidden rounded-xl sm:w-32"
        />

        <div class="min-w-0 grow space-y-1">
          <h1 class="text-2xl font-bold break-words sm:text-3xl">{{ name }}</h1>
          <p v-if="secondaryName" class="text-default-400 text-sm">
            {{ secondaryName }}
          </p>
          <p v-if="facts.length" class="text-default-500 text-sm">
            {{ facts.join(' · ') }}
          </p>
          <p v-if="aliases.length" class="text-default-500 text-sm">
            别名 {{ aliases.join(' / ') }}
          </p>
        </div>
      </section>

      <section v-if="intro" class="mt-6 space-y-1">
        <p class="text-default-600 text-sm whitespace-pre-line">
          {{ intro.intro }}
        </p>
        <p class="text-default-400 text-xs">
          资料来自 {{ intro.source || '鲲 Galgame 目录'
          }}<template v-if="intro.machine">, 由机器翻译</template>
        </p>
      </section>

      <section v-if="staff.siblings.length" class="mt-6 space-y-1.5">
        <p class="text-default-400 text-xs">其他名义</p>
        <div class="flex flex-wrap items-baseline gap-x-4 gap-y-1.5">
          <NuxtLink
            v-for="sibling in staff.siblings"
            :key="sibling.id"
            :to="`/galgame/staff/${sibling.id}`"
            class="text-default-800 hover:text-primary text-sm"
          >
            {{ getPreferredLanguageText(sibling.name) }}
          </NuxtLink>
        </div>
      </section>

      <section
        v-if="staff.links.length"
        class="mt-6 flex flex-wrap items-center gap-x-4 gap-y-2"
      >
        <a
          v-for="link in staff.links"
          :key="link.url"
          :href="link.url"
          target="_blank"
          rel="noopener noreferrer"
          class="text-default-500 hover:text-primary text-sm"
        >
          {{ link.name }}
          <KunIcon name="lucide:external-link" class="inline size-3" />
        </a>
      </section>

      <section class="mt-8 space-y-4">
        <KunHeader
          name="参与作品"
          description="该制作人员参与的 Galgame, 资料来自 鲲 Galgame 目录"
          scale="h2"
        />

        <KunNull
          v-if="!works.items.value.length"
          description="暂无该制作人员参与的 Galgame"
        />

        <GalgameList v-else :items="works.items.value">
          <template #meta="{ item }">
            <p
              v-if="item.roles?.length"
              class="text-default-500 mt-1 line-clamp-2 text-xs"
              :title="roleText(item)"
            >
              {{ roleText(item) }}
            </p>
          </template>
        </GalgameList>

        <div v-if="works.hasMore.value" class="flex justify-center">
          <KunButton
            variant="flat"
            color="primary"
            :is-loading="works.isLoading.value"
            @click="works.loadMore"
          >
            加载更多作品
          </KunButton>
        </div>
      </section>
    </template>
  </div>
</template>
