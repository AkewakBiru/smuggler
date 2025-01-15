package config

import (
	"sync"
	"time"
)

type Global struct {
	Method    string
	ExitEarly bool
	Timeout   time.Duration
	Test      string
	Wg        sync.WaitGroup
}

var Glob Global
