<script setup lang="ts">
const loadingLogin = ref(false)
const loadingRegister = ref(false)

const handleLogin = async () => {
  loadingLogin.value = true
  try {
    await startOAuthLogin()
  } catch {
    loadingLogin.value = false
  }
}

const handleRegister = async () => {
  loadingRegister.value = true
  try {
    await startOAuthRegister()
  } catch {
    loadingRegister.value = false
  }
}
</script>

<template>
  <div class="flex w-72 flex-col gap-4">
    <div class="flex flex-col items-center gap-2">
      <KunImage
        src="/nextmoe.webp"
        alt="NextMoe·未萌"
        :width="48"
        :height="48"
        object-fit="cover"
        class-name="border-default-200 size-12 shrink-0 overflow-hidden rounded-xl border"
      />
      <p class="text-default-500 text-center text-sm">
        本站账号由
        <span class="text-foreground font-medium">NextMoe·未萌 账号</span>
        统一管理
      </p>
    </div>

    <KunButton
      color="primary"
      size="lg"
      full-width
      :loading="loadingLogin"
      :disabled="loadingLogin || loadingRegister"
      @click="handleLogin"
    >
      <KunIcon v-if="!loadingLogin" name="lucide:log-in" class="size-5" />
      使用 NextMoe·未萌 账号登录
    </KunButton>

    <KunButton
      color="primary"
      variant="bordered"
      size="lg"
      full-width
      :loading="loadingRegister"
      :disabled="loadingLogin || loadingRegister"
      @click="handleRegister"
    >
      <KunIcon v-if="!loadingRegister" name="lucide:user-plus" class="size-5" />
      注册 NextMoe·未萌 账号
    </KunButton>

    <p class="text-default-400 text-center text-xs">
      即原「鲲 Galgame 账号」，账号与数据不变
    </p>
  </div>
</template>
