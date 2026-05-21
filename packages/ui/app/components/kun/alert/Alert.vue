<script setup lang="ts">
const { state, handleConfirm, handleCancel } = useKunAlertState()
</script>

<template>
  <KunModal
    :model-value="state.show"
    class-name="z-2000 fixed"
    @update:model-value="(value: boolean) => !value && handleCancel()"
  >
    <div class="max-w-80">
      <div class="space-y-2">
        <h3 v-if="state.title" class="text-lg">{{ state.title }}</h3>
        <p v-if="state.message" class="text-sm">{{ state.message }}</p>
      </div>

      <div class="mt-6 flex justify-end gap-2">
        <KunButton
          v-if="state.showCancel"
          variant="light"
          color="danger"
          @click="handleCancel"
        >
          取消
        </KunButton>
        <KunButton @click="handleConfirm">确定</KunButton>
      </div>
    </div>
  </KunModal>
</template>
