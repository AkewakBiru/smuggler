package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"smuggler/smuggler"
	"strings"
)

var glob smuggler.Glob

func init() {
	flag.Usage = func() {
		h := "\nHTTP Request Smuggling tester\n"
		h += "Usage: "
		h += "smuggler [Options]\n\n"
		h += "-u, --url destination server address to test\n"
		h += "-s, --scheme scheme for the url (use http|https)\n"
		h += "-e, --exit-early exit as soon as a Desync is detected"
		fmt.Fprintln(os.Stderr, h)
	}
}

// TODO: add verbosity (print request/response headers to stdout)
//
// default to POST method if method isn't provided
// default read timeout is set to 5 seconds
func main() {
	uri := flag.String("url", "", "--url \"https://www.google.com\"")
	flag.StringVar(uri, "u", "", "-u \"https://www.google.com\"")
	method := flag.String("method", "POST", "--method \"GET\"")
	flag.StringVar(method, "X", "POST", "-X \"GET\"")
	port := flag.Int("port", 443, "--port 443")
	flag.IntVar(port, "p", 443, "-p 443")
	exitOnSuccess := flag.Bool("exit-early", false, "--exit-early false")
	flag.BoolVar(exitOnSuccess, "e", false, "--exit-early false")
	timeout := flag.Int("time", 5, "--timeout 5")
	flag.IntVar(timeout, "t", 5, "-t 5")
	flag.Parse()

	glob.Uri = *uri
	glob.Method = *method
	glob.Port = *port
	glob.ExitEarly = *exitOnSuccess
	glob.Timeout = *timeout

	if glob.Uri == "" {
		os.Exit(1)
	}

	glob.Header = make(map[string]string)
	parseURI()

	var desyncr smuggler.DesyncerImpl
	desyncr.GetCookies(&glob)
	smuggler.PopPayload()
	desyncr.Start()
}

func parseURI() {
	if !strings.HasPrefix(glob.Uri, "http:") && !strings.HasPrefix(glob.Uri, "https:") {
		if glob.Port == 443 {
			glob.Uri = "https://" + glob.Uri
			glob.Scheme = "https"
		} else if glob.Port == 80 {
			glob.Uri = "http://" + glob.Uri
			glob.Scheme = "http"
		}
	}

	u, err := url.Parse(glob.Uri)
	if err != nil {
		log.Fatal(err)
	}

	glob.Url = u
	glob.Method = strings.ToUpper(glob.Method)
	if len(u.Path) == 0 {
		glob.Url.Path = "/"
	}
	if len(glob.Url.User.Username()) > 0 {
		glob.Header["Authorization"] = fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(glob.Url.User.String())))
	}
}
