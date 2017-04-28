[![godoc](https://godoc.org/github.com/pierrec/xxHash?status.png)](https://godoc.org/github.com/pierrec/xxHash)
[![Build Status](https://travis-ci.org/pierrec/xxHash.svg?branch=master)](https://travis-ci.org/pierrec/xxHash)

# Pure Go implementation of xxHash (32 and 64 bits versions)

## Synopsis

xxHash is a very fast hashing algorithm (see the details [here](https://github.com/Cyan4973/xxHash/)).
This package implements xxHash in pure [Go](http://www.golang.com).


## Usage

This package follows the hash interfaces (hash.Hash32 and hash.Hash64).

```go
	import (
		"fmt"

		"github.com/pierrec/xxHash/xxHash32"
	)

 	x := xxHash32.New(0xCAFE) // hash.Hash32
	x.Write([]byte("abc"))
	x.Write([]byte("def"))
	fmt.Printf("%x\n", x.Sum32())

	x.Reset()
	x.Write([]byte("abc"))
	fmt.Printf("%x\n", x.Sum32())
```

## Command line utility

A simple command line utility is provided to hash files content under the xxhsum directory.

