/**
 * Multiline chat input: **Enter** sends, **Shift+Enter** inserts a newline.
 * During IME composition (e.g. Chinese Pinyin), **Enter** confirms the word — does not send.
 */
export function onChatInputEnterToSend (
  e: KeyboardEvent,
  send: () => void | Promise<void>
): void {
  if (e.key !== 'Enter') return
  if (e.shiftKey) return
  if (e.isComposing) return
  if (e.keyCode === 229) return
  e.preventDefault()
  void send()
}
