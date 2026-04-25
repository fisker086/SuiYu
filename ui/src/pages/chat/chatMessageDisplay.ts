import { api } from 'boot/axios'

export function getFileIcon (url: string): string {
  const ext = url.split('.').pop()?.toLowerCase() || ''
  const iconMap: Record<string, string> = {
    pdf: 'picture_as_pdf',
    doc: 'description',
    docx: 'description',
    xls: 'table_chart',
    xlsx: 'table_chart',
    txt: 'text_snippet',
    md: 'text_snippet'
  }
  return iconMap[ext] || 'insert_drive_file'
}

export function getFileName (url: string): string {
  const parts = url.split('/')
  return parts[parts.length - 1] || 'file'
}

/**
 * 将接口返回的路径拼成浏览器可用的绝对 URL。
 * - 已是 http(s)/data/blob 则原样返回
 * - 以 `/` 开头的相对路径：用 `new URL(path, base)`，base 为 axios 的绝对 baseURL，否则为 `window.location.origin`
 */
export function resolveChatImageUrl (pathOrUrl: string | undefined): string {
  if (!pathOrUrl) return ''
  if (pathOrUrl.startsWith('data:') || pathOrUrl.startsWith('blob:')) return pathOrUrl
  if (pathOrUrl.startsWith('http://') || pathOrUrl.startsWith('https://')) return pathOrUrl
  const base = (api.defaults.baseURL ?? '/api/v1').replace(/\/$/, '')
  if (!pathOrUrl.startsWith('/')) return pathOrUrl
  try {
    const baseForResolve = base.startsWith('http') ? base : window.location.origin
    return new URL(pathOrUrl, baseForResolve).href
  } catch {
    return pathOrUrl
  }
}

export function isSafeImagePreviewSrc (s: string): boolean {
  const t = s.trim()
  if (!t) return false
  const lower = t.toLowerCase()
  if (lower.startsWith('javascript:') || lower.startsWith('data:text/html')) return false
  if (lower.startsWith('data:image/')) return true
  if (lower.startsWith('blob:')) return true
  if (lower.startsWith('http://') || lower.startsWith('https://')) return true
  if (t.startsWith('/') && !t.startsWith('//')) return true
  return false
}

/** 与后端 userTextForMemory 一致：正文前的「[图片×N]」「[文件×N]」或组合（U+00D7） */
export const USER_ATTACHMENT_SUMMARY_RE =
  /^\[(?:图片×\d+|文件×\d+)(?:\s+(?:图片×\d+|文件×\d+))?\]\s*/

/** @deprecated 使用 USER_ATTACHMENT_SUMMARY_RE */
export const USER_IMAGE_SUMMARY_RE = /^\[图片×\d+\]\s*/

/**
 * 用户气泡展示字段：与 `ChatHistoryMessage`（GET /messages）一致，均为 snake_case。
 * - `image_urls` / `file_urls`：服务端路径或上传后的 URL
 * - `image_data_urls`：仅本页本地回合、未落库前的多图 data URL 预览（接口无此字段）
 */
export type UserMessageBubbleFields = {
  role: string
  content: string
  image_urls?: string[]
  file_urls?: string[]
  image_data_urls?: string[]
}

/** 气泡内用于 `<img src>` 的地址列表：优先本地预览，否则接口 URL */
export function userMessageImageUrls (m: Pick<UserMessageBubbleFields, 'image_urls' | 'image_data_urls'>): string[] {
  if (m.image_data_urls?.length) return m.image_data_urls
  if (m.image_urls?.length) return m.image_urls
  return []
}

/**
 * 用户气泡内文字：去掉与缩略图/附件列表重复的摘要前缀；无 URL 时勿只显示原始占位串。
 */
export function userMessageTextToDisplay (m: UserMessageBubbleFields, t: (key: string) => string): string {
  if (m.role !== 'user') return m.content ?? ''
  const raw = (m.content ?? '').trim()
  const imgUrls = userMessageImageUrls(m)
  const fileUrls = m.file_urls ?? []
  const afterStrip = raw.replace(USER_ATTACHMENT_SUMMARY_RE, '').trim()
  if (imgUrls.length > 0) {
    return afterStrip
  }
  if (fileUrls.length > 0) {
    return afterStrip
  }
  if (USER_ATTACHMENT_SUMMARY_RE.test(raw) && afterStrip === '') {
    if (/\[图片×/.test(raw)) return t('chatHistoryImageNoPreview')
    if (/\[文件×/.test(raw)) return t('chatFileOnly')
    return t('chatFileOnly')
  }
  return afterStrip || raw
}
