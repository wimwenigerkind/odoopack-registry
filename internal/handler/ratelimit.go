package handler

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Manual syncs: a user may start syncBurst at once and then one every
// syncRefill, across all addons; each addon syncs manually at most once per
// limits.sync_interval, whoever asks. Webhook pushes are not counted: the
// unique index on active sync jobs already folds them into one pending job
// per addon.
const (
	syncBurst  = 10
	syncRefill = 30 * time.Second
)

type bucket struct {
	tokens float64
	at     time.Time
}

// take refills b for the time since it was last used and takes one token. If
// none is left it returns b unchanged and the wait until the next token.
func take(b bucket, now time.Time, burst int, refill time.Duration) (bucket, time.Duration, bool) {
	if b.at.IsZero() {
		b = bucket{tokens: float64(burst), at: now}
	}
	if elapsed := now.Sub(b.at); elapsed > 0 {
		b.tokens = math.Min(float64(burst), b.tokens+float64(elapsed)/float64(refill))
		b.at = now
	}
	if b.tokens < 1 {
		return b, time.Duration((1 - b.tokens) * float64(refill)), false
	}
	b.tokens--
	return b, 0, true
}

// userLimiter keeps one token bucket per user in this process. Only signed in
// users reach it, so the map is bounded by the number of accounts. With
// several web replicas a user gets the burst once per replica; the database
// interval per addon is the limit that holds across all of them.
type userLimiter struct {
	mu      sync.Mutex
	buckets map[uuid.UUID]bucket
	now     func() time.Time
	burst   int
	refill  time.Duration
}

func newUserLimiter(burst int, refill time.Duration, now func() time.Time) *userLimiter {
	return &userLimiter{buckets: map[uuid.UUID]bucket{}, now: now, burst: burst, refill: refill}
}

func (l *userLimiter) allow(user uuid.UUID) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, wait, ok := take(l.buckets[user], l.now(), l.burst, l.refill)
	l.buckets[user] = b
	return wait, ok
}

func tooManyRequests(c *gin.Context, wait time.Duration) {
	c.Header("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
	c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many sync requests, retry later"})
}
