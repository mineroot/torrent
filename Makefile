build:
	go build -o ./bin/torrent-client

run: build
	./bin/torrent-client

test:
	go test -race ./...
