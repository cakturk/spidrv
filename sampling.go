package main

import (
	"errors"
	"io"
)

type adc8ChanDaisy struct {
	m mapper
	r io.Reader
}

type rawSample struct {
	buf [24]byte
}

// size of s must be multiple of 24
func (a *adc8ChanDaisy) readSamples(s []rawSample) (int, error) {
	var nrSamples int
	for i := range s {
		p := s[i].buf[:]
		n, err := a.r.Read(p)
		if err != nil {
			return 0, err
		}
		if n < len(p) {
			return nrSamples, errors.New("readSamples: short read")
		}
		nrSamples++
	}
	return nrSamples, nil
}
