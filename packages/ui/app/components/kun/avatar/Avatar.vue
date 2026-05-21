<script setup lang="ts">
import { getRandomSticker } from '../../../utils/getRandomSticker'
import type { KunAvatarProps } from './type'

const props = withDefaults(defineProps<KunAvatarProps>(), {
  size: 'md',
  isNavigation: true,
  className: '',
  imageClassName: ''
})

const handleClickAvatar = async (event: MouseEvent) => {
  event.preventDefault()
  if (!props.isNavigation || !props.user?.uid) return
  await navigateTo(`/user/${props.user.uid}/info`)
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
  const user = props.user
  if (user?.avatar) {
    return props.size === 'original' || props.size === 'original-sm'
      ? user.avatar
      : user.avatar.replace(/\.webp$/, '-100.webp')
  }
  // Fallback for null user or missing avatar — deterministic per name
  // so the same unknown user gets the same sticker. Empty string is a
  // safe key for `getRandomSticker`.
  return getRandomSticker(user?.name ?? '').value
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
    <KunImage
      :class="
        cn('inline-block rounded-full', sizeClasses, props.imageClassName)
      "
      :src="userAvatarSrc"
      :alt="user?.name ?? '未知用户'"
    />
  </div>
</template>
