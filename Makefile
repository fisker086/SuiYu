.PHONY: help dev build build-ui build-desktop run clean test lint swagger docker-build docker-up docker-down

help:
	@echo "SuiYu Agent Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make dev              - 启动后端开发服务器"
	@echo "  make build            - 编译后端"
	@echo "  make build-ui         - 构建前端"
	@echo "  make run              - 运行编译后的后端"
	@echo "  make test             - 运行测试"
	@echo "  make lint             - 代码检查"
	@echo "  make clean            - 清理构建产物"
	@echo "  make docker-build     - 构建 Docker 镜像"
	@echo "  make swagger        - 生成 Swagger 文档"
	@echo "  make docker-up        - 启动 Docker 服务"
	@echo "  make docker-down      - 停止 Docker 服务"

dev:
	@echo "启动后端开发服务器..."
	@cp -n .env.example .env 2>/dev/null || true
	go run ./cmd/server

build: build-ui
	@echo "编译后端..."
	@mkdir -p build
	go build -o build/suiyu-agent ./cmd/server

build-ui:
	@echo "构建前端..."
	cd ui && npm run build

run:
	@echo "运行后端..."
	./build/suiyu-agent

test:
	@echo "运行测试..."
	go test ./...

lint:
	@echo "代码检查..."
	go vet ./...

swagger:
	@echo "生成 Swagger 文档..."
	@bash scripts/regen-swagger.sh

clean:
	@echo "清理构建产物..."
	rm -rf build/
	rm -rf ui/dist/
	rm -f suiyu-agent

docker-build:
	@echo "构建 Docker 镜像..."
	docker build -t suiyu-agent:latest .

docker-up:
	@echo "启动 Docker 服务..."
	docker compose up -d

docker-down:
	@echo "停止 Docker 服务..."
	docker compose down
