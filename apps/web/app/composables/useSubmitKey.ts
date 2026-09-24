// One press of a create button keeps its key while the same payload is sent
// again after an answer that does not say whether it landed (a network error
// or a 5xx), so catalog hands back what the first try made instead of making
// it twice. Any definite answer ends the press: catalog remembers refusals for
// 24h too, and would replay one to the next deliberate try.
export const useSubmitKey = () => {
  let pending: { payload: string; key: string } | null = null

  const keyFor = (payload: unknown) => {
    const serialized = JSON.stringify(payload)
    if (pending?.payload !== serialized) {
      pending = { payload: serialized, key: crypto.randomUUID() }
    }
    return pending.key
  }

  // -1 is a request that never got an answer, a bare 5xx is a proxy's page, and
  // 5xxxx is this API's own server error.
  const settle = (code: number) => {
    const unknown = code === -1 || (code >= 500 && code < 600) || code >= 50000
    if (!unknown) pending = null
  }

  return { keyFor, settle }
}
