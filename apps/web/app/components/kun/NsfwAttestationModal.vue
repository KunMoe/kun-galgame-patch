<script setup lang="ts">
// The site never asks "are you 18?" itself. Age attestation is an identity
// operation owned by the account centre (OAuth content-preferences contract
// §四), so the only thing moyu does with an 18008 is hand over the link.
const { attestationOpen } = useKunNsfwStance()

const config = useRuntimeConfig()
const oauthWebUrl =
  (config.public.oauthWebUrl as string) || 'https://account.nextmoe.com'
const settingsUrl = `${oauthWebUrl}/settings`

const goToAccountCentre = () => {
  attestationOpen.value = false
  if (import.meta.client) window.open(settingsUrl, '_blank', 'noopener')
}
</script>

<template>
  <KunModal
    v-model="attestationOpen"
    inner-class-name="max-w-sm"
    aria-label="需要完成年龄确认"
  >
    <div class="space-y-4 text-center">
      <KunIcon
        name="lucide:shield-alert"
        class="text-warning-500 mx-auto text-4xl"
      />
      <div class="space-y-1">
        <p class="text-lg font-semibold">需要先完成年龄确认</p>
        <p class="text-default-600 text-sm leading-relaxed">
          显示成人向内容需要您在 NextMoe 账号中心确认已年满 18
          周岁。确认一次后全站生效，返回本页重新选择即可。
        </p>
      </div>
      <div class="flex justify-center gap-2">
        <KunButton
          variant="flat"
          color="default"
          @click="attestationOpen = false"
        >
          稍后再说
        </KunButton>
        <KunButton color="primary" @click="goToAccountCentre">
          前往账号中心
        </KunButton>
      </div>
    </div>
  </KunModal>
</template>
