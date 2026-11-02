SHELL := /bin/sh

GO ?= go
APP_NAME ?= peck
BUILD_DIR ?= build
CGO_ENABLED ?= 1
ARGS ?=

.DEFAULT_GOAL := help

.PHONY: help deps version doctor run fmt test vet check build package package-dmg render-projects clean

help: ## 显示可用的快捷指令
	@awk 'BEGIN {FS = ":.*##"; printf "用法: make <目标>\n\n目标:\n"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-14s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

deps: ## 下载 Go 模块依赖
	$(GO) mod download

version: ## 显示 Go 和 Viva 版本
	@$(GO) version
	@$(GO) tool viva version

doctor: ## 检查 Viva 构建环境
	$(GO) tool viva doctor

run: ## 直接运行应用
	CGO_ENABLED=$(CGO_ENABLED) $(GO) run . $(ARGS)


fmt: ## 格式化 Go 源码
	$(GO) fmt ./...

test: ## 运行全部测试
	$(GO) test ./...

vet: ## 执行 go vet 静态检查
	$(GO) vet ./...

check: test vet ## 运行测试和静态检查

build: ## 构建本地可执行文件
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build -o $(BUILD_DIR)/$(APP_NAME) .

package: ## 构建未签名的 macOS App 和 ZIP
	CGO_ENABLED=$(CGO_ENABLED) $(GO) tool viva package --format app,zip --no-sign $(ARGS)

package-dmg: ## 构建未签名的 macOS App、ZIP 和 DMG
	CGO_ENABLED=$(CGO_ENABLED) $(GO) tool viva package --format app,zip,dmg --no-sign $(ARGS)

render-projects: ## 校验项目清单并更新 README 表格
	node .github/scripts/render-project-links.mjs --write

clean: ## 删除本地构建产物
	rm -rf $(BUILD_DIR)
