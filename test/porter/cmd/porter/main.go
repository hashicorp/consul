package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"

	"github.com/hashicorp/consul/test/porter"
)

var addrFile = "/tmp/porter.addr"

var (
	addr      string
	firstPort int
	lastPort  int
	verbose   bool

	mu   sync.Mutex
	port int
)

func main() {
	log.SetFlags(0)

	flag.StringVar(&addr, "addr", porter.DefaultAddr, "host:port")
	flag.IntVar(&firstPort, "first-port", 10000, "first port to allocate")
	flag.IntVar(&lastPort, "last-port", 20000, "last port to allocate")
	flag.BoolVar(&verbose, "verbose", false, "log port allocations")
	flag.Parse()

	// check if there is an instance running
	b, err := ioutil.ReadFile(addrFile)
	switch {
	// existing instance but no command to run
	case err == nil && len(flag.Args()) == 0:
		log.Println("porter already running on", string(b))
		os.Exit(0)

	// existing instance with command to run
	case err == nil:
		addr = string(b)
		log.Println("re-using porter instance on", addr)

	// new instance
	case os.IsNotExist(err):
		if err := ioutil.WriteFile(addrFile, []byte(addr), 0644); err != nil {
			log.Fatalf("Cannot write %s: %s", addrFile, err)
		}
		defer os.Remove(addrFile)
		go func() {
			http.HandleFunc("/", servePort)
			if err := http.ListenAndServe(addr, nil); err != nil {
				log.Fatal(err)
			}
		}()
	}

	args := flag.Args()

	// no command to run: wait for CTRL-C
	if len(args) == 0 {
		log.Print("PORTER_ADDR=" + addr)
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		s := <-c
		log.Println("Got signal:", s)
		return
	}

	// run command and exit with 1 in case of error
	if err := run(args); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	path, err := exec.LookPath(args[0])
	if err != nil {
		return fmt.Errorf("Cannot find %q in path", args[0])
	}

	cmd := exec.Command(path, args[1:]...)
	if os.Getenv("PORTER_ADDR") == "" {
		cmd.Env = append(os.Environ(), "PORTER_ADDR="+addr)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// todo(fs): check which ports are currently bound and exclude them
func servePort(w http.ResponseWriter, r *http.Request) {
	var count int
	n, err := strconv.Atoi(r.RequestURI[1:])
	if err == nil {
		count = n
	}
	if count <= 0 {
		count = 1
	}

	mu.Lock()
	if port < firstPort {
		port = firstPort
	}
	if port+count >= lastPort {
		port = firstPort
	}
	from, to := port, port+count
	port = to
	mu.Unlock()

	p := make([]int, count)
	for i := 0; i < count; i++ {
		p[i] = from + i
	}
	if err := json.NewEncoder(w).Encode(p); err != nil {
		// this shouldn't happen so we panic since we can't recover
		panic(err)
	}
	if verbose {
		log.Printf("porter: allocated ports %d-%d (%d)", from, to, count)
	}
}
