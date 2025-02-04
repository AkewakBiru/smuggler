package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"smuggler/config"
	"smuggler/smuggler"
	"strings"
	"sync"
	"time"

	"github.com/panjf2000/ants"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	hosts    = flag.String("i", "", "file containing list of `URLs` to test")
	method   = flag.String("X", "POST", "`method` for sending a request")
	ttype    = flag.String("test", "basic", "`type` of test to run. options [basic, double, exhaustive]")
	destUrl  = flag.String("dest-url", "", "out-of-band `URL` for generating payload after a result is found")
	priority = flag.String("p", "CLTEH2", "`priority` indicating which test to run first when not using concurrency")
	timeout  = flag.Uint("T", 5, "per-request `timeout` in seconds to decide if there is a desync issue")
	poolSize = flag.Uint("t", 100, "number of threads `per-process`")
	eos      = flag.Bool("e", true, "`exit` on success")
	conc     = flag.Bool("c", false, "enable `per-URL` concurrency. Could show a lot of false positives")
	verbose  = flag.Bool("v", false, "show `verbose` output about the status of each test")
)

// per-host unique gadgets that must be sent for a request to work
type hostInfo struct {
	URL    string `json:"url"`
	Method string `json:"method"`
	Body   string `json:"body"`

	Hdrs map[string][]string `json:"headers"`
}

func init() {
	flag.Usage = func() {
		h := "Usage: smuggler [options]\nFlags:"
		fmt.Fprintln(os.Stderr, h)
		flag.PrintDefaults()
	}
}

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

func getInput(name string) *os.File {
	var file *os.File
	var err error
	if len(name) > 0 {
		file, err = os.OpenFile(name, os.O_RDONLY, 0664)
		if err == nil {
			return file
		}
	}
	log.Info().Err(err).Msg("falling back to STDIN")
	return os.Stdin
}

func chkStdIn() error {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return err
	}
	if stat.Mode()&os.ModeCharDevice == 0 { // checks if input is a coming from a file or pipe
		return nil
	}
	return errors.New("")
}

func main() {
	flag.Parse()

	fl := false
	for _, f := range []string{"basic", "double", "exhaustive"} {
		if f == *ttype {
			fl = true
			break
		}
	}
	if !fl {
		log.Warn().
			Msg("Invalid test type: Available options: [basic, double, exhaustive]")
		config.Glob.Test = config.B
	} else {
		setLevel(strings.ToUpper(*ttype))
	}
	if *verbose {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	config.Glob.ExitEarly = *eos
	config.Glob.Timeout = time.Duration(*timeout) * time.Second
	// config.Glob.Method = strings.ToUpper(strings.TrimSpace(*method))

	if *hosts == "" && chkStdIn() != nil {
		log.Fatal().
			Msg("File containing URLs must be present or a list of URLs must be passed from the stdin")
	}

	config.Glob.Hdr = make(map[string][]string)
	config.Glob.Hdr["User-Agent"] =
		[]string{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:132.0) Gecko/20100101 Firefox/132.0"}
	config.Glob.Hdr["Accept"] = []string{"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"}
	config.Glob.Hdr["Accept-Language"] = []string{"en-US,en;q=0.5"}
	config.Glob.Hdr["Accept-Encoding"] = []string{"gzip, deflate, br"}
	config.Glob.Hdr["Sec-Fetch-Dest"] = []string{"document"} // some requests with a browser User-Agent to website
	config.Glob.Hdr["Sec-Fetch-Mode"] = []string{"navigate"} // don't work, and these headers help with that
	config.Glob.Hdr["Sec-Fetch-Site"] = []string{"none"}
	config.Glob.Hdr["Sec-Fetch-User"] = []string{"?1"}

	config.Glob.DestURL, _ = url.Parse(*destUrl) // if nil, i will use the per-host URL
	config.Glob.Concurrent = *conc
	sl := []string{"CLTEH2", "CLH2TE", "TECLH2", "TEH2CL", "H2CLTE", "H2TECL"}
	if len(*priority) != 6 || !contains(sl, strings.ToUpper(*priority)) {
		log.Warn().
			Msg("Invalid priority: unknown priority sequence was used")
		*priority = "CLTEH2"
	}
	setPriority(strings.ToUpper(*priority))

	file := getInput(*hosts)
	defer file.Close()

	procInput(file)
}

func procInput(file *os.File) {
	config.Glob.Wg = sync.WaitGroup{}
	pool, err := ants.NewPool(int(*poolSize))
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	defer pool.Release()

	var decoder *json.Decoder
	if filepath.Ext(file.Name()) == ".json" {
		decoder = json.NewDecoder(file)
		if _, err := decoder.Token(); err != nil {
			log.Error().Err(err).Msg("error getting json decoder token")
			return
		}

		for decoder.More() {
			config.Glob.Wg.Add(1)
			var hinfo hostInfo
			if err := decoder.Decode(&hinfo); err == nil {
				scanHost(&hinfo)
			}
		}
		config.Glob.Wg.Wait()
		return
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		config.Glob.Wg.Add(1)
		host := scanner.Text()
		rec := hostInfo{
			URL:    host,
			Method: strings.ToUpper(strings.TrimSpace(*method)),
			Hdrs:   make(map[string][]string),
		}
		pool.Submit(func() {
			scanHost(&rec)
		})
	}
	config.Glob.Wg.Wait()
}

func scanHost(rec *hostInfo) {
	defer config.Glob.Wg.Done()
	var desyncr smuggler.DesyncerImpl
	desyncr.Hdr = rec.Hdrs
	desyncr.Method = rec.Method
	desyncr.Body = rec.Body

	if config.Glob.Concurrent {
		desyncr.Wg = sync.WaitGroup{}
		desyncr.Ctx, desyncr.Cancel = context.WithCancel(context.Background())
		desyncr.TestDone = make(chan struct{}, 1)
	}

	if err := desyncr.ParseURL(rec.URL); err != nil {
		log.Error().Err(err).Msg(rec.URL)
		return
	}

	if err := desyncr.GetCookie(); err != nil {
		log.Error().Err(err).Msg(desyncr.URL.Host)
		return
	}

	if len(desyncr.Hdr["Cookie"]) == 0 {
		orig := *desyncr.URL
		desyncr.URL.Path = "/" // check for cookies on URL root
		if err := desyncr.GetCookie(); err != nil {
			log.Error().Err(err).Msg(desyncr.URL.Host)
			return
		}
		desyncr.URL = &orig
	}
	desyncr.RunTests()
}

func contains(slice []string, pstr string) bool {
	for _, str := range slice {
		if pstr == str {
			return true
		}
	}
	return false
}

func setLevel(str string) {
	levelMap := map[string]config.LEVEL{
		"BASIC":      config.B,
		"DOUBLE":     config.M,
		"EXHAUSTIVE": config.E,
	}

	if val, ok := levelMap[str]; ok {
		config.Glob.Test = val
	}
}

func setPriority(str string) {
	priorityMap := map[string]config.Priority{
		"H2CLTE": config.H2CLTE,
		"H2TECL": config.H2TECL,
		"CLTEH2": config.CLTEH2,
		"CLH2TE": config.CLH2TE,
		"TECLH2": config.TECLH2,
		"TEH2CL": config.TEH2CL,
	}
	if val, ok := priorityMap[str]; ok {
		config.Glob.Priority = val
	}
}

// CL.0 -> Front-End takes all the content, but backend takes none (weird behaviour)
// before trying to test for anything, i need to make sure if the path
// returns a 200 OK and the given method works on the endpoint provided
