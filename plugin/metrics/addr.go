package metrics

// addrs keeps track on which addrs we listen, so we only start one listener, is
// prometheus is used in multiple Server Blocks.
type addrs struct {
	a map[string]value
}

type value struct {
	state int
	f     func() error
}

var uniqAddr addrs

func newAddress() addrs {
	return addrs{a: make(map[string]value)}
}

func (a addrs) setAddress(addr string, f func() error) {
	if a.a[addr].state == done {
		return
	}
	a.a[addr] = value{todo, f}
}

// setAddressTodo sets addr to 'todo' again.
func (a addrs) setAddressTodo(addr string) {
	v, ok := a.a[addr]
	if !ok {
		return
	}
	v.state = todo
	a.a[addr] = v
}

// forEachTodo iterates for a and executes f for each element that is 'todo' and sets it to 'done'.
func (a addrs) forEachTodo() error {
	for k, v := range a.a {
		if v.state == todo {
			v.f()
		}
		v.state = done
		a.a[k] = v
	}
	return nil
}

const (
	todo = 1
	done = 2
)
