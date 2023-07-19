package utils

import (
	"fmt"
	"strconv"
)

func FormatBytes[T uint | uint8 | uint16 | uint32 | uint64](bytes T) string {
	const unit = 1024
	b := uint64(bytes)
	if b < unit {
		return strconv.FormatUint(b, 10) + " B"
	}
	div, exp := unit, 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
