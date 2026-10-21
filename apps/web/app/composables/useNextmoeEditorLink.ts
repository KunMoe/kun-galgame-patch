import { kunMoyuMoe } from '~/config/moyu-moe'
import type { NextmoeEditorLinkInput } from '~/shared/utils/nextmoeEditor'

const useEditorLink =
  (build: (input: NextmoeEditorLinkInput) => string) => () => {
    const config = useRuntimeConfig()
    const userStore = useUserStore()

    return (workId: number, returnPath = `/galgame/${workId}`) =>
      build({
        base: config.public.editorUrl,
        workId,
        returnUrl: `${kunMoyuMoe.domain.main}${returnPath}`,
        loginHint: userStore.user.sub || undefined
      })
  }

export const useNextmoeEditorLink = useEditorLink(nextmoeEditorWorkUrl)

export const useNextmoeEditorHistoryLink = useEditorLink(
  nextmoeEditorHistoryUrl
)
