build:
	go build -o ./bin/torrent-client github.com/mineroot/torrent/cmd/torrentclient

test:
	go test -race ./...

faketracker:
	go build -o ./bin/faketracker github.com/mineroot/torrent/cmd/faketracker && ./bin/faketracker
