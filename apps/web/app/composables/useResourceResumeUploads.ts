// localStorage-backed registry of interrupted (started-but-not-completed)
// multipart patch-resource uploads, scoped per galgame. Lets the publish modal
// surface "you have N unfinished uploads" + a resume / delete list that survives
// a page reload — the artifact's already-uploaded parts live in B2, so a resume
// only re-sends the missing ones (see useResourceUpload + the artifact /resume
// endpoint).
//
// All access is guarded (SSR / private mode / quota): failures degrade to an
// empty list rather than throwing, so the worst case is "resume isn't offered".
export const useResourceResumeUploads = (galgameId: number) => {
  const key = `kun-patch-upload-resume:${galgameId}`

  const read = (): PatchPendingUpload[] => {
    if (!import.meta.client) return []
    try {
      const raw = localStorage.getItem(key)
      const parsed = raw ? JSON.parse(raw) : []
      return Array.isArray(parsed) ? (parsed as PatchPendingUpload[]) : []
    } catch {
      return []
    }
  }

  const write = (items: PatchPendingUpload[]) => {
    if (!import.meta.client) return
    try {
      localStorage.setItem(key, JSON.stringify(items))
    } catch {
      // private mode / quota — resume-across-reload just won't be offered
    }
  }

  // Newest first, so the list reads most-recent-on-top.
  const list = (): PatchPendingUpload[] =>
    read().sort((a, b) => b.updatedAt - a.updatedAt)

  // Insert or replace by uuid.
  const upsert = (record: PatchPendingUpload) => {
    write([
      ...read().filter((p) => p.artifactUuid !== record.artifactUuid),
      record
    ])
  }

  // Update the persisted progress as parts land — keeps the list accurate even
  // if the tab is closed mid-upload (no interruption event fires then).
  const setProgress = (artifactUuid: string, progress: number) => {
    const all = read()
    const record = all.find((p) => p.artifactUuid === artifactUuid)
    if (!record) return
    record.progress = progress
    record.updatedAt = Date.now()
    write(all)
  }

  const remove = (artifactUuid: string) => {
    write(read().filter((p) => p.artifactUuid !== artifactUuid))
  }

  return { list, upsert, setProgress, remove }
}
