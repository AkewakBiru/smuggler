package smuggler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"smuggler/smuggler"
	"strings"
	"testing"
	"time"
)

type test struct {
	method  string
	host    string
	scheme  string
	query   string
	path    string
	body    string
	hdrs    map[string]string
	timeout time.Duration

	want any
}

func buildReqLine(_test *test) *smuggler.Request {
	url := url.URL{Scheme: _test.scheme, Host: _test.host, Path: _test.path}
	payload := smuggler.Payload{}
	if len(_test.body) > 0 {
		payload.Body = _test.body
		payload.Cl = len(payload.Body)
	}
	payload.ReqLine = smuggler.ReqLine{
		Method:  _test.method,
		Query:   _test.query,
		Version: "HTTP/1.1",
	}
	if len(url.Path) == 0 {
		payload.ReqLine.Path = "/"
	} else {
		payload.ReqLine.Path = url.Path
	}

	if _test.hdrs != nil {
		headers := make(map[string]string)
		for k, v := range _test.hdrs {
			headers[k] = v
		}
		payload.Header = headers
	}
	return &smuggler.Request{Payload: &payload, Url: &url, Timeout: _test.timeout}
}

func buildReqHdr(lst []string) map[string]string {
	res := make(map[string]string)

	for i := 0; i < len(lst); i += 2 {
		res[lst[i]] = lst[i+1]
	}
	return res
}

func TestRoundTripGET(t *testing.T) {
	table := []test{
		{
			method:  http.MethodGet,
			host:    "www.google.com",
			scheme:  "https",
			path:    "/",
			hdrs:    buildReqHdr([]string{"Host", "www.google.com"}),
			timeout: time.Second * 3,
			want:    http.StatusOK,
		},
		{
			method:  http.MethodGet,
			host:    "httpbin.org",
			scheme:  "https",
			path:    "/",
			hdrs:    buildReqHdr([]string{"Host", "httpbin.org"}),
			timeout: time.Second * 3,
			want:    http.StatusOK,
		},
		{
			method:  http.MethodGet,
			host:    "www.instagram.com",
			scheme:  "https",
			path:    "/",
			hdrs:    buildReqHdr([]string{"Host", "www.instagram.com", "User-Agent", "my-agent"}),
			timeout: time.Second * 3,
			want:    http.StatusOK,
		},
	}

	tr := smuggler.Transport{}
	for _, Case := range table {
		t.Run(Case.method, func(t *testing.T) {
			req := buildReqLine(&Case)
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
			method:  http.MethodPost,
			host:    "www.google.com",
			scheme:  "https",
			path:    "/",
			hdrs:    buildReqHdr([]string{"Host", "www.google.com"}),
			timeout: time.Second * 3,
			want:    http.StatusLengthRequired,
		},
		{
			method:  http.MethodPost,
			host:    "httpbin.org",
			scheme:  "https",
			path:    "/post",
			hdrs:    buildReqHdr([]string{"Host", "httpbin.org", "accept", "application/json"}),
			timeout: time.Second * 3,
			want:    http.StatusOK,
		},
		{
			method:  http.MethodPost,
			host:    "httpbin.org",
			scheme:  "https",
			path:    "/post",
			hdrs:    buildReqHdr([]string{"Host", "httpbin.org", "accept", "application/json", "content-type", "application/json"}),
			body:    fmt.Sprintf("{\"key\":\"%s\"}", strings.Repeat("A", 1024)),
			timeout: time.Second * 3,
			want:    strings.Repeat("A", 1024),
		},
	}

	tr := smuggler.Transport{}
	for _, Case := range table {
		t.Run(Case.method, func(t *testing.T) {
			req := buildReqLine(&Case)
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
			method:  http.MethodHead,
			host:    "www.google.com",
			scheme:  "https",
			path:    "/",
			hdrs:    buildReqHdr([]string{"Host", "www.google.com"}),
			timeout: time.Second * 3,
			want:    http.StatusOK,
		},
		{
			method:  http.MethodHead,
			host:    "httpbin.org",
			scheme:  "https",
			path:    "/",
			hdrs:    buildReqHdr([]string{"Host", "httpbin.org"}),
			timeout: time.Second * 3,
			want:    http.StatusOK,
		},
		{
			method:  http.MethodHead,
			host:    "www.instagram.com",
			scheme:  "https",
			path:    "/",
			hdrs:    buildReqHdr([]string{"Host", "www.instagram.com", "User-Agent", "my-agent"}),
			timeout: time.Second * 3,
			want:    http.StatusOK,
		},
	}

	tr := smuggler.Transport{}
	for _, Case := range table {
		t.Run(Case.method, func(t *testing.T) {
			req := buildReqLine(&Case)
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
			method:  http.MethodOptions,
			host:    "httpbin.org",
			scheme:  "https",
			path:    "/",
			hdrs:    buildReqHdr([]string{"Host", "httpbin.org"}),
			timeout: time.Second * 3,
			want:    http.StatusOK,
		},
	}

	tr := smuggler.Transport{}
	for _, Case := range table {
		t.Run(Case.method, func(t *testing.T) {
			req := buildReqLine(&Case)
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
			method:  http.MethodGet,
			host:    "postman-echo.com",
			scheme:  "https",
			path:    "/delay/3",
			hdrs:    buildReqHdr([]string{"Host", "postman-echo.com"}),
			timeout: time.Second * 2,
			want:    nil,
		},
	}

	tr := smuggler.Transport{}
	for _, Case := range table {
		t.Run(Case.method, func(t *testing.T) {
			req := buildReqLine(&Case)
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

// 	tr := smuggler.Transport{}
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
