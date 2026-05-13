package timeutil

import "time"

var nowFn = time.Now

func Now() int64 { return nowFn().UnixNano() }

func SetNow(fn func() time.Time) { nowFn = fn }

func FormatNanoISO(ns int64) string {
	return time.Unix(0, ns).UTC().Format(time.RFC3339Nano)
}
