
clean:
	go clean -i ./...

deps:
	go get -d -v ./...

test:
	go test -cover ./...

test-race:
	go test -race ./...

protoc:
	protoc rpc.proto -I ./ -I ../../../ --go_out=plugins=grpc:.
