export interface NextmoeEditorLinkInput {
  base: string
  workId: number
  returnUrl: string
  loginHint?: string
}

export const nextmoeEditorWorkUrl = ({
  base,
  workId,
  returnUrl,
  loginHint
}: NextmoeEditorLinkInput) => {
  const url = new URL(`/works/${workId}/edit`, base)
  url.searchParams.set('from', 'moyu')
  url.searchParams.set('return', returnUrl)
  if (loginHint) {
    url.searchParams.set('login_hint', loginHint)
  }
  return url.toString()
}
