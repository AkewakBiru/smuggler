package config

import (
	"net/url"
	"sync"
	"time"
)

type Priority uint8
type LEVEL byte

const (
	B LEVEL = iota
	M
	E
)

const (
	H2CLTE Priority = iota
	H2TECL
	TECLH2
	TEH2CL
	CLTEH2
	CLH2TE
)

type Global struct {
	Test LEVEL

	ExitEarly  bool
	Concurrent bool

	Priority Priority

	Timeout time.Duration
	Wg      sync.WaitGroup
	DestURL *url.URL
}

var Glob Global
