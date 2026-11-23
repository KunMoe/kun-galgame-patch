type ChipColor = 'default' | 'primary' | 'success' | 'warning' | 'danger'

export const ENV_KIND: Record<
  EnvVariableKind,
  { label: string; color: ChipColor }
> = {
  bootstrap: { label: '启动接线', color: 'danger' },
  topology: { label: '拓扑', color: 'default' },
  secret: { label: '密钥', color: 'warning' },
  plain: { label: '普通', color: 'primary' }
}

export const ENV_SOURCE: Record<EnvValueSource, string> = {
  env: '环境变量',
  store: '配置中心',
  default: '代码默认'
}

export const ENV_ISSUE: Record<
  EnvVariableIssue,
  { label: string; hint: string; color: ChipColor }
> = {
  panel_redundant: {
    label: '面板值可删除',
    hint: '环境变量与存储值一致, 从 Dokploy 面板删掉后重新部署',
    color: 'success'
  },
  panel_overrides_store: {
    label: '面板值覆盖了配置中心',
    hint: '两者不一致, 生效的是环境变量。确认哪个对, 删面板值或改存储值',
    color: 'danger'
  },
  restart_pending: {
    label: '待重启生效',
    hint: '存储值在 API 启动后改过, 重启 moyu-api 后生效',
    color: 'warning'
  },
  not_read: {
    label: '没有代码读取',
    hint: '配置中心存着它, 但 moyu-api 不读: 可以删除这条存储值',
    color: 'warning'
  }
}

export const ENV_CHANGE: Record<EnvChange['change_kind'], string> = {
  set: '写入',
  delete: '删除',
  import: '导入'
}

export const envValueText = (value: EnvValue | null) => {
  if (!value) return '—'
  if (!value.is_set) return '(空)'
  if (value.plain !== null) return value.plain
  return `${value.fingerprint ?? ''} ${value.hint ?? ''}`.trim()
}
