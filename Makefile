NAME := deployment-sdk

.PHONY: test

test:
	go fmt ./...
	gotestsum ./...

update-aws:
	go get -u $$(go list -mod=mod -m all | grep '^github.com/aws/aws-sdk-go-v2' | awk '{print $$1}') && go mod tidy

update-gcp:
	go get -u $$(go list -mod=mod -m all | grep '^cloud.google.com/go' | awk '{print $$1}') && go mod tidy
