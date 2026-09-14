# ============================================================================
# Fluxor 构建 Makefile
# ----------------------------------------------------------------------------
# 目录结构:
#   frontend/  Vue 3 + Vite + TypeScript 前端源码
#   backend/   Go 后端源码（内嵌 frontend/dist 构建产物）
#
# 常用命令:
#   make           构建前端 + 同步 dist 到 backend + 编译后端二进制
#   make frontend  仅构建前端 (frontend/dist)
#   make backend   同步 dist 并编译后端二进制 (./fluxor)
#   make sync      仅将 frontend/dist 同步到 backend/dist
#   make run       以开发方式运行后端（依赖 ./backend/dist 已存在）
#   make clean     清理构建产物
#
# 环境变量:
#   GOOS / GOARCH  Go 交叉编译目标（默认跟随本机）
#   GOFLAGS        Go 构建附加参数（如 -ldflags）
#   BIN            输出二进制名称与路径（默认 ./fluxor）
# ============================================================================

# 目录与路径
FRONTEND_DIR ?= frontend
BACKEND_DIR  ?= backend
FRONTEND_DIST = $(FRONTEND_DIR)/dist
BACKEND_DIST  = $(BACKEND_DIR)/dist
BIN          ?= fluxor
GO           ?= go
GOFLAGS      ?= -ldflags="-s -w"

# 默认目标：完整构建
.PHONY: all frontend sync backend run clean
all: backend

# 仅构建前端
frontend:
	cd $(FRONTEND_DIR) && npm install
	cd $(FRONTEND_DIR) && npm run build

# 将前端构建产物同步到 backend/dist 供 go:embed 内嵌
sync: $(FRONTEND_DIST)
	rm -rf $(BACKEND_DIST)
	mkdir -p $(BACKEND_DIR)
	cp -r $(FRONTEND_DIST) $(BACKEND_DIST)

# 完整编译后端二进制（依赖 dist 已同步）
backend: sync
	cd $(BACKEND_DIR) && CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build $(GOFLAGS) -o $(CURDIR)/$(BIN)

# 开发运行后端（需先执行 make sync）
run:
	cd $(BACKEND_DIR) && $(GO) run .

# 清理构建产物
clean:
	rm -rf $(FRONTEND_DIST)
	rm -rf $(BACKEND_DIST)
	rm -f $(CURDIR)/$(BIN)