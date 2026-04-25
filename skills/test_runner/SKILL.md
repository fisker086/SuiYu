---
name: test-runner
description: Run unit tests and test suites for various frameworks locally on desktop
activation_keywords: [test, run, unit, suite, jest, pytest, go test, cargo test]
execution_mode: client
---

# Test Runner Skill

在 **AI TaskMeta 桌面端** 本地运行测试命令。

## 支持的框架

| 框架 | 命令 |
|------|------|
| Go | `go test -v` |
| JavaScript/TypeScript | `npm test` / `npx jest` |
| Python | `pytest -v` |
| Rust | `cargo test` |

## 参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `framework` | string | 测试框架：go, jest, pytest, cargo（默认 go） |
| `path` | string | 测试文件或目录路径 |
| `pattern` | string | 测试名称匹配模式 |
| `work_dir` | string | 工作目录（默认当前目录） |

## 使用示例

```json
{
  "framework": "go",
  "path": "./tests",
  "pattern": "TestMain",
  "work_dir": "/path/to/project"
}
```

```json
{
  "framework": "jest",
  "path": "test.spec.ts"
}
```

## 结合 E2E Recorder

E2E Recorder 生成的 Playwright 测试可以用此工具运行：

```json
{
  "framework": "jest",
  "path": "/tmp/e2e-test/test.spec.ts",
  "work_dir": "/tmp/e2e-test"
}
```

或者直接在命令行：
```bash
cd /tmp/e2e-test && npx playwright test