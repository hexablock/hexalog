
clean:
	go clean -i ./...


deps:
	go get github.com/golang/dep/cmd/dep
	dep ensure

test:
	go test -cover $(shell go list ./... | grep -v /vendor/)

test-race:
	go test -race ./...

protoc:
	protoc rpc.proto -I ./ -I ../../../ --go_out=plugins=grpc:.
