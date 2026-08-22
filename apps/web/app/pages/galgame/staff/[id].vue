<script setup lang="ts">
import { GALGAME_STAFF_GENDER_MAP } from '~/constants/galgameEntity'
import { imageServiceUrl } from '~/shared/utils/resolveBannerUrl'

const route = useRoute()
const api = useApi()

const staffId = computed(() => Number((route.params as { id: string }).id))

const { data } = await useAsyncData<PatchStaffDetail | null>(
  () => `galgame-staff-${staffId.value}`,
  async () => {
    const res = await api.get<PatchStaffDetail>(`/galgame/staff/${staffId.value}`)
    return res.code === 0 ? res.data : null
  }
)

const staff = computed(() => data.value)

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

const roleText = (credit: PatchStaffCredit) =>
  credit.roles
    .map((r) => (r.character ? `${r.role_name}（${r.character}）` : r.role_name))
    .join(' · ')

useHead(() => ({
  title: name.value ? `${name.value} 参与制作的 Galgame` : '制作人员'
}))
</script>

<template>
  <div
    v-if="staff"
    class="mx-auto w-full max-w-7xl space-y-6 px-3 py-4"
  >
    <div class="bg-content1 shadow-kun-sm rounded-3xl p-6 sm:p-8">
      <div class="flex items-start gap-4">
        <KunImage
          v-if="photoSrc"
          :src="photoSrc"
          :alt="name"
          loading="eager"
          aspect-ratio="1/1"
          object-fit="cover"
          class-name="w-24 shrink-0 overflow-hidden rounded-xl sm:w-32"
        />

        <div class="min-w-0 grow space-y-2">
          <h1 class="text-2xl font-bold break-words sm:text-3xl">
            {{ name }}
          </h1>
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
      </div>

      <div v-if="intro" class="mt-4 space-y-1">
        <p class="text-default-600 text-sm whitespace-pre-line">
          {{ intro.intro }}
        </p>
        <p class="text-default-400 text-xs">
          资料来自 {{ intro.source || '鲲 Galgame 目录'
          }}<template v-if="intro.machine">, 由机器翻译</template>
        </p>
      </div>

      <div v-if="staff.siblings.length" class="mt-4 space-y-1.5">
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
      </div>

      <div v-if="staff.links.length" class="mt-4 flex flex-wrap gap-3">
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
      </div>
    </div>

    <section v-if="staff.credits.length" class="space-y-4">
      <KunHeader
        name="参与作品"
        :description="`共 ${staff.credits.length} 部`"
        scale="h2"
      />

      <ul class="space-y-2">
        <li
          v-for="(credit, index) in staff.credits"
          :key="`${credit.galgame_id}-${index}`"
          class="bg-content1 shadow-kun-sm flex flex-wrap items-baseline gap-x-3 gap-y-1 rounded-2xl px-4 py-3"
        >
          <NuxtLink
            v-if="credit.galgame_id"
            :to="`/patch/${credit.galgame_id}`"
            class="text-default-800 hover:text-primary"
          >
            {{ getPreferredLanguageText(credit.name) }}
          </NuxtLink>
          <span v-else class="text-default-600">
            {{ getPreferredLanguageText(credit.name) }}
          </span>
          <span class="text-default-400 text-xs">{{ roleText(credit) }}</span>
        </li>
      </ul>
    </section>
  </div>

  <KunNull v-else description="未找到该制作人员" />
</template>
