
clean:
	go clean -i ./...

test-race:
	go test -race .

protoc:
	protoc rpc.proto -I ./ -I ../../../ --go_out=plugins=grpc:.
