# Go monkeypatching :monkey_face: :monkey:

Actual arbitrary monkeypatching for Go. Yes really.

Read this blogpost for an explanation on how it works: http://bouk.co/blog/monkey-patching-in-go/

## I thought that monkeypatching in Go is impossible?

It's not possible through regular language constructs, but we can always bend computers to our will! Monkey implements monkeypatching by rewriting the running executable at runtime and inserting a jump to the function you want called instead. **This is as unsafe as it sounds and I don't recommend anyone do it outside of a testing environment.**

Make sure you read the notes at the bottom of the README if you intend to use this library.

## Using monkey

Monkey's API is very simple and straightfoward. Call `monkey.Patch(<target function>, <replacement function>)` to replace a function. For example:

```go
package main

import (
	"fmt"
	"os"
	"strings"

	"bou.ke/monkey"
)

func main() {
	monkey.Patch(fmt.Println, func(a ...interface{}) (n int, err error) {
		s := make([]interface{}, len(a))
		for i, v := range a {
			s[i] = strings.Replace(fmt.Sprint(v), "hell", "*bleep*", -1)
		}
		return fmt.Fprintln(os.Stdout, s...)
	})
	fmt.Println("what the hell?") // what the *bleep*?
}
```

You can then call `monkey.Unpatch(<target function>)` to unpatch the method again. The replacement function can be any function value, whether it's anonymous, bound or otherwise.

If you want to patch an instance method you need to use `monkey.PatchInstanceMethod(<type>, <name>, <replacement>)`. You get the type by using `reflect.TypeOf`, and your replacement function simply takes the instance as the first argument. To disable all network connections, you can do as follows for example:

```go
package main

import (
	"fmt"
	"net"
	"net/http"
	"reflect"

	"bou.ke/monkey"
)

func main() {
	var d *net.Dialer // Has to be a pointer to because `Dial` has a pointer receiver
	monkey.PatchInstanceMethod(reflect.TypeOf(d), "Dial", func(_ *net.Dialer, _, _ string) (net.Conn, error) {
		return nil, fmt.Errorf("no dialing allowed")
	})
	_, err := http.Get("http://google.com")
	fmt.Println(err) // Get http://google.com: no dialing allowed
}

```

Note that patching the method for just one instance is currently not possible, `PatchInstanceMethod` will patch it for all instances. Don't bother trying `monkey.Patch(instance.Method, replacement)`, it won't work. `monkey.UnpatchInstanceMethod(<type>, <name>)` will undo `PatchInstanceMethod`.

If you want to remove all currently applied monkeypatches simply call `monkey.UnpatchAll`. This could be useful in a test teardown function.

If you want to call the original function from within the replacement you need to use a `monkey.PatchGuard`. A patchguard allows you to easily remove and restore the patch so you can call the original function. For example:

```go
package main

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"bou.ke/monkey"
)

func main() {
	var guard *monkey.PatchGuard
	guard = monkey.PatchInstanceMethod(reflect.TypeOf(http.DefaultClient), "Get", func(c *http.Client, url string) (*http.Response, error) {
		guard.Unpatch()
		defer guard.Restore()

		if !strings.HasPrefix(url, "https://") {
			return nil, fmt.Errorf("only https requests allowed")
		}

		return c.Get(url)
	})

	_, err := http.Get("http://google.com")
	fmt.Println(err) // only https requests allowed
	resp, err := http.Get("https://google.com")
	fmt.Println(resp.Status, err) // 200 OK <nil>
}
```

## Notes

1. Monkey sometimes fails to patch a function if inlining is enabled. Try running your tests with inlining disabled, for example: `go test -gcflags=-l`. The same command line argument can also be used for build.
2. Monkey won't work on some security-oriented operating system that don't allow memory pages to be both write and execute at the same time. With the current approach there's not really a reliable fix for this.
3. Monkey is not threadsafe. Or any kind of safe.
4. I've tested monkey on OSX 10.10.2 and Ubuntu 14.04. It should work on any unix-based x86 or x86-64 system.

© Bouke van der Bijl
