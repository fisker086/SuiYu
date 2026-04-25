import {
  DOCUMENT_BUTTON_EXT,
  IMAGE_BUTTON_EXT,
  isAllowedDocumentButtonFile,
  isAllowedImageButtonFile
} from './chatAttachmentRules'

type LogFn = (msg: string, meta?: Record<string, unknown>) => void

/**
 * 输入框内粘贴：图片走待发送预览；PDF/TXT/MD/JSON 走「上传文件」同款逻辑。
 * 与原先 useChatPage.onComposerPaste 一致。
 */
export type ComposerPasteDeps = {
  canStartSession: () => boolean
  maxChatImages: number
  maxChatImageBytes: number
  getPendingImageCount: () => number
  t: (key: string, params?: Record<string, number>) => string
  logChatAttach: LogFn
  notifyWarning: (message: string) => void
  setPendingImageFromFile: (file: File) => void
  enqueuePendingDocumentFromFile: (file: File) => void
}

export function handleComposerPaste (e: ClipboardEvent, d: ComposerPasteDeps): void {
  if (!d.canStartSession()) {
    d.logChatAttach('paste ignored: canStartSession false (select agent/workflow first)')
    return
  }
  const cd = e.clipboardData
  if (!cd) return

  const tryAcceptImageFile = (file: File | null): boolean => {
    if (!file || file.size === 0) return false
    const mime = (file.type || '').toLowerCase().trim()
    if (mime !== '' && !mime.startsWith('image/')) return false
    if (mime === '') {
      const lower = file.name.toLowerCase()
      if (DOCUMENT_BUTTON_EXT.test(lower) && !IMAGE_BUTTON_EXT.test(lower)) return false
    }
    e.preventDefault()
    if (!isAllowedImageButtonFile(file)) {
      d.notifyWarning(d.t('chatImageTypeNotAllowed'))
      return true
    }
    if (d.getPendingImageCount() >= d.maxChatImages) {
      d.notifyWarning(d.t('chatMaxImages', { n: d.maxChatImages }))
      return true
    }
    if (file.size > d.maxChatImageBytes) {
      d.notifyWarning(d.t('chatImageTooLarge'))
      return true
    }
    d.setPendingImageFromFile(file)
    return true
  }

  const tryAcceptDocumentFile = (file: File | null): boolean => {
    if (!file || file.size === 0) return false
    if (!isAllowedDocumentButtonFile(file)) return false
    e.preventDefault()
    d.enqueuePendingDocumentFromFile(file)
    return true
  }

  const { files } = cd
  if (files && files.length > 0) {
    let any = false
    for (let i = 0; i < files.length; i++) {
      const file = files[i]
      if (tryAcceptImageFile(file)) {
        any = true
        continue
      }
      if (tryAcceptDocumentFile(file)) {
        any = true
        continue
      }
      const mime = (file.type || '').toLowerCase().trim()
      if (mime !== '' && !mime.startsWith('image/')) {
        e.preventDefault()
        d.notifyWarning(d.t('chatDocumentTypeNotAllowed'))
        any = true
      }
    }
    if (any) return
  }

  const { items } = cd
  if (!items?.length) return
  for (let i = 0; i < items.length; i++) {
    const item = items[i]
    if (item.kind !== 'file') continue
    const file = item.getAsFile()
    if (tryAcceptImageFile(file)) return
    if (tryAcceptDocumentFile(file)) return
    const mime = (item.type || '').toLowerCase().trim()
    if (mime !== '' && !mime.startsWith('image/')) {
      e.preventDefault()
      d.notifyWarning(d.t('chatDocumentTypeNotAllowed'))
      return
    }
  }
}

export type ComposerDragDeps = {
  canStartSession: () => boolean
  maxChatImages: number
  maxChatImageBytes: number
  getPendingImageCount: () => number
  t: (key: string, params?: Record<string, number>) => string
  logChatAttach: LogFn
  notifyWarning: (message: string) => void
  setPendingImageFromFile: (file: File) => void
  enqueuePendingDocumentFromFile: (file: File) => void
}

export function handleComposerDragOver (e: DragEvent, d: ComposerDragDeps): void {
  if (!d.canStartSession()) return
  e.preventDefault()
  if (e.dataTransfer) e.dataTransfer.dropEffect = 'copy'
}

/** 拖入输入区：与文件夹按钮、粘贴一致，支持图片与允许类型的文档 */
export function handleComposerDrop (e: DragEvent, d: ComposerDragDeps): void {
  if (!d.canStartSession()) {
    d.logChatAttach('drop ignored: canStartSession false')
    return
  }
  e.preventDefault()
  const dt = e.dataTransfer
  if (!dt?.files?.length) return
  const list = Array.from(dt.files)
  d.logChatAttach(`onComposerDrop files=${list.length}`)
  for (const file of list) {
    const mime = (file.type || '').toLowerCase().trim()
    if (mime.startsWith('image/') || (mime === '' && IMAGE_BUTTON_EXT.test(file.name.toLowerCase()))) {
      if (!isAllowedImageButtonFile(file)) {
        d.notifyWarning(d.t('chatImageTypeNotAllowed'))
        continue
      }
      if (d.getPendingImageCount() >= d.maxChatImages) {
        d.notifyWarning(d.t('chatMaxImages', { n: d.maxChatImages }))
        break
      }
      if (file.size > d.maxChatImageBytes) {
        d.notifyWarning(d.t('chatImageTooLarge'))
        continue
      }
      d.setPendingImageFromFile(file)
      continue
    }
    if (isAllowedDocumentButtonFile(file)) {
      d.enqueuePendingDocumentFromFile(file)
      continue
    }
    if (mime !== '' && !mime.startsWith('image/')) {
      d.notifyWarning(d.t('chatDocumentTypeNotAllowed'))
    }
  }
}
