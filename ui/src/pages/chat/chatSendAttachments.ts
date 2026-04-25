import { fileFromBase64 } from './chatAttachmentRules'

/** 待发送区图片（与 useChatPage pendingImages 一致） */
export type ChatPendingImage = { dataUrl: string; base64: string; mime: string }

/** 待发送区文档（与 useChatPage pendingFiles 一致） */
export type ChatPendingFile = { name: string; size: number; base64: string; mime: string; url?: string }

/**
 * 用户未输入文字、仅图/仅文件时，用于流式请求与气泡展示的占位文案。
 */
export function computeStreamMessageLabel (
  trimmedText: string,
  imgs: ChatPendingImage[],
  files: ChatPendingFile[],
  t: (key: string) => string
): string {
  if (trimmedText !== '') return trimmedText
  if (imgs.length > 0) return t('chatImageOnly')
  if (files.length > 0) return t('chatFileOnly')
  return ''
}

export type UploadChatFileFn = (file: File) => Promise<{ url: string; filename: string } | null>

/**
 * 顺序上传待发送图片与文档，返回 URL 列表；部分图片失败会触发 onPartialImageUpload。
 * 若用户带了图但全部上传失败则抛错（与原先 send 行为一致）。
 */
export async function uploadPendingChatAttachments (
  imgs: ChatPendingImage[],
  files: ChatPendingFile[],
  uploadFile: UploadChatFileFn,
  opts: {
    t: (key: string) => string
    onPartialImageUpload: () => void
  }
): Promise<{ image_urls: string[]; file_urls: string[] }> {
  const uploadedImageUrls: string[] = []
  for (let i = 0; i < imgs.length; i++) {
    const imgFile = fileFromBase64(imgs[i].base64, imgs[i].mime, `image-${i}.png`)
    const uploaded = await uploadFile(imgFile)
    if (uploaded) {
      uploadedImageUrls.push(uploaded.url)
    }
  }

  const uploadedFileUrls: string[] = []
  for (const f of files) {
    const fFile = fileFromBase64(f.base64, f.mime, f.name || 'file')
    const uploaded = await uploadFile(fFile)
    if (uploaded) {
      uploadedFileUrls.push(uploaded.url)
    }
  }

  if (imgs.length > 0 && uploadedImageUrls.length < imgs.length) {
    opts.onPartialImageUpload()
  }
  if (imgs.length > 0 && uploadedImageUrls.length === 0) {
    throw new Error(opts.t('chatPartialImageUploadFailed'))
  }

  return { image_urls: uploadedImageUrls, file_urls: uploadedFileUrls }
}
