//go:build !darwin && !linux

package cli

import (
	"os"
	"time"
)

func checkoutCreatedAt(_ string, _ os.FileInfo) time.Time {
	return time.Time{}
}
