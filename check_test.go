package main

import (
	"testing"
)

func TestDeadlineTime(t *testing.T) {
	dt := DeadlineTime("2021-10-03")
	if dt.Year() != 2021 || dt.Month() != 10 || dt.Day() != 4 || dt.Hour() != 0 {
		t.Errorf("Wrong deadline time: %s", dt)
	}
}
