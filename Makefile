uname_p := $(shell uname -p) # store the output of the command in a variable

build: pre_build build_kafka build_rabbitmq

pre_build:
	./set_version.sh
	go mod tidy
	mkdir -p ./bin

build_kafka:
	go build -o ./bin/pgarrowkafka.$(uname_p) ./cmd/pgarrowkafka
	go build -o ./bin/kafkaarrowpg.$(uname_p) ./cmd/kafkaarrowpg

build_rabbitmq:
	go build -o ./bin/pgarrowrabbit.$(uname_p) ./cmd/pgarrowrabbit
	go build -o ./bin/rabbitarrowpg.$(uname_p) ./cmd/rabbitarrowpg

build_dlv:
	go get github.com/go-delve/delve/cmd/dlv@latest
	go build -o /bin/dlv.$(uname_p) github.com/go-delve/delve/cmd/dlv

# Use the following on m1:
# alias make='/usr/bin/arch -arch arm64 /usr/bin/make'
debug_pgarrow:
	go build -gcflags "all=-N -l" -o ./bin/pgarrowkafka.debug.$(uname_p) ./cmd/pgarrowkafka
	~/go/bin/dlv --headless --listen=:2345 --api-version=2 --accept-multiclient exec ./bin/pgarrowkafka.debug.$(uname_p)

debug_arrowpg:
	go build -gcflags "all=-N -l" -o ./bin/arrowpg.debug.$(uname_p) ./cmd/arrowpg
	~/go/bin/dlv --headless --listen=:2345 --api-version=2 --accept-multiclient exec ./bin/arrowpg.debug.$(uname_p)

#debug_test:
#	~/go/bin/dlv --headless --listen=:2345 --api-version=2 --accept-multiclient test ./pkg/pgarrow/

fmt:
	gofmt -w .
	goimports -w .
	gci write .

compose:
	./docker-compose-tests.sh

test: sec lint

sec:
	gosec ./...

lint:
	golangci-lint run
