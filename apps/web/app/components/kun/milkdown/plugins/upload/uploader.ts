import { Decoration } from '@milkdown/prose/view'
import type { Uploader } from '@milkdown/plugin-upload'
import type { Node } from '@milkdown/prose/model'

// Upload one image to moyu's image_service proxy and return a domain-agnostic
// content token `/image/<hash>` (or null on failure). We store the TOKEN, not an
// absolute CDN URL (image_service 契约 04 §"内容内嵌图的域名无关引用"): rendered
// HTML resolves it server-side (goldmark → CDN), and the web /image/:hash 302
// route resolves it for the editor preview / raw markdown. "换域名只改一处配置".
//
// apiBase is passed in rather than read here because this module runs outside
// Vue setup context (ProseMirror paste/drop handlers), where useRuntimeConfig()
// is unavailable. Contract: multipart { preset, file } → { code, data: { hash } }.
export const uploadEditorImage = async (
  apiBase: string,
  file: File
): Promise<string | null> => {
  const formData = new FormData()
  formData.append('preset', 'topic')
  formData.append('file', file, file.name)
  try {
    const res = await $fetch<{
      code: number
      message: string
      data: { hash: string } | null
    }>(`${apiBase}/upload/image-service`, {
      method: 'POST',
      body: formData,
      credentials: 'include'
    })
    return res.code === 0 && res.data ? `/image/${res.data.hash}` : null
  } catch {
    return null
  }
}

// Factory so the milkdown upload plugin gets an Uploader bound to the runtime
// apiBase captured in Editor.vue's setup.
export const createKunUploader =
  (apiBase: string): Uploader =>
  async (files, schema) => {
    const images: File[] = []
    for (let i = 0; i < files.length; i++) {
      const file = files.item(i)
      if (!file || !file.type.startsWith('image/')) {
        continue
      }
      images.push(file)
    }

    const nodes = await Promise.all(
      images.map(async (image) => {
        const src = await uploadEditorImage(apiBase, image)
        if (!src) {
          return null
        }
        return schema.nodes.image!.createAndFill({
          src,
          alt: image.name
        }) as Node
      })
    )

    return nodes.filter((node): node is Node => node !== null)
  }

export const kunUploadWidgetFactory = (
  pos: number,
  spec: Parameters<typeof Decoration.widget>[2]
) => {
  const widgetDOM = document.createElement('span')
  widgetDOM.textContent = '正在上传中...'
  widgetDOM.style.color = 'var(--color-primary)'
  return Decoration.widget(pos, widgetDOM, spec)
}
