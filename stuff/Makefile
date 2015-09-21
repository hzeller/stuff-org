# This is mostly for convenience, we don't use any of the dependency
# features of 'make'
##

all : stuff test

stuff: *.go
	go build

test:
	go test

clean:
	rm -f stuff
