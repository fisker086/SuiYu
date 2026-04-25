import DOMPurify from 'dompurify'
import { marked } from 'marked'

marked.setOptions({
  gfm: true,
  breaks: true
})

/** Render assistant Markdown to sanitized HTML for v-html (DOMPurify strips scripts/on*). */
export function renderChatMarkdown (text: string): string {
  const t = text?.trim()
  if (!t) return ''
  const html = marked.parse(t, { async: false }) as string
  return DOMPurify.sanitize(html, {
    USE_PROFILES: { html: true }
  })
}
