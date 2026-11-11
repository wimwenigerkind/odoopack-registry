package database

import (
	"testing"
	"time"
)

func TestNextSchemaWaitDoublesUpToTheCap(t *testing.T) {
	var got []time.Duration
	for d := schemaWaitFirst; len(got) < 7; d = nextSchemaWait(d) {
		got = append(got, d)
	}
	want := []time.Duration{1, 2, 4, 8, 16, 30, 30}
	for i := range want {
		if got[i] != want[i]*time.Second {
			t.Fatalf("delays = %v, want %v seconds", got, want)
		}
	}
}
