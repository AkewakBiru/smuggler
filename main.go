package main

import (
	"encoding/base64"
	"errors"
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
		h += "-f, --test type of test (basic, double, exhaustive)\n"
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
	port := flag.String("port", "", "--port 443")
	flag.StringVar(port, "p", "", "-p 443")
	exitOnSuccess := flag.Bool("exit-early", false, "--exit-early false")
	flag.BoolVar(exitOnSuccess, "e", false, "--exit-early false")
	timeout := flag.Int("time", 5, "--timeout 5")
	flag.IntVar(timeout, "t", 5, "-t 5")
	file := flag.String("test", "basic", "--test \"basic\"")
	flag.StringVar(file, "f", "basic", "-f \"basic\"")
	flag.Parse()

	fl := false
	for _, f := range []string{"basic", "double", "exhaustive"} {
		if f == *file {
			fl = true
			break
		}
	}
	if !fl {
		log.Fatal("Invalid test type: Available options: [basic, double, exhaustive]")
	}
	if *uri == "" {
		log.Fatal("Invalid URI: Empty URI")
	}

	if err := parseURI(*uri); err != nil {
		log.Fatal(err)
	}
	glob.Method = strings.ToUpper(strings.TrimSpace(*method))
	// given port overrides scheme port
	if *port != "" {
		glob.URL.Host = strings.Split(glob.URL.Host, ":")[0] + ":" + *port
	}
	glob.ExitEarly = *exitOnSuccess
	glob.Timeout = *timeout
	glob.File = *file

	glob.Header = make(map[string]string)

	var desyncr smuggler.DesyncerImpl
	desyncr.GetCookie(&glob)
	if err := desyncr.Start(); err != nil {
		log.Fatalln(err)
	}
}

func parseURI(uri string) error {
	u, err := url.Parse(uri)
	if err != nil {
		return err
	}
	glob.URL = u
	if glob.URL.Scheme == "" && glob.URL.Port() == "" {
		return errors.New("invalid URL: Empty Scheme & Port")
	}
	if glob.URL.Port() == "" {
		if glob.URL.Scheme == "http" {
			glob.URL.Host = glob.URL.Host + ":80"
		} else if glob.URL.Scheme == "https" {
			glob.URL.Host = glob.URL.Host + ":443"
		}
	}
	fmt.Println(glob.URL.Host)
	fmt.Println(glob.URL.Scheme)

	if glob.URL.Path == "/" {
		glob.URL.Path = "/"
	}

	if len(glob.URL.User.Username()) > 0 {
		glob.Header["Authorization"] = fmt.Sprintf("Basic %s",
			base64.StdEncoding.EncodeToString([]byte(glob.URL.User.String())))
	}
	return nil
}
