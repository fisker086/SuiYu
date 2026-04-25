# SuiYu Agent

[English](README.md)

---

## 中文

### 简介

SuiYu Agent 是一个开源的 AI Agent 平台，支持 Agent 管理、技能系统、MCP 协议、聊天对话与 RBAC 权限控制。

- **后端**：Go (Hertz) + PostgreSQL (pgvector)
- **前端**：可选嵌入的 Vue3 Web UI

### 功能特性

| 功能 | 说明 |
|------|------|
| Agent 管理 | 创建、配置、版本控制 |
| 技能系统 | Skill 封装与动态加载 |
| MCP 协议 | Model Context Protocol 支持 |
| 聊天对话 | 多会话、上下文记忆 |
| RBAC | 用户、角色、权限控制 |
| 长期记忆 | pgvector 向量存储 |
| 沙箱执行 | Docker 沙箱代码执行（需要挂载 /var/run/docker.sock） |

### 环境要求

| 组件 | 最低版本 |
|------|----------|
| Docker | 20.10+ |
| Docker Compose | 2.0+ |
| PostgreSQL | 15+ (带 pgvector) |
| Node.js | 22+ (前端开发) |
| Go | 1.26+ (后端开发) |

### 快速开始

#### Docker 部署

默认包含 PostgreSQL（pgvector），直接启动即可。

```bash
cp .env.example .env
# 编辑 .env，必填：JWT_SECRET_KEY、OPENAI_API_KEY、ADMIN_DEFAULT_PASSWORD
docker compose up -d
```

若改用外部 PostgreSQL，在 `.env` 中设置 `DATABASE_URL`（默认仍会启动 compose 内的 `postgres` 服务，高级场景可自行用 override 禁用）。

**访问：**
- Web: http://localhost:8080
- API: http://localhost:8080/api/v1
- Swagger: http://localhost:8080/swagger

#### 3. 本地开发

```bash
# 后端
cp .env.example .env
go run ./cmd/server

# 前端（另开终端）
cd ui && npm install && npm run dev
```

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `SERVER_PORT` | `8080` | HTTP 端口 |
| `JWT_SECRET_KEY` | - | **必填** JWT 密钥 |
| `MODEL_TYPE` | `openai` | `openai` / `ark` |
| `OPENAI_API_KEY` | - | OpenAI 密钥 |
| `DATABASE_URL` | - | PostgreSQL 连接串 |
| `MEMORY_PROVIDER` | `pgvector` | 长期记忆：`pgvector` / `none` |

完整变量列表见 `.env.example`。

### 技术栈

- **后端**: Go, Hertz, GORM, pgvector
- **前端**: Vue 3,  TypeScript
- **数据库**: PostgreSQL 15+, pgvector
- **LLM**: OpenAI 兼容 API, 火山方舟

### 许可证

Apache License 2.0

