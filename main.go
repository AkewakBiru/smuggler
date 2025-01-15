package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"smuggler/config"
	"smuggler/smuggler"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func init() {
	flag.Usage = func() {
		h := "\nHTTP Request Smuggling tester\n"
		h += "Usage: "
		h += "smuggler [Options]\n\n"
		h += "-i, --input-file file containing a list of URLs, this can also be passed as a STDIN to the program\n"
		h += "-s, --scheme scheme for the url (use http|https)\n"
		h += "-T, --timeout timeout for the request\n"
		h += "-t, --threads number of threads\n" // TODO: implement a thread-pool: Currently N-Goroutines Per N-hosts
		h += "-f, --test type of test (basic, double, exhaustive)\n"
		h += "-e, --exit-early exit as soon as a Desync is detected\n"
		h += "-v, --verbose shows every detail of what is happening"
		fmt.Fprintln(os.Stderr, h)
	}
}

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

func getInput(name string) (*os.File, error) {
	var file *os.File
	var err error
	file, err = os.OpenFile(name, os.O_RDONLY, 0664)
	if err != nil {
		log.Print(err)
		return os.Stdin, nil
	}
	return file, nil
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

// TODO: add verbosity (print request/response headers to stdout)
//
// default to POST method if method isn't provided
// default read timeout is set to 5 seconds
func main() {
	hosts := flag.String("input-file", "", "--input-file test_file.txt")
	flag.StringVar(hosts, "i", "", "-i test_file.txt")
	method := flag.String("method", "POST", "--method POST")
	flag.StringVar(method, "X", "POST", "-X POST")
	eos := flag.Bool("exit-early", true, "--exit-early false") //exit on success
	flag.BoolVar(eos, "e", true, "-e false")
	timeout := flag.Int("time", 5, "--timeout 5")
	flag.IntVar(timeout, "T", 5, "-T 5")
	ttype := flag.String("test", "basic", "--test basic")
	flag.StringVar(ttype, "f", "basic", "-f basic")
	verbose := flag.Bool("verbose", false, "--verbose")
	flag.BoolVar(verbose, "v", false, "--verbose")
	flag.Parse()

	fl := false
	for _, f := range []string{"basic", "double", "exhaustive"} {
		if f == *ttype {
			fl = true
			break
		}
	}
	if !fl {
		log.Fatal().Msg("Invalid test type: Available options: [basic, double, exhaustive]")
	}
	if *verbose {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	config.Glob.ExitEarly = *eos
	config.Glob.Timeout = time.Duration(*timeout) * time.Second
	config.Glob.Test = *ttype
	config.Glob.Method = strings.ToUpper(strings.TrimSpace(*method))

	if *hosts == "" && chkStdIn() != nil {
		log.Fatal().Msg("File containing URLs must be present or a list of URLs must be passed from the stdin")
	}

	file, err := getInput(*hosts)
	if err != nil {
		log.Fatal().Msgf("%v", err)
	}

	scanner := bufio.NewScanner(file)
	config.Glob.Wg = sync.WaitGroup{}
	for scanner.Scan() {
		config.Glob.Wg.Add(1)
		go scanHost(scanner.Text())
	}
	config.Glob.Wg.Wait()
}

func scanHost(host string) {
	defer config.Glob.Wg.Done()
	var desyncr smuggler.DesyncerImpl
	desyncr.Hdr = make(map[string]string)
	// desyncr.Dict = zerolog.Dict()

	if err := desyncr.ParseURL(host); err != nil {
		log.Error().Err(err).Msg(host)
		return
	}
	if err := desyncr.GetCookie(); err != nil {
		log.Error().Err(err).Msg(desyncr.URL.Host)
		return
	}
	if err := desyncr.Start(); err != nil {
		log.Error().Err(err).Msg(desyncr.URL.Host)
		return
	}
}

// CL.0 -> Front-End takes all the content, but backend takes none (weird behaviour)
// before trying to test for anything, i need to make sure if the path
// returns a 200 OK and the given method works on the endpoint provided
