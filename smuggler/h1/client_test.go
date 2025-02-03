package h1_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"smuggler/smuggler/h1"

	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type test struct {
	host    string
	scheme  string
	path    string
	body    string
	hdrs    map[string]string
	timeout time.Duration

	want any
}

func buildReqLine(_test *test, method string) *h1.Request {
	url := url.URL{Scheme: _test.scheme, Host: _test.host, Path: _test.path}
	payload := h1.Payload{URL: url, Method: method}
	if len(_test.body) > 0 {
		payload.Body = _test.body
		payload.Cl = len(payload.Body)
	}

	if _test.hdrs != nil {
		headers := make(map[string]string)
		for k, v := range _test.hdrs {
			headers[k] = v
		}
		payload.Header = headers
	}
	return &h1.Request{Payload: &payload, Url: &url, Timeout: _test.timeout}
}

func buildReqHdr(lst []string) map[string]string {
	res := make(map[string]string)

	for i := 0; i < len(lst); i += 2 {
		res[lst[i]] = lst[i+1]
	}
	return res
}

func TestRoundTripGET(t *testing.T) {
	// config.Glob.Method = http.MethodGet
	table := []test{
		{
			host:    "www.google.com",
			scheme:  "https",
			path:    "/",
			hdrs:    buildReqHdr([]string{"Host", "www.google.com"}),
			timeout: time.Second * 3,
			want:    http.StatusOK,
		},
		{
			host:    "httpbin.org",
			scheme:  "https",
			path:    "/",
			hdrs:    buildReqHdr([]string{"Host", "httpbin.org"}),
			timeout: time.Second * 3,
			want:    http.StatusOK,
		},
		{
			host:    "www.instagram.com",
			scheme:  "https",
			path:    "/",
			hdrs:    buildReqHdr([]string{"Host", "www.instagram.com", "User-Agent", "my-agent"}),
			timeout: time.Second * 3,
			want:    http.StatusOK,
		},
	}

	tr := h1.Transport{}
	for _, Case := range table {
		t.Run(Case.host, func(t *testing.T) {
			req := buildReqLine(&Case, http.MethodGet)
			resp, err := tr.RoundTrip(req)
			if err != nil {
				t.Error(err)
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != Case.want.(int) {
				t.Errorf("Wanted: %d, Got: %d", Case.want, resp.StatusCode)
			}
		})
	}
}

func TestRoundTripPOST(t *testing.T) {
	table := []test{
		{
			host:    "www.google.com",
			scheme:  "https",
			path:    "/",
			hdrs:    buildReqHdr([]string{"Host", "www.google.com"}),
			timeout: time.Second * 3,
			want:    http.StatusLengthRequired,
		},
		{
			host:    "httpbin.org",
			scheme:  "https",
			path:    "/post",
			hdrs:    buildReqHdr([]string{"Host", "httpbin.org", "accept", "application/json"}),
			timeout: time.Second * 3,
			want:    http.StatusOK,
		},
		{
			host:    "httpbin.org",
			scheme:  "https",
			path:    "/post",
			hdrs:    buildReqHdr([]string{"Host", "httpbin.org", "accept", "application/json", "content-type", "application/json"}),
			body:    fmt.Sprintf("{\"key\":\"%s\"}", strings.Repeat("A", 1024)),
			timeout: time.Second * 3,
			want:    strings.Repeat("A", 1024),
		},
	}

	tr := h1.Transport{}
	for _, Case := range table {
		t.Run(Case.host, func(t *testing.T) {
			req := buildReqLine(&Case, http.MethodPost)
			resp, err := tr.RoundTrip(req)
			if err != nil {
				t.Error(err)
				return
			}
			defer resp.Body.Close()
			switch want := Case.want.(type) {
			case int:
				if resp.StatusCode != Case.want {
					t.Errorf("wanted: %d, Got: %d", want, resp.StatusCode)
				}
			case string:
				result := make(map[string]any)
				dec := json.NewDecoder(resp.Body)
				if err := dec.Decode(&result); err != nil {
					t.Error("error while decoding:", err)
					return
				}
				kv := struct {
					Key string `json:"key"`
				}{}
				d, ok := result["data"].(string)
				if !ok {
					t.Error("type assertion error")
					return
				}
				if err := json.NewDecoder(bytes.NewBuffer([]byte(d))).Decode(&kv); err != nil {
					t.Error(err)
					return
				}
				if want != kv.Key {
					t.Errorf("wanted: %s, Got: %s", want, kv.Key)
				}
			}
		})
	}
}

func TestRoundTripHEAD(t *testing.T) {
	table := []test{
		{
			host:    "www.google.com",
			scheme:  "https",
			path:    "/",
			hdrs:    buildReqHdr([]string{"Host", "www.google.com"}),
			timeout: time.Second * 3,
			want:    http.StatusOK,
		},
		{
			host:    "httpbin.org",
			scheme:  "https",
			path:    "/",
			hdrs:    buildReqHdr([]string{"Host", "httpbin.org"}),
			timeout: time.Second * 3,
			want:    http.StatusOK,
		},
		{
			host:    "www.instagram.com",
			scheme:  "https",
			path:    "/",
			hdrs:    buildReqHdr([]string{"Host", "www.instagram.com", "User-Agent", "my-agent"}),
			timeout: time.Second * 3,
			want:    http.StatusOK,
		},
	}

	tr := h1.Transport{}
	for _, Case := range table {
		t.Run(Case.host, func(t *testing.T) {
			req := buildReqLine(&Case, http.MethodHead)
			resp, err := tr.RoundTrip(req)
			if err != nil {
				t.Error(err)
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != Case.want.(int) {
				t.Errorf("Wanted: %d, Got: %d", Case.want, resp.StatusCode)
			}
		})
	}
}

func TestRoundTripOPTIONS(t *testing.T) {
	table := []test{
		{
			host:    "httpbin.org",
			scheme:  "https",
			path:    "/",
			hdrs:    buildReqHdr([]string{"Host", "httpbin.org"}),
			timeout: time.Second * 3,
			want:    http.StatusOK,
		},
	}

	tr := h1.Transport{}
	for _, Case := range table {
		t.Run(Case.host, func(t *testing.T) {
			req := buildReqLine(&Case, http.MethodOptions)
			resp, err := tr.RoundTrip(req)
			if err != nil {
				t.Error(err)
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != Case.want {
				t.Errorf("Wanted: %d, Got: %d", Case.want, resp.StatusCode)
			}
		})
	}
}

func TestRoundTripReadTimeout(t *testing.T) {
	table := []test{
		{
			host:    "postman-echo.com",
			scheme:  "https",
			path:    "/delay/3",
			hdrs:    buildReqHdr([]string{"Host", "postman-echo.com"}),
			timeout: time.Second * 2,
			want:    nil,
		},
	}

	tr := h1.Transport{}
	for _, Case := range table {
		t.Run(Case.host, func(t *testing.T) {
			req := buildReqLine(&Case, http.MethodGet)
			resp, err := tr.RoundTrip(req)
			if err != nil {
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Error(err)
				}
				return
			}
			resp.Body.Close()
			t.Errorf("Wanted a timeout, Got: %d", resp.StatusCode)
		})
	}
}

// func TestRoundTripWriteTimeout(t *testing.T) {
// 	table := []test{
// 		{
// 			method: http.MethodGet,
// 			host:   "httpstat.us",
// 			scheme: "http",
// 			path:   "/200",
// 			query:  "sleep=10000",
// 			hdrs:   buildReqHdr([]string{"Host", "httpstat.us"}),
// 			want:   nil,
// 		},
// 	}

// 	tr := h1.Transport{}
// 	for _, Case := range table {
// 		dur := time.Second * 4
// 		time.AfterFunc(dur*2, func() {
// 			fmt.Println("Might be a write timeout")
// 		})
// 		t.Run(Case.method, func(t *testing.T) {
// 			req := BuildReqLine(&Case)
// 			req.Timeout = time.Second * 2
// 			fmt.Println(req.Payload.ToString())
// 			resp, err := tr.RoundTrip(req)
// 			if err != nil {
// 				var netErr net.Error
// 				fmt.Println(err)
// 				if errors.As(err, &netErr); !netErr.Timeout() {
// 					t.Error(err)
// 				}
// 				return
// 			}
// 			resp.Body.Close()
// 			t.Errorf("Wanted a timeout, Got: %d", resp.StatusCode)
// 		})
// 	}
// }
