HELM_HOME ?= $(shell helm home)
BINARY_NAME ?= helm-x
PLUGIN_NAME ?= helm-x
HELM_PLUGIN_DIR ?= $(HELM_HOME)/plugins/helm-x
VERSION := $(shell sed -n -e 's/version:[ "]*\([^"]*\).*/\1/p' plugin.yaml)
DIST := $(CURDIR)/_dist
LDFLAGS := "-X main.version=${VERSION}"

.PHONY: install
install: build
	mkdir -p $(HELM_PLUGIN_DIR)/bin
	cp $(BINARY_NAME) $(HELM_PLUGIN_DIR)/bin/
	cp plugin.yaml $(HELM_PLUGIN_DIR)/

.PHONY: uninstall
uninstall:
	echo "would you mind removing $(HELM_PLUGIN_DIR)/ by yourself? :)"

.PHONY: hookInstall
hookInstall: build

.PHONY: build
build:
	go build -o $(BINARY_NAME) -ldflags $(LDFLAGS) ./main.go

.PHONY: dist
dist:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME) -ldflags $(LDFLAGS) ./main.go
	tar -zcvf $(DIST)/$(PLUGIN_NAME)-linux-$(VERSION).tgz $(BINARY_NAME) README.md LICENSE.txt plugin.yaml
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o $(BINARY_NAME) -ldflags $(LDFLAGS) ./main.go
	tar -zcvf $(DIST)/$(PLUGIN_NAME)-macos-$(VERSION).tgz $(BINARY_NAME) README.md LICENSE.txt plugin.yaml
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o $(BINARY_NAME).exe -ldflags $(LDFLAGS) ./main.go
	tar -zcvf $(DIST)/$(PLUGIN_NAME)-windows-$(VERSION).tgz $(BINARY_NAME).exe README.md LICENSE.txt plugin.yaml
	rm inj
	rm inj.exe
