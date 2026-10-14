.PHONY: vet build test

GOCMD=GO111MODULE=on go

vet:
	${GOCMD} vet ${GOARGS} ./...

build:
	${GOCMD} build ${GOARGS} ./...

test:
	${GOCMD} test ${GOARGS} ./...
