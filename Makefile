.PHONY: dev-up dev-down dev-status build build-backend build-frontend build-datamanagementd test test-backend test-frontend test-frontend-critical test-datamanagementd secret-scan upstream-sync-preflight upstream-sync-check

FRONTEND_CRITICAL_VITEST := \
	src/i18n/__tests__/localeKeyCompleteness.spec.ts \
	src/api/__tests__/client.spec.ts \
	src/api/__tests__/tokenRefresh.spec.ts \
	src/api/__tests__/channelMonitorV2.spec.ts \
	src/views/auth/__tests__/LinuxDoCallbackView.spec.ts \
	src/views/auth/__tests__/WechatCallbackView.spec.ts \
	src/views/user/__tests__/PaymentView.spec.ts \
	src/views/user/__tests__/PaymentResultView.spec.ts \
	src/views/user/__tests__/ChannelStatusView.mode.spec.ts \
	src/components/user/profile/__tests__/ProfileInfoCard.spec.ts \
	src/views/admin/__tests__/SettingsView.spec.ts \
	src/features/channel-monitor-v2/__tests__/designSystem.structure.spec.ts \
	src/features/channel-monitor-v2/__tests__/monitorFormat.spec.ts \
	src/features/channel-monitor-v2/__tests__/monitorZoom.spec.ts

# 一键启动本地 Docker 开发环境
dev-up:
	@./deploy/start-local.sh

dev-down:
	@./deploy/start-local.sh --stop

dev-status:
	@./deploy/start-local.sh --status

# 一键编译前后端：先生成前端产物，再把最新 dist 嵌入后端二进制
build: build-frontend build-backend

# 编译后端（复用 backend/Makefile，嵌入最新前端产物）
build-backend:
	@$(MAKE) -C backend build-embed

# 编译前端（需要已安装依赖）
build-frontend:
	@pnpm --dir frontend run build

# 编译 datamanagementd（宿主机数据管理进程）
build-datamanagementd:
	@cd datamanagement && go build -o datamanagementd ./cmd/datamanagementd

# 运行测试（后端 + 前端）
test: test-backend test-frontend

test-backend:
	@$(MAKE) -C backend test

test-frontend:
	@pnpm --dir frontend run lint:check
	@pnpm --dir frontend run typecheck
	@$(MAKE) test-frontend-critical

test-frontend-critical:
	@pnpm --dir frontend exec vitest run $(FRONTEND_CRITICAL_VITEST)

test-datamanagementd:
	@cd datamanagement && go test ./...

secret-scan:
	@python3 tools/secret_scan.py

upstream-sync-preflight:
	@python3 tools/upstream_sync.py preflight $(UPSTREAM_SYNC_ARGS)

upstream-sync-check:
	@python3 tools/upstream_sync.py check $(UPSTREAM_SYNC_ARGS)
