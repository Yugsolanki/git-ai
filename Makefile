.PHONY: build clean install

build:
	go build -o git-ai main.go

install:
	go build -o git-ai main.go
	sudo mv git-ai ~/.local/bin/

clean:
	rm -f git-ai