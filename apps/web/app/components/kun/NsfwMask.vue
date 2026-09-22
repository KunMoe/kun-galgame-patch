<script setup lang="ts">
interface Props {
  // Whether this item is NSFW. The stance is read here so the ~dozen call
  // sites do not each re-derive it (and forget the adult_confirmed half).
  nsfw?: boolean
  // Matches the corner radius of whatever is being covered; the overlay is a
  // sibling of it, not a child, so it cannot inherit one.
  rounded?: string
  label?: string
}

const props = withDefaults(defineProps<Props>(), {
  nsfw: false,
  rounded: 'rounded-lg',
  label: '成人向内容'
})

// Reads the two stores rather than useKunNsfwStance, which would build an API
// client this component never uses — once per card, on a page that draws
// dozens. The fold itself still lives in exactly one place.
const settingStore = useSettingStore()
const userStore = useUserStore()
const stance = computed(() =>
  resolveNsfwStance(settingStore.data, userStore.user)
)

const revealed = ref(false)

const masked = computed(
  () => props.nsfw && stance.value === 'blur' && !revealed.value
)

// The cover is usually wrapped in a link, and a <button> inside an <a> is not
// valid markup — so the overlay sits BESIDE the link and covers it, rather than
// inside it. That also makes the first click reveal instead of navigate.
const reveal = () => {
  revealed.value = true
}
</script>

<template>
  <div :class="cn('relative', masked && `overflow-hidden ${props.rounded}`)">
    <!-- The CSS filter, not the overlay's backdrop-filter, is what actually
         masks: backdrop-filter is the one of the two a browser may not support,
         and a mask that silently does nothing shows the cover it was asked to
         hide. The overlay's own tint is the second floor under the same risk. -->
    <div :class="cn(masked && 'pointer-events-none blur-lg saturate-50')">
      <slot />
    </div>
    <button
      v-if="masked"
      type="button"
      :class="
        cn(
          'absolute inset-0 flex flex-col items-center justify-center gap-1 bg-black/35 text-center text-white backdrop-blur-sm',
          props.rounded
        )
      "
      :aria-label="`${props.label}已模糊，点击查看`"
      @click.stop.prevent="reveal"
    >
      <KunIcon name="lucide:eye-off" class="size-5" />
      <span class="px-1 text-[11px] leading-tight">{{ props.label }}</span>
      <span class="px-1 text-[11px] leading-tight opacity-80">点击查看</span>
    </button>
  </div>
</template>
