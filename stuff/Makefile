# This is mostly for convenience, we don't use any of the dependency
# features of 'make'
##

GO        ?= go
GOFMT     ?= $(GO)fmt

all : stuff test

stuff: *.go
	go build

test:
	go test

clean:
	rm -f stuff

style:
	@echo ">> checking code style"
	@fmtRes=$$($(GOFMT) -d $$(find . -path ./vendor -prune -o -name '*.go' -print)); \
	if [ -n "$${fmtRes}" ]; then \
		echo "gofmt checking failed!"; echo "$${fmtRes}"; echo; \
		echo "Please ensure you are using $$($(GO) version) for formatting code."; \
		exit 1; \
	fi
