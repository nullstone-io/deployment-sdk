NAME := deployment-sdk

.PHONY: test

test:
	go fmt ./...
	gotestsum ./...

update-platforms: update-aws update-gcp update-azure update-k8s update-docker tidy

update-aws:
	go get -u $$(go list -mod=mod -m all | grep '^github.com/aws/aws-sdk-go-v2' | awk '{print $$1}')

update-gcp:
	go get -u $$(go list -mod=mod -m all | grep '^cloud.google.com/go' | awk '{print $$1}')

update-azure:
	go get -u $$(go list -mod=mod -m all | grep '^github.com/Azure/azure-sdk-for-go' | awk '{print $$1}')

update-k8s:
	go get -u $$(go list -mod=mod -m all | grep '^k8s.io' | awk '{print $$1}')

update-docker:
	go get -u github.com/moby/moby/client github.com/docker/docker github.com/docker/cli

tidy:
	go mod tidy
