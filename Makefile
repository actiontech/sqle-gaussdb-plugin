# sqle-gaussdb-plugin Makefile（方向 A 双独立二进制方案）
#
# 与 sqle-tbase-plugin/Makefile 同源；改造点：
#   - 单进程单 PluginName 编译目标拆分为 build-gaussdb + build-opengauss
#   - 主 install / docker_install target 同时产出 2 个二进制：
#       bin/sqle-gaussdb-plugin   （PluginName=GaussDB,   默认端口 8000）
#       bin/sqle-opengauss-plugin （PluginName=openGauss, 默认端口 5432）
#   - 构建镜像保持 golang:1.19.6，与 sqle-pg-plugin / sqle-tbase-plugin 一致

# 当前 HEAD 的 tag
HEAD_TAG = $(shell git describe --exact-match --tags 2>/dev/null)

# 当前分支名
HEAD_BRANCH = $(shell git rev-parse --abbrev-ref HEAD)

# 当前 commit hash
HEAD_HASH = $(shell git rev-parse HEAD)

# 1. 如果 HEAD 存在 tag，则 GIT_VERSION=<版本名称>-<企业版/社区版> <commit>
# 2. 如果 HEAD 没有 tag，则 GIT_VERSION=<分支名> <commit>
override GIT_VERSION = $(if $(HEAD_TAG),$(shell echo $(HEAD_TAG) | sed 's/^v//'),$(HEAD_BRANCH))${CUSTOM} $(HEAD_HASH)
override DOCKER         		= $(shell which docker)
override LDFLAGS 				= -ldflags "-X 'main.version=\"${GIT_VERSION}\"'"
override GOBIN					= ${shell pwd}/bin
override GOOS           		= linux

GOARCH         		= amd64
BUILD_TARGET		=
GO_COMPILER_IMAGE ?= golang:1.19.6

EDITION ?= ee
GO_BUILD_TAGS = dummyhead,enterprise
ifeq ($(EDITION),trial)
    GO_BUILD_TAGS :=$(GO_BUILD_TAGS),plugin_trial
endif

## Arm Build Flag
ARM_CGO_BUILD_FLAG =
ifeq ($(EDITION)_$(GOARCH),ee_arm64)
    ARM_CGO_BUILD_FLAG = CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc
endif

ifeq ($(GOARCH), arm64)
    BUILD_TARGET = -aarch64
endif

# Copy from SQLE
PROJECT_VERSION = $(shell if [ "$$(git tag --points-at HEAD | tail -n1)" ]; then git tag --points-at HEAD | tail -n1 | sed 's/v\(.*\)/\1/'; else git rev-parse --abbrev-ref HEAD | sed 's/release-\(.*\)/\1/' | tr '-' '\n' | head -n1; fi)

default: install

# install 同时编译两个二进制，实现方向 A 的"双独立二进制注册" 部署形态
install: build-gaussdb build-opengauss

build-gaussdb:
	$(ARM_CGO_BUILD_FLAG) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -mod=mod ${LDFLAGS} -tags $(GO_BUILD_TAGS) -o $(GOBIN)/sqle-gaussdb-plugin$(BUILD_TARGET) ./cmd/sqle-gaussdb-plugin

build-opengauss:
	$(ARM_CGO_BUILD_FLAG) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -mod=mod ${LDFLAGS} -tags $(GO_BUILD_TAGS) -o $(GOBIN)/sqle-opengauss-plugin$(BUILD_TARGET) ./cmd/sqle-opengauss-plugin


# todo 升级 golang 版本后，git 获取版本号失败，临时添加 "git config --global --add safe.directory /universe" 解决
docker_install:
	$(DOCKER) run -v $(shell pwd):/universe --rm $(GO_COMPILER_IMAGE) sh -c "git config --global --add safe.directory /universe && cd /universe && make install $(MAKEFLAGS)"


upload:
	curl -T $(GOBIN)/sqle-gaussdb-plugin$(BUILD_TARGET) ftp://$(RELEASE_FTPD_HOST)/actiontech-sqle/plugins/$(PROJECT_VERSION)/sqle-gaussdb-plugin$(BUILD_TARGET) --ftp-create-dirs
	curl -T $(GOBIN)/sqle-opengauss-plugin$(BUILD_TARGET) ftp://$(RELEASE_FTPD_HOST)/actiontech-sqle/plugins/$(PROJECT_VERSION)/sqle-opengauss-plugin$(BUILD_TARGET) --ftp-create-dirs

.PHONY: vendor
vendor:
	go mod vendor
	modvendor -copy="**/*.c **/*.h" -v

.PHONY: install build-gaussdb build-opengauss docker_install upload
