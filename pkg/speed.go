package pkg

import (
	"context"
	"time"

	"github.com/mineroot/torrent/pkg/event"
	"github.com/mineroot/torrent/pkg/torrent"
)

const twentySeconds = 20

func (c *Client) calculateDownloadSpeed(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer func() {
		ticker.Stop()
		close(c.progressSpeed)
	}()
	bytesByHash := make(map[torrent.Hash]int, c.storage.Len())
	bytesForTwentySecondsByHash := make(map[torrent.Hash][twentySeconds]int, c.storage.Len())

	for {
		select {
		case <-ctx.Done():
			return
		case connRead := <-c.progressConnReads:
			bytesByHash[connRead.Hash] += connRead.Bytes
		case <-ticker.C:
			for hash := range bytesByHash {
				bytesForTwentySeconds := bytesForTwentySecondsByHash[hash]
				bytesForTwentySeconds = shiftAndAddValue(bytesForTwentySeconds, bytesByHash[hash])
				bytesForTwentySecondsByHash[hash] = bytesForTwentySeconds

				avgSpeed := calculateAverageSpeed(bytesForTwentySeconds)
				select {
				case c.progressSpeed <- event.NewProgressSpeed(hash, avgSpeed):
				default:
				}
				bytesByHash[hash] = 0
			}
		}
	}
}

// shiftAndAddValue shifts the array to the left and adds the current value at the end
func shiftAndAddValue(arr [twentySeconds]int, value int) [twentySeconds]int {
	tmp := append(arr[1:], value)
	copy(arr[:], tmp)
	return arr
}

// calculateAverageSpeed calculates the average speed for the last twenty seconds
func calculateAverageSpeed(arr [twentySeconds]int) int {
	total := 0
	for _, b := range arr {
		total += b
	}
	return total / twentySeconds
}
