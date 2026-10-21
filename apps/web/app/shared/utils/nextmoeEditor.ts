export interface NextmoeEditorLinkInput {
  base: string
  workId: number
  returnUrl: string
  loginHint?: string
}

const editorUrl = (
  path: string,
  { base, returnUrl, loginHint }: Omit<NextmoeEditorLinkInput, 'workId'>
) => {
  const url = new URL(path, base)
  url.searchParams.set('from', 'moyu')
  url.searchParams.set('return', returnUrl)
  if (loginHint) {
    url.searchParams.set('login_hint', loginHint)
  }
  return url.toString()
}

export const nextmoeEditorWorkUrl = (input: NextmoeEditorLinkInput) =>
  editorUrl(`/works/${input.workId}/edit`, input)

export const nextmoeEditorHistoryUrl = (input: NextmoeEditorLinkInput) =>
  editorUrl(`/history/work/${input.workId}`, input)
