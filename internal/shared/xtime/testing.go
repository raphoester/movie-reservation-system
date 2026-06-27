package xtime

import "time"

func GetTestTime() time.Time {
	return time.Date(2024, time.October, 10, 0, 0, 0, 0, time.UTC)
}
