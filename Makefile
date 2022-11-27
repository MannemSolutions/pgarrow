uname_p := $(shell uname -p) # store the output of the command in a variable

build: pre_build build_kafka build_rabbit

pre_build:
	./set_version.sh
	go mod tidy
	mkdir -p ./bin

build_kafka:
	go build -o ./bin/pgarrowkafka.$(uname_p) ./cmd/pgarrowkafka
	go build -o ./bin/kafkaarrowpg.$(uname_p) ./cmd/kafkaarrowpg

build_rabbit:
	go build -o ./bin/pgarrowrabbit.$(uname_p) ./cmd/pgarrowrabbit
	go build -o ./bin/rabbitarrowpg.$(uname_p) ./cmd/rabbitarrowpg

build_dlv:
	go get github.com/go-delve/delve/cmd/dlv@latest
	mkdir -p ~/go/bin/dlv
	go build -o ~/bin/dlv.$(uname_p) github.com/go-delve/delve/cmd/dlv
	ln -sf ~/bin/dlv.$(uname_p) ~/bin/dlv

# Use the following on m1:
# alias make='/usr/bin/arch -arch arm64 /usr/bin/make'
debug_pgarrowkafka:
	go build -gcflags "all=-N -l" -o ./bin/pgarrowkafka.debug.$(uname_p) ./cmd/pgarrowkafka
	~/bin/dlv --headless --listen=:2345 --api-version=2 --accept-multiclient exec ./bin/pgarrowkafka.debug.$(uname_p)

debug_kafkaarrowpg:
	go build -gcflags "all=-N -l" -o ./bin/kafkaarrowpg.debug.$(uname_p) ./cmd/kafkaarrowpg
	~/bin/dlv --headless --listen=:2345 --api-version=2 --accept-multiclient exec ./bin/kafkaarrowpg.debug.$(uname_p)

debug_pgarrowrabbit:
	go build -gcflags "all=-N -l" -o ./bin/pgarrowrabbit.debug.$(uname_p) ./cmd/pgarrowrabbit
	~/bin/dlv --headless --listen=:2345 --api-version=2 --accept-multiclient exec ./bin/pgarrowrabbit.debug.$(uname_p)

debug_rabbitarrowpg:
	go build -gcflags "all=-N -l" -o ./bin/rabbitarrowpg.debug.$(uname_p) ./cmd/rabbitarrowpg
	~/bin/dlv --headless --listen=:2345 --api-version=2 --accept-multiclient exec ./bin/rabbitarrowpg.debug.$(uname_p)



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
