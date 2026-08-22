<script setup lang="ts">
const props = defineProps<{
  staff: PatchDetailStaffGroup[]
}>()

const COLLAPSED = 6

const isExpanded = ref(false)
const isCollapsible = computed(() => props.staff.length > COLLAPSED)
const visible = computed(() =>
  isCollapsible.value && !isExpanded.value
    ? props.staff.slice(0, COLLAPSED)
    : props.staff
)

const playedText = (characters?: KunLanguage[]) =>
  (characters ?? []).map((c) => getPreferredLanguageText(c)).join(' / ')
</script>

<template>
  <section v-if="staff.length" class="space-y-4">
    <KunHeader
      name="制作人员"
      description="该 Galgame 的剧本, 原画, 音乐, 导演等制作人员名单, 资料来自 鲲 Galgame 目录"
      scale="h2"
    />

    <dl class="space-y-4">
      <div
        v-for="group in visible"
        :key="group.role_key"
        class="grid grid-cols-1 gap-x-4 gap-y-1 sm:grid-cols-[6rem_1fr]"
      >
        <dt class="text-default-500 pt-0.5 text-sm font-medium">
          {{ group.role_name }}
        </dt>
        <dd class="flex flex-wrap items-baseline gap-x-4 gap-y-1.5">
          <span v-for="person in group.people" :key="person.id" class="text-sm">
            <NuxtLink
              :to="`/galgame/staff/${person.id}`"
              class="text-default-800 hover:text-primary"
            >
              {{ getPreferredLanguageText(person.name) }}
            </NuxtLink>
            <span v-if="person.characters?.length" class="text-default-400">
              （{{ playedText(person.characters) }}）
            </span>
          </span>
        </dd>
      </div>
    </dl>

    <KunButton
      v-if="isCollapsible"
      variant="flat"
      color="primary"
      size="sm"
      @click="isExpanded = !isExpanded"
    >
      <KunIcon
        :name="isExpanded ? 'lucide:chevron-up' : 'lucide:chevron-down'"
      />
      {{
        isExpanded
          ? '收起制作人员'
          : `展开其余 ${staff.length - COLLAPSED} 项职位`
      }}
    </KunButton>
  </section>
</template>
