// discover provides node discovery on the command line.
package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	discover "github.com/hashicorp/go-discover"
)

func main() {
	help := flag.Bool("h", false, "show help")
	quiet := flag.Bool("q", false, "no output")
	flag.Parse()

	args := strings.Join(flag.Args(), " ")
	if *help || args == "" {
		fmt.Println("Usage: discover key=val key=val ...")
		os.Exit(0)
	}

	var w io.Writer = os.Stderr
	if *quiet {
		w = ioutil.Discard
	}
	l := log.New(w, "", log.LstdFlags)

	addrs, err := discover.Discover(args, l)
	if err != nil {
		l.Fatal("[ERR] ", err)
	}
	fmt.Println(strings.Join(addrs, " "))
}
