package testharness_test

import (
	"testing"
	"time"

	th "github.com/0xkhdr/specd/internal/testharness"
)

func TestFakeClockAutoAdvances(t *testing.T) {
	c := th.NewFakeClock()
	first, second := c.Now(), c.Now()
	if !first.Equal(th.Epoch) {
		t.Errorf("first read = %v, want Epoch %v", first, th.Epoch)
	}
	if !second.Equal(th.Epoch.Add(time.Second)) {
		t.Errorf("second read = %v, want Epoch+1s", second)
	}
}

func TestFakeClockSet(t *testing.T) {
	c := th.NewFakeClock()
	target := time.Date(2030, 5, 6, 7, 8, 9, 0, time.UTC)
	c.Set(target)
	if got := c.Now(); !got.Equal(target) {
		t.Errorf("after Set, Now = %v, want %v", got, target)
	}
}

func TestFakeClockAdvanceWithoutConsumingRead(t *testing.T) {
	c := th.NewFakeClock()
	c.Advance(time.Hour)
	if got := c.Now(); !got.Equal(th.Epoch.Add(time.Hour)) {
		t.Errorf("after Advance(1h), Now = %v, want Epoch+1h", got)
	}
}

func TestFakeClockSetStep(t *testing.T) {
	c := th.NewFakeClock()
	c.SetStep(time.Minute)
	_ = c.Now()
	if got := c.Now(); !got.Equal(th.Epoch.Add(time.Minute)) {
		t.Errorf("with step=1m, second read = %v, want Epoch+1m", got)
	}
}

func TestFakeClockFreezeIsConstant(t *testing.T) {
	c := th.NewFakeClock()
	c.Freeze()
	if a, b := c.Now(), c.Now(); !a.Equal(b) {
		t.Errorf("frozen clock advanced: %v then %v", a, b)
	}
}
