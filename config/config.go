package config

import (
	"net/url"
	"sync"
	"time"
)

type Global struct {
	Method    string
	ExitEarly bool
	Timeout   time.Duration
	Test      string
	Wg        sync.WaitGroup
	DestURL   *url.URL
}

var Glob Global
