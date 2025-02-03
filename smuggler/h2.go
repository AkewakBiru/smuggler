package smuggler

import (
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net"
	"os"
	"smuggler/config"
	"smuggler/smuggler/h2"
	"smuggler/smuggler/tests"
	"smuggler/utils"
	"time"

	"github.com/rs/zerolog/log"
)

type H2 struct {
	*DesyncerImpl
}

func (h *H2) Run() bool {
	if config.Glob.Concurrent {
		defer h.Wg.Done()
	}
	// create a list of payloads and loop over them
	priorityOrder := map[config.Priority][]tests.PTYPE{
		config.H2CLTE: {tests.CL, tests.TE, tests.CRLF},
		config.H2TECL: {tests.TE, tests.CL, tests.CRLF},
	}

	order, exists := priorityOrder[config.Glob.Priority]
	if exists {
		log.Warn().
			Any("Priority", config.Glob.Priority).Msg("Unknown priority, defaulting to H2CLTE")
		order = []tests.PTYPE{tests.CRLF, tests.CL, tests.TE}
	}
	for _, t := range order {
		if h.run(t) {
			return true
		}
	}
	return false
}

func (h *H2) run(t tests.PTYPE) bool {
	log.Info().Str("endpoint", h.URL.String()).Msgf("Running H2-%s desync tests...", t.String())
	ctr := 0
	generator := tests.Generator{}
	pl := generator.Generate(t, config.Glob.Test)
	for k, vv := range pl {
		for _, v := range vv {
			var req *h2.Request
			if t == tests.CL {
				req = h.newRequest(v, k)
			} else {
				req = h.newRequest(k, v)
			}

			if h.runTest(req, t) {
				ctr++
				if config.Glob.ExitEarly {
					log.Info().
						Str("endpoint", h.URL.String()).
						Str("status", "success").
						Msgf("Test stopped on success: PoC payload stored in /result/%s directory", h.URL.Hostname())
					if config.Glob.Concurrent {
						h.TestDone <- struct{}{}
					}
					return true
				}
			}
			if config.Glob.Concurrent {
				select {
				case <-h.Ctx.Done():
					return false
				default:
				}
			}
		}
	}
	if ctr > 0 { // if eos, it shouldn't even come here on success
		log.Info().
			Str("endpoint", h.URL.String()).
			Str("status", "success").
			Msgf("finished H2%s desync tests: PoC payload stored in /result/%s directory", h.URL.Hostname(), t.String())
	} else {
		log.Info().
			Str("endpoint", h.URL.String()).
			Str("status", "failure").
			Msgf("finished H2%s desync tests: no issues found", t.String())
	}
	return false
}

func (h *H2) runTest(req *h2.Request, t tests.PTYPE) bool {
	ctr := 0
	for {
		t.Body(req, false)
		ret, err := h.sendRequest(req)
		if ret != 1 {
			if ret == -1 {
				log.Debug().
					Str("endpoint", h.URL.String()).Err(err).Msg("")
			} else if ret == 2 {
				log.Debug().
					Str("endpoint", h.URL.String()).
					Msg("disconnected before timeout")
			}
			return false
		}
		t.Body(req, true)
		ret2, err := h.sendRequest(req)
		if ret2 == -1 {
			log.Debug().
				Str("endpoint", h.URL.String()).Err(err).Msg("")
			return false
		}
		if ret2 == 0 {
			ctr++
			if ctr < 3 {
				continue
			}
			t.Body(req, false)
			log.Info().
				Str("endpoint", h.URL.String()).
				Msgf("Potential H2%s issue found - %s@%s://%s%s", t.String(), h.Method,
					h.URL.Scheme, h.URL.Host, h.URL.Path)
			// generate a report here
			h.generateH2Report(req)
			return true
		}
		log.Debug().
			Str("endpoint", h.URL.String()).
			// Str("payload", h.HdrPl).
			Err(err).Msgf("%s timeout on both payloads", t.String())
		return false
	}
}

func (h *H2) generateH2Report(req *h2.Request) {
	if err := createDir("/result/"); err != nil {
		log.Warn().Err(err).Msg("")
	}
	if err := createDir(fmt.Sprintf("/result/%s", h.URL.Hostname())); err != nil {
		log.Warn().Err(err).Msg("")
	}
	pwd, err := os.Getwd()
	if err != nil {
		log.Warn().Err(err).Msg("")
		return
	}
	fname := fmt.Sprintf("%s/result/%s/%ss", pwd, h.URL.Hostname(), h.URL.Query().Get("t"))
	file, err := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Warn().Err(err).Msg("")
		return
	}
	defer file.Close()

	if _, err := file.WriteString(utils.GetH2RequestSummary(req)); err != nil {
		log.Warn().Err(err).Msg("Failed to write report to file")
	}
}

func (h *H2) newRequest(key, val string) *h2.Request {
	req := &h2.Request{
		URL:    h.URL,
		Method: h.Method,
	}
	req.Hdrs = utils.CloneMap(h.Hdr)
	if len(key) > 0 {
		req.Payload = &h2.Payload{Key: key, Val: val}
	}
	return req
}

func (h *H2) sendRequest(req *h2.Request) (int, error) {
	t := h2.Transport{}
	req.URL = h.URL
	q := req.URL.Query()
	q.Set("t", fmt.Sprintf("%d", rand.Int32N(math.MaxInt32))) // avoid caching
	req.URL.RawQuery = q.Encode()
	start := time.Now()
	resp, err := t.RoundTrip(req)
	if err != nil {
		var netErr net.Error // check for timeout error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return 1, err
		}
		return 0, err
	}
	diff := time.Since(start)

	sample := make([]byte, 100)
	if _, err := resp.Body.Read(sample); err != nil && err != io.EOF {
		return -1, err
	}
	resp.Body.Close()
	if len(sample) == 0 {
		if diff < config.Glob.Timeout-time.Second {
			return 2, nil
		}
		return 1, nil
	}
	return 0, nil
}
