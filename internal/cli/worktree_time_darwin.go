package cli

import (
	"os"
	"syscall"
	"time"
)

func checkoutCreatedAt(_ string, info os.FileInfo) time.Time {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Birthtimespec == (syscall.Timespec{}) {
		return time.Time{}
	}
	return time.Unix(stat.Birthtimespec.Unix())
}
