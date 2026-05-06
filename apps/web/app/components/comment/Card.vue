<script setup lang="ts">
import DOMPurify from 'isomorphic-dompurify'

interface Props {
  comment: PatchComment
}

const props = defineProps<Props>()

const patchName = computed(() =>
  props.comment.patch?.name
    ? getPreferredLanguageText(props.comment.patch.name)
    : `补丁 #${props.comment.patch_id}`
)

const contentHtml = computed(() =>
  DOMPurify.sanitize(props.comment.content_html || '', {
    ADD_ATTR: ['data-uid']
  })
)
</script>

<template>
  <KunCard
    is-pressable
    :href="`/patch/${props.comment.patch_id}/comment`"
    class-name="w-full"
  >
    <div class="flex gap-4">
      <KunAvatar :user="props.comment.user" />
      <div class="space-y-2">
        <div class="flex flex-wrap items-center gap-2">
          <h2 class="font-semibold">{{ props.comment.user.name }}</h2>
          <span class="text-small text-default-500">
            评论在
            <span class="text-primary-500">{{ patchName }}</span>
          </span>
        </div>
        <div class="kun-prose mt-1" v-html="contentHtml" />
        <div class="mt-2 flex items-center gap-4">
          <div class="text-small text-default-500 flex items-center gap-1">
            <KunIcon name="lucide:thumbs-up" class="size-3.5" />
            {{ props.comment.like_count }}
          </div>
          <span class="text-small text-default-500">
            {{
              formatDate(props.comment.created, {
                isPrecise: true,
                isShowYear: true
              })
            }}
          </span>
        </div>
      </div>
    </div>
  </KunCard>
</template>
