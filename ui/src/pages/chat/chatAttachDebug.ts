/** 附件/待发送预览调试：开发环境默认输出；正式环境可执行 `localStorage.setItem('chatDebug','1')` 后刷新 */
export function chatAttachDebugEnabled (): boolean {
  if (import.meta.env.DEV) return true
  try {
    return typeof localStorage !== 'undefined' && localStorage.getItem('chatDebug') === '1'
  } catch {
    return false
  }
}

export function logChatAttach (...args: unknown[]): void {
  if (!chatAttachDebugEnabled()) return
  // eslint-disable-next-line no-console -- 受 chatAttachDebugEnabled 与 DEV/localStorage 控制
  console.log('[chat-attach]', ...args)
}
