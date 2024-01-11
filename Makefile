uname_p := $(shell uname -p) # store the output of the command in a variable

build: pre_build build_arrow

pre_build:
	./set_version.sh
	go mod tidy
	mkdir -p ./bin

build_local_image:
	docker build . -t mannemsolutions/pgarrow

build_arrow:
	go build -buildvcs=false -o ./bin/arrow ./cmd/arrow
	ln -f ./bin/arrow ./bin/arrow.$(uname_p)

build_dlv:
	go get github.com/go-delve/delve/cmd/dlv@latest
	mkdir -p ~/go/bin/dlv
	go build -o ~/bin/dlv.$(uname_p) github.com/go-delve/delve/cmd/dlv
	ln -sf ~/bin/dlv.$(uname_p) ~/bin/dlv

# Use the following on m1:
# alias make='/usr/bin/arch -arch arm64 /usr/bin/make'
debug_arrow:
	go build -gcflags "all=-N -l" -o ./bin/arrow.debug.$(uname_p) ./cmd/arrow
	~/bin/dlv --headless --listen=:2345 --api-version=2 --accept-multiclient exec ./bin/arrow.debug.$(uname_p)

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
