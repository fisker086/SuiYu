---
name: browser-desktop-visible-chrome
description: On AI TaskMeta desktop, drive a visible Chrome/Chromium via CDP (navigate, click, type, screenshot, page text for the model)
activation_keywords: [browser, web, url, fetch, page, desktop, client, builtin_browser, chrome, cdp]
execution_mode: client
---

# `builtin_browser`（桌面端可见 Chrome）

在 **AI TaskMeta 桌面端**（Tauri）执行：启动或复用 **有窗口的 Chrome/Chromium**，通过 **Chrome DevTools Protocol** 在同一标签页中导航、点击、输入、截图，并把 **页面文本/HTML 摘要** 作为工具结果返回给模型。**不是**无头模式；**不是**服务端 `curl`/HTTP 客户端在后台拉网页。

- 与用户日常自己打开的 Chrome **用户数据隔离**（默认临时 profile），登录态不共享。
- 若用户 **手动关掉** 自动化窗口，下一次调用工具会 **重新启动** 会话（应用侧会检测进程）。
- 需要 **仅 HTTP 拉取、无 DOM 操作** 时，服务端用 **`builtin_http_client`**；桌面端也可用其作补充。

## URL 限制（导航类 `operation`）

- 允许：`https://...`
- 允许：`http://127.0.0.1` / `http://localhost` / `http://[::1]`
- 其它 `http://` 会被拒绝。

## 常用 `operation`

| operation | 说明 |
|-----------|------|
| `open` / `goto` / `visit` / `fetch` / `get` / `content` / `navigate` | 必填 `url`（或 `target`），在当前页跳转并返回抽取的正文等 |
| `text` | 当前页或指定 `url` 后取文本；可配合 `selector` |
| `html` | 原始 HTML（有截断） |
| `click` / `dblclick` | `selector` |
| `type` / `input` / `fill` | `selector` + `text` |
| `press` | 按键 |
| `screenshot` | 可选 `path` 保存文件 |
| `scroll` / `wait` / `eval` / `reload` / `back` | 见实现 |
| `close` / `quit` | 结束自动化会话 |

可选环境变量：`AITASKMETA_CHROME_USER_DATA_DIR`（旧名 `AGENTSPHERE_CHROME_USER_DATA_DIR` 仍兼容）指向固定用户数据目录（需避免与正在运行的 Chrome 抢锁）。

## 示例

```json
{
  "operation": "goto",
  "url": "https://example.com"
}
```

## 安全

仅白名单 URL；返回内容有长度上限。勿向用户谎称「无头」或「无法在屏幕上显示」——本工具即为 **可见窗口**。
