// A folder is a catalog folder (nextmoe-infra /v2/me/folders), shared with the
// forum: the same shelf, whichever site you file from. This site's API is a
// thin face over it, see apps/api/internal/patch/service/folders.go.
//
// `id` is the CATALOG folder id, not a patch id.
type FolderVisibility = 'public' | 'private'

interface Folder {
  id: number
  name: string
  description: string
  visibility: FolderVisibility
  is_default: boolean
  item_count: number
  created: string
  updated: string
  // image_service hashes, not URLs — the card builds the URL. Fewer than
  // item_count, and sometimes none: the shelf is shared with kungal and can
  // hold games this site has no page for, and the NSFW gate drops rows here
  // like it does everywhere else.
  preview_covers: string[]
}

interface FolderMembership extends Folder {
  contains: boolean
}
