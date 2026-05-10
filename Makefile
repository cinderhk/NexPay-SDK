BINARY := nexpay
MODULE := github.com/nexpay/nexpay-sdk
ENTRY  := ./cmd/server
CONFIG := configs/config.yaml

UPX        ?= upx
COMPRESS   ?= 1
BUILD_DIR  := bin
BUILD_TAGS :=
BUILD_FLAGS := -v

BUILD_COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
BUILD_TIME    := $(shell date "+%Y-%m-%d %H:%M:%S")

CGO_ENABLED := 0
GO111MODULE := on

HOST_OS := $(shell uname -s | tr A-Z a-z)

LDFLAGS := -w -s -buildid=
LDFLAGS += -X "$(MODULE)/internal/version.Version=$(BUILD_VERSION)"
LDFLAGS += -X "$(MODULE)/internal/version.Commit=$(BUILD_COMMIT)"
LDFLAGS += -X "$(MODULE)/internal/version.BuildTime=$(BUILD_TIME)"

GO_BUILD = GO111MODULE=$(GO111MODULE) CGO_ENABLED=$(CGO_ENABLED) \
	go build $(BUILD_FLAGS) -trimpath -ldflags '$(LDFLAGS)' -tags '$(BUILD_TAGS)'

define compress
	@target="$(1)"; \
	case "$$target" in \
		*darwin*) echo ">> skip upx: darwin target not supported ($$target)"; exit 0 ;; \
	esac; \
	if [ "$$target" = "$(BUILD_DIR)/$(BINARY)" ] && [ "$(HOST_OS)" = "darwin" ]; then \
		echo ">> skip upx: host is darwin ($$target)"; \
		exit 0; \
	fi; \
	if [ "$(COMPRESS)" = "1" ] && command -v $(UPX) >/dev/null 2>&1; then \
		echo ">> compressing $$target"; \
		$(UPX) -9 $$target; \
	else \
		echo ">> skip upx (not installed or disabled)"; \
	fi
endef

.PHONY: all build run start migrate clean linux darwin windows tidy fmt vet test lint debug

all: build

build:
	@echo ">> building $(BINARY) version=$(BUILD_VERSION) commit=$(BUILD_COMMIT)"
	@mkdir -p $(BUILD_DIR)
	$(GO_BUILD) -o $(BUILD_DIR)/$(BINARY) $(ENTRY)
	$(call compress,$(BUILD_DIR)/$(BINARY))

# 直接使用 go run 运行（开发态）
run:
	go run $(ENTRY) --config $(CONFIG)

# 编译产物启动（生产态）
start: build
	./$(BUILD_DIR)/$(BINARY) --config $(CONFIG)

# 执行 GORM AutoMigrate
migrate: build
	./$(BUILD_DIR)/$(BINARY) migrate --config $(CONFIG)

clean:
	rm -rf $(BUILD_DIR)

tidy:
	go mod tidy

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./... -race -count=1

lint: vet

# ===== 跨平台构建 =====

linux:
	@echo ">> build linux amd64"
	GOOS=linux GOARCH=amd64 $(GO_BUILD) -o $(BUILD_DIR)/$(BINARY)-linux-amd64 $(ENTRY)
	$(call compress,$(BUILD_DIR)/$(BINARY)-linux-amd64)

darwin:
	@echo ">> build mac"
	GOOS=darwin GOARCH=amd64 $(GO_BUILD) -o $(BUILD_DIR)/$(BINARY)-darwin-amd64 $(ENTRY)
	$(call compress,$(BUILD_DIR)/$(BINARY)-darwin-amd64)

windows:
	@echo ">> build windows"
	GOOS=windows GOARCH=amd64 $(GO_BUILD) -o $(BUILD_DIR)/$(BINARY)-windows-amd64.exe $(ENTRY)
	$(call compress,$(BUILD_DIR)/$(BINARY)-windows-amd64.exe)

# ===== debug 构建（保留 build tag 钩子，目前代码未使用） =====
debug: BUILD_TAGS += debug
debug: build
