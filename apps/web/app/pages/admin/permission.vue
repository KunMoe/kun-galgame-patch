<script setup lang="ts">
useKunDisableSeo('权限管理')

const tabs = [
  { value: 'role', textValue: '角色权限', icon: 'lucide:users' },
  { value: 'user', textValue: '用户权限', icon: 'lucide:user-cog' },
  { value: 'change', textValue: '变更日志', icon: 'lucide:history' }
]
const active = ref('role')
</script>

<template>
  <div class="space-y-4">
    <div>
      <h1 class="text-2xl font-bold">权限管理</h1>
      <p class="text-default-500 text-sm">
        默认权限由代码基线定义；这里的调整是相对基线的授予 /
        撤销，保存后一分钟内同步到所有实例。前端只决定显示什么，真正的边界始终在后端。
      </p>
    </div>

    <KunTab
      v-model="active"
      :items="tabs"
      variant="underlined"
      color="primary"
      size="sm"
    />

    <AdminPermissionRoleMatrix v-if="active === 'role'" />
    <AdminPermissionUserOverrides v-else-if="active === 'user'" />
    <AdminPermissionChangeLog v-else />
  </div>
</template>
