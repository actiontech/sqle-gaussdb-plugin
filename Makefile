# 当前 HEAD 的 tag
HEAD_TAG = $(shell git describe --exact-match --tags 2>/dev/null)

# 当前分支名
HEAD_BRANCH = $(shell git rev-parse --abbrev-ref HEAD)

# 当前 commit hash
HEAD_HASH = $(shell git rev-parse HEAD)

# 1. 如果HEAD存在tag，则GIT_VERSION=<版本名称>-<企业版/社区版> <commit>
# PS: 通常会在版本名称前增加字符“v”作为tag内容，当版本名称为 3.2411.0时，tag内容为v3.2411.0 
# e.g. tag为v3.2411.0时，GIT_VERSION=3.2411.0 a6355ff4cf8d181315a2b30341bc954b29576b11
# 2. 如果HEAD没有tag，则GIT_VERSION=<分支名> <commit>
# e.g. 分支名为main时，GIT_VERSION=main a6355ff4cf8d181315a2b30341bc954b29576b11
# e.g. 分支名为release-3.2411.x时，GIT_VERSION=release-3.2411.x a6355ff4cf8d181315a2b30341bc954b29576b11
override GIT_VERSION = $(if $(HEAD_TAG),$(shell echo $(HEAD_TAG) | sed 's/^v//'),$(HEAD_BRANCH))${CUSTOM} $(HEAD_HASH)
override DOCKER         		= $(shell which docker)
override PROJECT_NAME 			= sqle-gaussdb-plugin
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

install:
	$(ARM_CGO_BUILD_FLAG) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -mod=vendor ${LDFLAGS} -tags $(GO_BUILD_TAGS) -o $(GOBIN)/$(PROJECT_NAME)$(BUILD_TARGET) ./


# todo 升级golang版本后，git获取版本号失败，临时添加"git config --global --add safe.directory /universe"解决
docker_install:
	$(DOCKER) run -v $(shell pwd):/universe --rm $(GO_COMPILER_IMAGE) sh -c "git config --global --add safe.directory /universe && cd /universe && make install $(MAKEFLAGS)"


upload:
	curl -T $(GOBIN)/$(PROJECT_NAME)$(BUILD_TARGET) ftp://$(RELEASE_FTPD_HOST)/actiontech-sqle/plugins/$(PROJECT_VERSION)/$(PROJECT_NAME)$(BUILD_TARGET) --ftp-create-dirs

.PHONY: vendor
vendor:
	go mod vendor
	modvendor -copy="**/*.c **/*.h" -v	