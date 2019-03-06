GOBUILD = GOARCH=$(GOARCH) GOOS=$(GOOS) go build
GOFLAGS := -ldflags="-s -w"
PROGRAM := spidrv
SRC := spidrv.go
GOOS ?= linux
GOARCH ?= arm

all: spidrv

spidrv:
	$(GOBUILD) $(GOFLAGS)

rebuild:
	$(GOBUILD) -a $(GOFLAGS)

install: spidrv
	cp -f $(PROGRAM) ~/olimexfs/gocode/src/github.com/cakturk/spidrv/

clean:
	@-rm -f $(PROGRAM)

.PHONY: install clean spidrv rebuild
