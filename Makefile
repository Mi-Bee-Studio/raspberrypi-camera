# rpi3b-cam Makefile
# Cross-compile for RPi 3B (aarch64) from workstation

GOOS ?= linux
GOARCH ?= arm64
BINARY := build/rpi-cam
REMOTE_HOST ?= pi@192.168.1.100
REMOTE_DIR ?= ~/rpi-cam

.PHONY: build test deploy clean service-restart service-stop service-logs service-status mediamtx-disable

build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(BINARY) ./cmd/server

test:
	go test ./...

deploy: build
	scp $(BINARY) configs/config.yaml $(REMOTE_HOST):$(REMOTE_DIR)/
	scp deploy/rpi-cam.service $(REMOTE_HOST):/tmp/
	ssh $(REMOTE_HOST) 'sudo mv /tmp/rpi-cam.service /etc/systemd/system/ && sudo systemctl daemon-reload && sudo systemctl enable rpi-cam'

service-restart:
	ssh $(REMOTE_HOST) 'sudo systemctl restart rpi-cam'

service-stop:
	ssh $(REMOTE_HOST) 'sudo systemctl stop rpi-cam'

service-logs:
	ssh $(REMOTE_HOST) 'journalctl -u rpi-cam -f'

service-status:
	ssh $(REMOTE_HOST) 'systemctl status rpi-cam'

mediamtx-disable:
	ssh $(REMOTE_HOST) 'sudo systemctl stop mediamtx && sudo systemctl disable mediamtx'

clean:
	rm -rf build/
