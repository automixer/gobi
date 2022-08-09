APP_NAME				:= gobi
APP_PATH				:= cmd/gobi/*.go

SHELL					:= /bin/bash
BUILD_DIR				:= build/
BUILD_DATE				?= $(shell date +%FT%T%z)
GOOS					?= $(shell go env GOOS)
GOARCH					?= $(shell go env GOARCH)
APP_VERSION				?= $(shell git describe --abbrev --long --tags HEAD)
LDFLAGS					?= '-X main.appName=$(APP_NAME) -X main.appVersion=$(APP_VERSION) -X main.buildDate=$(BUILD_DATE)'
BIN_NAME				:= $(BUILD_DIR)$(APP_NAME)-$(GOOS)-$(GOARCH)
BIN_NAME_RELEASE		:= $(BUILD_DIR)$(APP_NAME)
.DEFAULT_GOAL			:= build

fmt:
	go fmt ./...
.PHONY: fmt

vet: fmt
	go vet ./...
.PHONY: vet

prepare:
	mkdir -p $(BUILD_DIR)
.PHONY: prepare

clean:
	rm -rf $(BUILD_DIR)
.PHONY: clean

build: prepare vet
	go build -ldflags $(LDFLAGS) -o $(BIN_NAME) $(APP_PATH)
.PHONY: build

release: prepare vet
	go build -ldflags $(LDFLAGS) -o $(BIN_NAME_RELEASE) $(APP_PATH)
.PHONY: release
