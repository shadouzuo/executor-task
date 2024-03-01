
DEFAULT: build-cur

ifeq ($(GOPATH),)
  GOPATH = $(HOME)/go
endif

build-cur:
	GOPATH=$(GOPATH) go install github.com/pefish/go-build-tool/cmd/...@latest
	$(GOPATH)/bin/go-build-tool

install: build-cur
	sudo install -C ./build/bin/linux/executor-task /usr/local/bin/executor-task

install-service: install
	sudo mkdir -p /etc/systemd/system
	sudo install -C -m 0644 ./script/executor-task.service /etc/systemd/system/executor-task.service
	sudo systemctl daemon-reload
	@echo
	@echo "executor-task service installed."

