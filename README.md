# SuiYu Agent

[中文](README-zh.md)

---

## English

### Introduction

SuiYu Agent is an open-source AI Agent platform with Agent management, Skill system, MCP protocol, chat functionality, and RBAC permission control.

- **Backend**: Go (Hertz) + PostgreSQL (pgvector)
- **Frontend**: Optional embedded Vue3 Web UI

### Features

| Feature | Description |
|---------|-------------|
| Agent Management | Create, configure, version control |
| Skill System | Skill encapsulation and dynamic loading |
| MCP Protocol | Model Context Protocol support |
| Chat | Multiple sessions, context memory |
| RBAC | User, role, permission control |
| Long-term Memory | pgvector storage |
| Sandbox Execution | Docker sandbox for code execution (需要挂载 /var/run/docker.sock) |

### Requirements

| Component | Minimum Version |
|-----------|-----------------|
| Docker | 20.10+ |
| Docker Compose | 2.0+ |
| PostgreSQL | 15+ (with pgvector) |
| Node.js | 22+ (frontend dev) |
| Go | 1.26+ (backend dev) |

### Quick Start

#### Docker Deployment

Compose includes PostgreSQL (pgvector) by default.

```bash
cp .env.example .env
# Edit .env, required: JWT_SECRET_KEY, OPENAI_API_KEY, ADMIN_DEFAULT_PASSWORD
docker compose up -d
```

To use an external PostgreSQL instead, set `DATABASE_URL` in `.env` (the stack still starts the bundled `postgres` service unless you adjust compose with an override).

**Access:**
- Web: http://localhost:8080
- API: http://localhost:8080/api/v1
- Swagger: http://localhost:8080/swagger

#### 3. Local Development

```bash
# Backend
cp .env.example .env
go run ./cmd/server

# Frontend (separate terminal)
cd ui && npm install && npm run dev
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_PORT` | `8080` | HTTP port |
| `JWT_SECRET_KEY` | - | **Required** JWT secret |
| `MODEL_TYPE` | `openai` | `openai` / `ark` |
| `OPENAI_API_KEY` | - | OpenAI API key |
| `DATABASE_URL` | - | PostgreSQL connection string |
| `MEMORY_PROVIDER` | `pgvector` | Memory: `pgvector` / `none` |

See `.env.example` for full variable list.

### Tech Stack

- **Backend**: Go, Hertz, GORM, pgvector
- **Frontend**: Vue 3, TypeScript
- **Desktop**: Tauri, React
- **Database**: PostgreSQL 15+, pgvector
- **LLM**: OpenAI compatible API, Volcengine Ark

### License

Apache License 2.0

