package server

import (
	"fmt"
	"net/http"

	"golang.org/x/time/rate"
)

type ipLimiter struct {
	limiter *rate.Limiter
	address string
}

var ipLimiters = map[string]ipLimiter{}

func isOverLimit(r *http.Request) error {
	addr, err := getRealClientIP(r)
	if err != nil {
		return fmt.Errorf("[server-IsOverLimit-1] could not get client ip address")
	}
	iplm, ok := ipLimiters[addr]
	if !ok {
		iplm = ipLimiter{
			limiter: rate.NewLimiter(1, 8),
			address: addr,
		}
		ipLimiters[addr] = iplm
	}
	if !iplm.limiter.Allow() {
		return fmt.Errorf("[server-IsOverLimit-2] ip address %s is over limit", addr)
	}
	return nil
}
