# repro-constrained-travis
Resource constrained env based on Travis CI for surfacing flakiness in tests.

## Usage:
Build binary, then run wth `./flake-repro [options]`

Options:
```
  -pkg=""             Target package
  -test=""            Target test (requires pkg flag)
  -cpus=0.15          Amount of CPU resources for container
  -n=30               Number of times to run tests
```


## Examples:
Run all tests for a package:
```
./flake-repro.sh -pkg connect/proxy
```

Run tests matching a pattern:
```
./flake-repro.sh -pkg connect/proxy -test Listener
```

Run a single test:
```
./flake-repro.sh -pkg connect/proxy -test TestUpstreamListener
```

Run a single test 100 times:
```
./flake-repro.sh -pkg connect/proxy -test TestUpstreamListener -n 100
```