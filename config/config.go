package config

import (
	"net/url"
	"sync"
	"time"
)

type Priority uint8

const (
	H2CLTE Priority = iota
	H2TECL
	TECLH2
	TEH2CL
	CLTEH2
	CLH2TE
)

type Global struct {
	Method string
	Test   string

	ExitEarly  bool
	Concurrent bool

	Priority Priority

	Timeout time.Duration
	Wg      sync.WaitGroup
	DestURL *url.URL
}

var Glob Global
