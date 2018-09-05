package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

const (
	image      = "travis-img-v0.13"
	container  = "travis-cnt"
	goos       = "linux"
	goarch     = "amd64"
	testBinary = "flake.test"
)

func main() {
	var pkg string
	var test string
	var cpus float64
	var iterations int

	flags := flag.NewFlagSet("repro", flag.ExitOnError)
	flags.Usage = func() { printUsage() }

	flags.Float64Var(&cpus, "cpus", 0.15, "amount of cpus to use")
	flags.IntVar(&iterations, "n", 30, "number of times to run tests")
	flags.StringVar(&pkg, "pkg", "", "package to test")
	flags.StringVar(&test, "test", "", "test to run")

	if err := flags.Parse(os.Args[1:]); err != nil {
		flags.Usage()
		os.Exit(1)
	}

	if pkg == "" || len(strings.Split(pkg, " ")) > 1 {
		log.Fatal("error: one pkg is required")
	}

	// a[n-2] because not running from base of consul repo
	appCmd := "echo $(pwd | awk '{n=split($0, a, \"/\"); print a[n-2]}')"
	bytes, err := exec.Command("sh", "-c", appCmd).Output()
	if err != nil {
		log.Fatalf("failed to get app name from cwd")
	}
	app := strings.TrimSpace(string(bytes))

	fmt.Printf("App:\t\t%s\n", app)
	fmt.Printf("Package:\t%s\n", pkg)
	fmt.Printf("Test:\t\t%s\n", test)
	fmt.Printf("CPUs:\t\t%f\n", cpus)
	fmt.Printf("Iterations:\t%d\n", iterations)

	fmt.Println("----> Cleaning up old containers...")
	rmContainerCmd := fmt.Sprintf(
		`if docker ps -a | grep %s ; then
			docker rm -f $(docker ps -a | grep %s | awk '{print $1;}')
		fi`, container, container)

	out, err := exec.Command("sh", "-c", rmContainerCmd).CombinedOutput()
	if err != nil {
		log.Println(string(out))
		log.Fatalf("failed to remove existing container '%s': %v", container, err)
	}

	fmt.Println("---> Rebuilding image...")
	buildImageCmd := fmt.Sprintf("docker build -q -t %s --no-cache .", image)

	out, err = exec.Command("sh", "-c", buildImageCmd).CombinedOutput()
	if err != nil {
		log.Println(string(out))
		log.Fatalf("failed to build image '%s': %v", image, err)
	}

	fmt.Println("--> Building app binary...")
	installCmd := fmt.Sprintf("(cd ../../ && env GOOS=%s GOARCH=%s go build -o bin/%s)", goos, goarch, app)

	out, err = exec.Command("sh", "-c", installCmd).CombinedOutput()
	if err != nil {
		log.Println(string(out))
		log.Fatalf("failed to build app binary: %v", err)
	}

	compileCmd := fmt.Sprintf("(cd ../../ && env GOOS=%s GOARCH=%s go test -c \"./%s\" -o %s)", goos, goarch, pkg, testBinary)

	out, err = exec.Command("sh", "-c", compileCmd).CombinedOutput()
	if err != nil {
		log.Println(string(out))
		log.Fatalf("failed to compile test binary: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to get cwd: %v", err)
	}
	baseDir := strings.SplitAfter(wd, app)[0]

	fmt.Println("-> Running container...")
	runCmd := fmt.Sprintf(`docker run \
		--name %s \
		--cpus="%f" \
		-v %s:/home/travis/go/%s \
		-e TEST_BINARY="%s" \
		-e TEST_PKG="%s" \
		-e TEST="%s" \
		-e ITERATIONS="%d" \
		-e APP="%s" \
		%s`,
		container, cpus, baseDir, app, testBinary, pkg, test, iterations, app, image)

	cmd := exec.Command("sh", "-c", runCmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatalf("failed to run: %v", err)
	}
	// Scan from stdout in real time
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("failed to scan stdout: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		log.Fatalf("failed to wait for cmd: %v", err)
	}
	if err != nil {
		log.Fatalf("failed to run container: %v", err)
	}

	// remove linux/amd64 binary
	os.Remove(fmt.Sprintf("bin/%s", app))

	cleanCmd := fmt.Sprintf("docker rm -f %s", container)

	out, err = exec.Command("sh", "-c", cleanCmd).CombinedOutput()
	if err != nil {
		log.Println(string(out))
		log.Fatalf("failed to clean up container: %v", err)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, helpText)
}

const helpText = `Usage: flake-repro [options]

  flake-repro surfaces flakiness in tests by constraining CPU resources.
  
  Single or package-wide tests are run for multiple iterations with a configurable
  amount of CPU resources. 

  0.15 CPUs and 30 iterations are configured as sane defaults.

  See Docker docs for more info on tuning 'cpus' param: 
  https://docs.docker.com/config/containers/resource_constraints/#cpu

Options:

  -pkg=""             Target package
  -test=""            Target test (requires pkg flag)
  -cpus=0.15          Amount of CPU resources for container
  -n=30               Number of times to run tests

Examples:

  ./flake-repro.sh -pkg connect/proxy
  ./flake-repro.sh -pkg connect/proxy -cpus 0.20
  ./flake-repro.sh -pkg connect/proxy -test Listener
  ./flake-repro.sh -pkg connect/proxy -test TestUpstreamListener
  ./flake-repro.sh -pkg connect/proxy -test TestUpstreamListener -n 100
`
