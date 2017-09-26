package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
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
	startServer := true
	b, err := ioutil.ReadFile(addrFile)
	if err == nil {
		addr = string(b)
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			log.Println("found running porter instance at", addr)
			startServer = false
			conn.Close()
		} else {
			log.Printf("found dead porter instance at %s, will take over", addr)
		}
	}

	args := flag.Args()
	if startServer {
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
	} else {
		if len(args) == 0 {
			log.Println("no command and existing porter instance found, exiting")
			os.Exit(0)
		}
	}

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

func servePort(w http.ResponseWriter, r *http.Request) {
	var count int
	n, err := strconv.Atoi(r.RequestURI[1:])
	if err == nil {
		count = n
	}
	if count <= 0 {
		count = 1
	}

	// getPort assumes the lock is already held and tries to return a port
	// that's not in use. It will panic if it has to try too many times.
	getPort := func() int {
		for i := 0; i < 10; i++ {
			port++
			if port < firstPort {
				port = firstPort
			}
			if port >= lastPort {
				port = firstPort
			}

			conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
			if err != nil {
				return port
			}
			conn.Close()
			if verbose {
				log.Printf("porter: skipping port %d, already in use", port)
			}
		}
		panic(fmt.Errorf("could not find a free port"))
	}

	p := make([]int, count)
	mu.Lock()
	for i := 0; i < count; i++ {
		p[i] = getPort()
	}
	mu.Unlock()

	if err := json.NewEncoder(w).Encode(p); err != nil {
		// this shouldn't happen so we panic since we can't recover
		panic(err)
	}
	if verbose {
		log.Printf("porter: allocated ports %v", p)
	}
}
