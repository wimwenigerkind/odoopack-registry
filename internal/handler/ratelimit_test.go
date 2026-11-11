package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestTakeSpendsTheBurstAndRefills(t *testing.T) {
	start := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	var b bucket
	var ok bool
	for i := range 3 {
		if b, _, ok = take(b, start, 3, 30*time.Second); !ok {
			t.Fatalf("request %d of the burst was refused", i+1)
		}
	}

	b, wait, ok := take(b, start.Add(10*time.Second), 3, 30*time.Second)
	if ok || wait != 20*time.Second {
		t.Fatalf("after the burst: ok = %v, wait = %s, want refused with 20s", ok, wait)
	}
	if b, _, ok = take(b, start.Add(30*time.Second), 3, 30*time.Second); !ok {
		t.Fatal("refused after one refill interval")
	}
	if _, _, ok = take(b, start.Add(31*time.Second), 3, 30*time.Second); ok {
		t.Fatal("allowed a second request on one refilled token")
	}

	// A long pause refills to the burst and no further.
	b, _, _ = take(b, start.Add(time.Hour), 3, 30*time.Second)
	if b.tokens != 2 {
		t.Errorf("tokens after a long pause and one request = %v, want 2", b.tokens)
	}
}

func TestUserLimiterKeepsUsersApart(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	l := newUserLimiter(1, time.Minute, func() time.Time { return now })
	alice, bob := uuid.New(), uuid.New()

	if _, ok := l.allow(alice); !ok {
		t.Fatal("first request refused")
	}
	if wait, ok := l.allow(alice); ok || wait != time.Minute {
		t.Fatalf("second request: ok = %v, wait = %s, want refused with 1m", ok, wait)
	}
	if _, ok := l.allow(bob); !ok {
		t.Fatal("another user was limited by the first one")
	}
	now = now.Add(time.Minute)
	if _, ok := l.allow(alice); !ok {
		t.Fatal("refused after the refill")
	}
}

func TestTooManyRequestsSetsRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	tooManyRequests(c, 1500*time.Millisecond)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", w.Code)
	}
	if got := w.Header().Get("Retry-After"); got != "2" {
		t.Errorf("Retry-After = %q, want 2", got)
	}
}
