HELM_HOME ?= $(shell helm home)
BINARY_NAME ?= helm-x
PLUGIN_NAME ?= helm-x
HELM_PLUGIN_DIR ?= $(HELM_HOME)/plugins/helm-x
VERSION := $(shell sed -n -e 's/version:[ "]*\([^"]*\).*/\1/p' plugin.yaml)
DIST := $(CURDIR)/_dist
LDFLAGS := "-X main.Version=${VERSION}"

.PHONY: helm
helm:
	curl -LO https://git.io/get_helm.sh
	chmod 700 get_helm.sh
	./get_helm.sh

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

.PHONY: format
format:
	test -z "$$(find . -path ./vendor -prune -type f -o -name '*.go' -exec gofmt -d {} + | tee /dev/stderr)" || \
	test -z "$$(find . -path ./vendor -prune -type f -o -name '*.go' -exec gofmt -w {} + | tee /dev/stderr)"

.PHONY: test
test: build
	go test ./...

.PHONY: build
build:
	go build -o $(BINARY_NAME) -ldflags $(LDFLAGS) ./

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

release/minor:
	git fetch origin master
	bash -c 'if git branch | grep autorelease; then git branch -D autorelease; else echo no branch to be cleaned; fi'
	git checkout -b autorelease origin/master
	git branch -D master || echo "no master branch found. skipping deletion"
	git branch -m autorelease master
	hack/semtag final -s minor

release/patch:
	git fetch origin master
	bash -c 'if git branch | grep autorelease; then git branch -D autorelease; else echo no branch to be cleaned; fi'
	git checkout -b autorelease origin/master
	git branch -D master || echo "no master branch found. skipping deletion"
	git branch -m autorelease master
	hack/semtag final -s patch
