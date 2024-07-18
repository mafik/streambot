package backoff

import (
	"fmt"
	"time"

	"github.com/fatih/color"
)

var backoffSleepTimes = []time.Duration{
	time.Second * 1,
	time.Second * 5,
	time.Second * 10,
	time.Second * 30,
	time.Minute,
	time.Minute * 5,
	time.Minute * 10,
}

type Backoff struct {
	attempts     int
	last_attempt time.Time
	Description  string
	Color        *color.Color
}

func (b *Backoff) printf(format_string string, args ...interface{}) {
	if b.Color == nil {
		fmt.Printf(format_string, args...)
	} else {
		b.Color.Printf(format_string, args...)
	}
}

func (b *Backoff) Attempt() {
	if time.Since(b.last_attempt) > time.Minute {
		b.attempts = 0
	} else {
		backoffIndex := min(b.attempts, len(backoffSleepTimes)-1)
		b.attempts++
		sleep_time := backoffSleepTimes[backoffIndex]
		b.printf("Backing off %s for %s\n", b.Description, sleep_time)
		time.Sleep(sleep_time)
	}
	b.last_attempt = time.Now()
}

func (b *Backoff) Success() {
	if b.attempts > 0 {
		b.printf("%s recovered after %d attempts\n", b.Description, b.attempts)
	}
	b.attempts = 0
	b.last_attempt = time.UnixMilli(0)
}
