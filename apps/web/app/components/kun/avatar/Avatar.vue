<script setup lang="ts">
import type { KunAvatarProps } from './type'

const props = withDefaults(defineProps<KunAvatarProps>(), {
  size: 'md',
  isNavigation: true,
  className: '',
  imageClassName: '',
  disableFloating: false,
  floatingPosition: 'top'
})

// During the OAuth migration some response paths can transiently produce a
// missing user (e.g. comment.user not yet enriched via userclient, message
// sender that was a deleted user). Treat undefined / null user as an
// "anonymous" placeholder rather than crashing the whole page.
const safeUser = computed(() => ({
  id: props.user?.id ?? 0,
  name: props.user?.name ?? '',
  avatar: props.user?.avatar ?? ''
}))

const handleClickAvatar = async (event: MouseEvent) => {
  event.preventDefault()
  if (props.isNavigation && safeUser.value.id > 0) {
    await navigateTo(`/user/${safeUser.value.id}/info`)
  }
}

const sizeClasses = computed(() => {
  if (props.size === 'original') {
    return 'size-40'
  }
  if (props.size === 'original-sm') {
    return 'size-24'
  }

  if (props.size === 'xs') {
    return 'size-4'
  } else if (props.size === 'sm') {
    return 'size-6'
  } else if (props.size === 'md') {
    return 'size-8'
  } else if (props.size === 'lg') {
    return 'size-10'
  } else if (props.size === 'xl') {
    return 'size-12'
  } else {
    return 'size-8'
  }
})

const userAvatarSrc = computed(() => {
  const u = safeUser.value
  if (u.avatar) {
    return props.size === 'original' || props.size === 'original-sm'
      ? u.avatar
      : u.avatar.replace(/\.webp$/, '-100.webp')
  }
  return getRandomSticker(u.name).value
})
</script>

<template>
  <div
    :class="
      cn(
        'flex shrink-0 cursor-pointer justify-center',
        'rounded-full transition duration-150 ease-in-out hover:scale-110',
        sizeClasses,
        className
      )
    "
    @click="handleClickAvatar($event)"
  >
    <!-- <KunImage
          :class="cn('inline-block rounded-full', sizeClasses)"
          v-if="user.avatar"
          :src="userAvatarSrc"
          :alt="user.name"
        /> -->
    <KunImage
      :class="
        cn('inline-block rounded-full', sizeClasses, props.imageClassName)
      "
      :src="userAvatarSrc"
      :alt="safeUser.name"
    />
    <!-- <span
          :style="{ height: size, width: size }"
          :class="
            cn(
              'bg-default flex shrink-0 items-center justify-center rounded-full text-white',
              sizeClasses
            )
          "
          v-if="!user.avatar"
        >
          {{ user.name.slice(0, 1).toUpperCase() }}
        </span> -->
  </div>
</template>
