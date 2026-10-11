<script setup lang="ts">
import { pickRoleBadge, USER_ROLE_MAP } from '~/constants/user'

const props = defineProps<{
  user: SearchUser
  keywords?: string
}>()

// One chip, the highest rank the account holds — a name followed by four of
// them is a badge collection, not an identity. The 普通用户 fallback is dropped
// entirely: a grid of twelve results would otherwise be twelve identical chips.
const role = computed(() =>
  pickRoleBadge(props.user.roles, props.user.site_roles)
)
const hasRole = computed(() => role.value.label !== USER_ROLE_MAP.user)

const stats = computed(() => [
  { icon: 'lucide:lollipop', label: '萌萌点', value: props.user.moemoepoint },
  {
    icon: 'lucide:gamepad-2',
    label: '发布的 Galgame',
    value: props.user.patch_count
  },
  {
    icon: 'lucide:puzzle',
    label: '发布的补丁资源',
    value: props.user.resource_count
  },
  {
    icon: 'lucide:message-square',
    label: '评论',
    value: props.user.comment_count
  }
])
</script>

<template>
  <KunCard
    :href="`/user/${user.id}/resource`"
    :is-hoverable="true"
    padding="none"
  >
    <div class="flex w-full gap-3 p-3">
      <KunAvatar
        :user="toKunUser(user)"
        :is-navigation="false"
        :disable-floating="true"
        size="lg"
        class-name="shrink-0"
      />

      <div class="min-w-0 flex-1 space-y-1">
        <div class="flex items-center gap-2">
          <span class="truncate text-sm font-medium">
            <SearchHighlight :text="user.name" :keywords="keywords" />
          </span>
          <KunChip
            v-if="hasRole"
            size="xs"
            variant="flat"
            :color="role.site ? 'secondary' : 'primary'"
          >
            {{ role.label }}
          </KunChip>
        </div>

        <p
          v-if="user.bio"
          class="text-default-600 line-clamp-2 text-xs break-all"
        >
          <SearchHighlight :text="user.bio" :keywords="keywords" />
        </p>

        <div
          class="text-default-500 flex flex-wrap items-center gap-x-3 text-xs tabular-nums"
        >
          <span
            v-for="stat in stats"
            :key="stat.icon"
            class="flex items-center gap-1"
            :title="stat.label"
          >
            <KunIcon :name="stat.icon" class="size-3.5" />
            {{ formatNumber(stat.value) }}
          </span>
        </div>
      </div>
    </div>
  </KunCard>
</template>
