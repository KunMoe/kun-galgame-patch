<script setup lang="ts">
// Report entry point mounted at every reportable content site. It only TRIGGERS
// the single global report modal (useReportModal → <ReportModal> at app root),
// so it's safe inside a ⋯ KunPopover without the modal being torn down when the
// popover closes. A new content type costs one <ReportButton> mount with its
// subject_kind + subject_id (+ optional snapshot / subject_url).
const props = withDefaults(
  defineProps<{
    subjectKind: string
    subjectId: string | number
    snapshot?: string
    // Absolute deep-link to the reported content (built by the caller), so
    // moderators can jump straight to it from the console.
    subjectUrl?: string
    label?: string
    // `menu` renders a full-width row (for ⋯ popover menus); default is a
    // compact icon-only button (for action bars).
    menu?: boolean
  }>(),
  { snapshot: '', subjectUrl: '', label: '举报', menu: false }
)

const { requireLogin } = useAuthModal()
const { open } = useReportModal()

const trigger = () => {
  // Not logged in → open the global login modal instead (uniform site-wide).
  if (!requireLogin()) return
  open({
    subjectKind: props.subjectKind,
    subjectId: props.subjectId,
    snapshot: props.snapshot,
    subjectUrl: props.subjectUrl
  })
}
</script>

<template>
  <KunButton
    v-if="menu"
    variant="light"
    color="danger"
    size="sm"
    class-name="w-full justify-start gap-2 whitespace-nowrap"
    @click="trigger"
  >
    <KunIcon class-name="text-lg" name="lucide:flag" />{{ label }}
  </KunButton>

  <KunTooltip v-else :text="label">
    <KunButton
      :is-icon-only="true"
      color="danger"
      variant="light"
      size="sm"
      @click="trigger"
    >
      <KunIcon name="lucide:flag" />
    </KunButton>
  </KunTooltip>
</template>
