NAME := deployment-sdk

.PHONY: test

test:
	go fmt ./...
	gotestsum ./...
