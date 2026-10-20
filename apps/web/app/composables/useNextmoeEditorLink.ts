import { kunMoyuMoe } from '~/config/moyu-moe'

export const useNextmoeEditorLink = () => {
  const config = useRuntimeConfig()
  const userStore = useUserStore()

  return (workId: number, returnPath = `/galgame/${workId}`) =>
    nextmoeEditorWorkUrl({
      base: config.public.editorUrl,
      workId,
      returnUrl: `${kunMoyuMoe.domain.main}${returnPath}`,
      loginHint: userStore.user.sub || undefined
    })
}
