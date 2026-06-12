<script setup lang="ts">
// W2 / PR3b — manage the covers array of a galgame (image_service hashes).
//
// Scope: pin / demote / remove rows on the EXISTING covers list. Upload of a
// brand-new banner uses the parent form's multipart `file` flow (PUT
// /galgame/:gid multipart → Wiki PromoteCoverHash auto-pins to sort_order=0).
// That separation avoids requiring `galgame_banner` preset to be enabled for
// moyu's OAuth client on image_service (Wiki has it; moyu typically doesn't).
//
// The `sort_order` invariant Wiki enforces (DB partial unique idx):
//   - at most one row with sort_order = 0 (the "pinned banner")
// So "pin row X" = swap: set X.sort_order=0; whoever was at 0 → 1.

import { imageServiceUrl } from '~/shared/utils/resolveBannerUrl'

const props = defineProps<{
  modelValue: GalgameCoverRow[]
}>()
const emit = defineEmits<{
  'update:modelValue': [GalgameCoverRow[]]
}>()

// Sort by sort_order asc for display; pinned (sort_order=0) shows first.
const sorted = computed(() =>
  [...props.modelValue].sort((a, b) => a.sort_order - b.sort_order)
)

const pin = (hash: string) => {
  const next = props.modelValue.map((c) =>
    c.image_hash === hash
      ? { ...c, sort_order: 0 }
      : c.sort_order === 0
        ? { ...c, sort_order: 1 }
        : c
  )
  emit('update:modelValue', next)
}

const remove = async (hash: string) => {
  const ok = await useKunAlert({
    title: '移除封面',
    message: '确定移除该封面？将在保存后从 Wiki 集合里删除。'
  })
  if (!ok) return
  emit(
    'update:modelValue',
    props.modelValue.filter((c) => c.image_hash !== hash)
  )
}
</script>

<template>
  <div class="border-default/20 space-y-3 rounded-xl border p-3">
    <div class="flex items-center justify-between">
      <p class="text-foreground text-sm font-semibold">封面集合</p>
      <p class="text-default-400 text-xs">
        sort_order=0 那张即当前 banner（最多一张）
      </p>
    </div>
    <p v-if="!sorted.length" class="text-default-400 text-xs">
      暂无封面。通过上方"新 Banner"上传一张即可。
    </p>
    <div v-else class="grid grid-cols-2 gap-3 sm:grid-cols-3">
      <div
        v-for="c in sorted"
        :key="c.image_hash"
        class="border-default/20 relative rounded-lg border p-2"
      >
        <KunImage
          :src="imageServiceUrl(c.image_hash, 'mini') || imageServiceUrl(c.image_hash)"
          :alt="c.image_hash.slice(0, 8)"
          aspect-ratio="16 / 9"
          class-name="bg-default-100 rounded"
        />
        <div class="mt-2 flex items-center justify-between">
          <span
            v-if="c.sort_order === 0"
            class="bg-primary/15 text-primary rounded px-1.5 py-0.5 text-xs font-medium"
          >
            当前 Banner
          </span>
          <span v-else class="text-default-400 text-xs">
            #{{ c.sort_order }}
          </span>
          <div class="flex gap-1">
            <KunButton
              v-if="c.sort_order !== 0"
              variant="light"
              size="sm"
              @click="pin(c.image_hash)"
            >
              设为 Banner
            </KunButton>
            <KunButton
              variant="light"
              color="danger"
              size="sm"
              @click="remove(c.image_hash)"
            >
              移除
            </KunButton>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
