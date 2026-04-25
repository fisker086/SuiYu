/** 由已有 base64 构建 File，供 /chat/upload 使用（勿用未调用 read 的 FileReader，否则会永久挂起） */
export function fileFromBase64 (base64: string, mime: string, filename: string): File {
  const binaryString = atob(base64)
  const bytes = new Uint8Array(binaryString.length)
  for (let i = 0; i < binaryString.length; i++) {
    bytes[i] = binaryString.charCodeAt(i)
  }
  return new File([bytes], filename, { type: mime || 'application/octet-stream' })
}

/** 与后端 `/chat/upload`、桌面端一致 */
export const IMAGE_BUTTON_EXT = /\.(png|jpe?g|gif|webp)$/i
export const DOCUMENT_BUTTON_EXT = /\.(pdf|txt|md|json)$/i

export const CHAT_IMAGE_INPUT_ACCEPT =
  'image/png,image/jpeg,image/gif,image/webp,.png,.jpg,.jpeg,.gif,.webp'

export const CHAT_DOCUMENT_INPUT_ACCEPT =
  '.pdf,.txt,.md,.json,application/pdf,text/plain,text/markdown,text/x-markdown,application/json'

/** 去掉 `; charset=...` 等参数，避免 `text/plain; charset=UTF-8` 误判为不支持 */
export function mimeBaseType (mime: string): string {
  const m = (mime || '').toLowerCase().trim()
  if (m === '') return ''
  const semi = m.indexOf(';')
  return semi >= 0 ? m.slice(0, semi).trim() : m
}

export function isAllowedImageButtonFile (file: File): boolean {
  const mime = mimeBaseType(file.type || '')
  if (mime === 'image/png' || mime === 'image/jpeg' || mime === 'image/gif' || mime === 'image/webp') {
    return true
  }
  if (mime.startsWith('image/')) {
    return false
  }
  return IMAGE_BUTTON_EXT.test(file.name)
}

export function isAllowedDocumentButtonFile (file: File): boolean {
  const mime = mimeBaseType(file.type || '')
  if (mime.startsWith('image/')) return false
  if (mime === 'application/pdf') return true
  if (mime === 'text/plain' || mime === 'text/markdown' || mime === 'text/x-markdown') return true
  if (mime === 'application/json') return true
  return DOCUMENT_BUTTON_EXT.test(file.name)
}

export function inferImageMimeFromFile (file: File): string {
  const m = (file.type || '').toLowerCase().trim()
  if (m === 'image/png' || m === 'image/jpeg' || m === 'image/gif' || m === 'image/webp') return m
  const lower = file.name.toLowerCase()
  if (lower.endsWith('.png')) return 'image/png'
  if (lower.endsWith('.jpg') || lower.endsWith('.jpeg')) return 'image/jpeg'
  if (lower.endsWith('.gif')) return 'image/gif'
  if (lower.endsWith('.webp')) return 'image/webp'
  return 'image/png'
}
