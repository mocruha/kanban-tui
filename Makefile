BIN  := $(HOME)/local/bin/kanban
PATHFILE ?= $(HOME)/.local/share/kanban/board.json

.PHONY: build test vet fmt install uninstall

build:
	go build -o kanban .

test:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

install: build
	mkdir -p $(dir $(BIN))
	install -m 0755 kanban $(BIN)

uninstall:
	rm -f $(BIN) $(BIN).bak $(PATHFILE).bak
