package positions

import "time"

type RealTicker struct {
	ticker *time.Ticker
}

func NewRealTicker(d time.Duration) *RealTicker {
	return &RealTicker{
		ticker: time.NewTicker(d),
	}
}

func (r *RealTicker) C() <-chan time.Time {
	return r.ticker.C
}

func (r *RealTicker) Stop() {
	r.ticker.Stop()
}
