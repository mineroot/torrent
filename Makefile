build:
	go build -o ./bin/torrent-client github.com/mineroot/torrent/cmd/torrentclient

run: build
	./bin/torrent-client

test:
	go test -race ./...
