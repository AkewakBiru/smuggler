package h2_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"smuggler/smuggler/h2"
	"strings"
	"testing"
)

type test struct {
	req  *h2.Request
	want any
}

func TestGET(t *testing.T) {
	transport := h2.Transport{}

	table := []test{
		{req: &h2.Request{
			Method: http.MethodGet,
			URL:    &url.URL{Scheme: "https", Path: "/", Host: "www.youtube.com"},
			Body:   nil,
			Hdrs:   nil},
			want: http.StatusOK,
		},
		{req: &h2.Request{
			Method: http.MethodGet,
			URL:    &url.URL{Scheme: "https", Path: "/", Host: "www.google.com"},
			Body:   nil,
			Hdrs:   nil},
			want: http.StatusOK,
		},
		{req: &h2.Request{
			Method: http.MethodGet,
			URL:    &url.URL{Scheme: "https", Path: "/", Host: "httpbin.org"},
			Body:   nil,
			Hdrs:   nil},
			want: http.StatusOK,
		},
		{req: &h2.Request{
			Method: http.MethodGet,
			URL:    &url.URL{Scheme: "https", Path: "/", Host: "www.instagram.com"},
			Body:   nil,
			Hdrs: map[string][]string{
				"user-agent": {"my client"},
			}},
			want: http.StatusOK,
		},
		{req: &h2.Request{
			Method: http.MethodGet,
			URL:    &url.URL{Scheme: "https", Path: "/", Host: "medium.com"},
			Body:   nil,
			Hdrs:   nil},
			want: http.StatusOK,
		},
		{req: &h2.Request{
			Method: http.MethodGet,
			URL:    &url.URL{Scheme: "https", Path: "/", Host: "www.linkedin.com"},
			Body:   nil,
			Hdrs:   nil},
			want: http.StatusOK,
		},
	}

	for _, Case := range table {
		t.Run(Case.req.URL.Host, func(t *testing.T) {
			resp, err := transport.RoundTrip(Case.req)
			if err != nil {
				t.Error(err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != Case.want {
				t.Errorf("Wanted: %d, Got: %d", Case.want, resp.StatusCode)
			}

			// if resp.StatusCode == 200 {
			// 	if resp.Request != nil {
			// 		text := make([]byte, 100)
			// 		if _, err := resp.Body.Read(text); err != nil {
			// 			t.Error(err)
			// 			return
			// 		}
			// 		log.Print(string(text))
			// 	}
			// 	respH, err := httputil.DumpResponse(resp, false)
			// 	if err != nil {
			// 		t.Error(err)
			// 		return
			// 	}
			// 	log.Print(string(respH))
			// } else {
			// 	t.Errorf("Wanted: %d, Got: %d", Case.want, resp.StatusCode)
			// }
		})
	}
}

// i believe this is good enough for DATA testing
func TestPOST(t *testing.T) {
	table := []test{
		{req: &h2.Request{
			URL:    &url.URL{Scheme: "https", Host: "httpbin.org", Path: "/post"},
			Method: http.MethodPost,
			Body:   []byte(fmt.Sprintf("{\"key\": \"%s\"}", strings.Repeat("A", 10240))),
			Hdrs: map[string][]string{
				"user-agent":   {"my client"},
				"accept":       {"application/json"},
				"content-type": {"application/json"},
			}},
			want: strings.Repeat("A", 10240)},
	}

	for _, Case := range table {
		t.Run(Case.req.URL.Host, func(t *testing.T) {
			transport := h2.Transport{}
			resp, err := transport.RoundTrip(Case.req)
			if err != nil {
				t.Error(err)
				return
			}
			defer resp.Body.Close()

			result := make(map[string]any)
			dec := json.NewDecoder(resp.Body)
			if err := dec.Decode(&result); err != nil {
				t.Errorf("error while decoding: %v", err)
				return
			}
			kv := struct {
				Key string `json:"key"`
			}{}
			d, ok := result["data"].(string)
			if !ok {
				t.Error("Couldn't type assert")
				return
			}
			if err := json.NewDecoder(bytes.NewBuffer([]byte(d))).Decode(&kv); err != nil {
				fmt.Printf("%#v\n", d)
				t.Error(err)
				return
			} else if kv.Key != Case.want {
				t.Errorf("wanted: %s, Got: %s", Case.want, kv.Key)
				return
			}
		})
	}
}

func TestHEAD(t *testing.T) {
	transport := h2.Transport{}

	table := []test{
		{req: &h2.Request{
			Method: http.MethodHead,
			URL:    &url.URL{Scheme: "https", Path: "/", Host: "www.youtube.com"},
			Body:   nil,
			Hdrs:   nil},
			want: http.StatusOK,
		},
		{req: &h2.Request{
			Method: http.MethodHead,
			URL:    &url.URL{Scheme: "https", Path: "/", Host: "www.google.com"},
			Body:   nil,
			Hdrs:   nil},
			want: http.StatusOK,
		},
		{req: &h2.Request{
			Method: http.MethodHead,
			URL:    &url.URL{Scheme: "https", Path: "/", Host: "httpbin.org"},
			Body:   nil,
			Hdrs:   nil},
			want: http.StatusOK,
		},
		{req: &h2.Request{
			Method: http.MethodHead,
			URL:    &url.URL{Scheme: "https", Path: "/", Host: "www.instagram.com"},
			Body:   nil,
			Hdrs: map[string][]string{
				"user-agent": {"my client"},
			}},
			want: http.StatusOK,
		},
		{req: &h2.Request{
			Method: http.MethodHead,
			URL:    &url.URL{Scheme: "https", Path: "/", Host: "medium.com"},
			Body:   nil,
			Hdrs:   nil},
			want: http.StatusOK,
		},
		{req: &h2.Request{
			Method: http.MethodHead,
			URL:    &url.URL{Scheme: "https", Path: "/", Host: "www.linkedin.com"},
			Body:   nil,
			Hdrs:   nil},
			want: http.StatusOK,
		},
	}

	for _, Case := range table {
		t.Run(Case.req.URL.Host, func(t *testing.T) {
			resp, err := transport.RoundTrip(Case.req)
			if err != nil {
				t.Error(err)
				return
			}
			resp.Body.Close()

			if resp.StatusCode != Case.want {
				t.Errorf("Wanted: %d, Got: %d", Case.want, resp.StatusCode)
			}
		})
	}
}

func TestGETH2C(t *testing.T) {
	transport := h2.Transport{}

	table := []test{
		{req: &h2.Request{
			Method: http.MethodGet,
			URL:    &url.URL{Scheme: "http", Path: "/", Host: "localhost:4444"},
			Body:   nil,
			Hdrs:   nil,
			Mode:   h2.H2C},
			want: http.StatusOK,
		},
		{req: &h2.Request{
			Method: http.MethodGet,
			URL:    &url.URL{Scheme: "http", Path: "/", Host: "www.linkedin.com"},
			Body:   nil,
			Hdrs:   nil,
			Mode:   h2.H2C},
			want: fmt.Errorf("h2c is not supported: %s", "http://www.linkedin.com/"),
		},
		{req: &h2.Request{
			Method: http.MethodGet,
			URL:    &url.URL{Scheme: "http", Path: "/", Host: "httpbin.org"},
			Body:   nil,
			Hdrs:   nil,
			Mode:   h2.H2C},
			want: fmt.Errorf("h2c is not supported: %s", "http://httpbin.org/"),
		},
		{req: &h2.Request{
			Method: http.MethodGet,
			URL:    &url.URL{Scheme: "http", Path: "/", Host: "www.instagram.com"},
			Body:   nil,
			Hdrs: map[string][]string{
				"user-agent": {"my client"},
			},
			Mode: h2.H2C},
			want: fmt.Errorf("h2c is not supported: %s", "http://www.instagram.com/"),
		},
		{req: &h2.Request{
			Method: http.MethodGet,
			URL:    &url.URL{Scheme: "http", Path: "/", Host: "medium.com"},
			Body:   nil,
			Hdrs:   nil,
			Mode:   h2.H2C},
			want: fmt.Errorf("h2c is not supported: %s", "http://medium.com/"),
		},
	}

	for _, Case := range table {
		t.Run(Case.req.URL.Host, func(t *testing.T) {
			resp, err := transport.RoundTrip(Case.req)
			if err != nil {
				w, ok := Case.want.(error)
				if !ok {
					t.Error(err)
				} else {
					if w.Error() != err.Error() {
						t.Errorf("Wanted: %v, Got: %v", w, err)
					}
				}
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != Case.want {
				t.Errorf("Wanted: %d, Got: %d", Case.want, resp.StatusCode)
			}
		})
	}
}
