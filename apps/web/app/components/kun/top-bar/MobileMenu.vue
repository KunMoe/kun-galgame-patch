<script setup lang="ts">
import { kunMoyuMoe } from '~/config/moyu-moe'
import { kunMobileAdminItem, kunMobileNavItem } from '~/constants/top-bar'

const userStore = useUserStore()
const navItems = computed(() =>
  userStore.isAdmin
    ? [...kunMobileNavItem, ...kunMobileAdminItem]
    : kunMobileNavItem
)

interface Props {
  isOpen: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{ 'update:isOpen': [value: boolean] }>()

const closeMenu = () => emit('update:isOpen', false)

watch(
  () => props.isOpen,
  (v) => {
    if (import.meta.client) {
      document.body.style.overflow = v ? 'hidden' : ''
    }
  }
)

onUnmounted(() => {
  if (import.meta.client) document.body.style.overflow = ''
})
</script>

<template>
  <Transition
    enter-active-class="transition-opacity duration-200"
    leave-active-class="transition-opacity duration-200"
    enter-from-class="opacity-0"
    leave-to-class="opacity-0"
  >
    <div
      v-if="props.isOpen"
      class="bg-background/95 fixed inset-0 top-16 z-40 overflow-y-auto px-4 py-6 backdrop-blur md:hidden"
    >
      <NuxtLink
        class="mb-6 flex items-center"
        to="/"
        @click="closeMenu"
      >
        <KunImage
          src="/favicon.webp"
          :alt="kunMoyuMoe.titleShort"
          :width="50"
          :height="50"
          class-name="rounded-2xl"
        />
        <p class="mr-2 ml-4 font-bold">
          {{ kunMoyuMoe.creator.name }}
        </p>
        <KunChip size="sm" variant="flat" color="primary"> 补丁 </KunChip>
      </NuxtLink>

      <nav class="flex flex-col gap-1">
        <NuxtLink
          v-for="item in navItems"
          :key="item.href"
          :to="item.href"
          class="hover:bg-default-100 rounded-lg px-3 py-3"
          @click="closeMenu"
        >
          {{ item.name }}
        </NuxtLink>
      </nav>
    </div>
  </Transition>
</template>
