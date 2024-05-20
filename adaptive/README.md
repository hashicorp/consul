go-immutable-adaptive-radix [![Run CI Tests](https://github.com/hashicorp/go-immutable-adaptive-radix/actions/workflows/ci.yaml/badge.svg)](https://github.com/hashicorp/go-immutable-adaptive-radix/actions/workflows/ci.yaml)
=========

Provides the `adaptive` package that implements an immutable adaptive [radix tree](http://en.wikipedia.org/wiki/Radix_tree).
The package only provides a single `RadixTree` implementation, optimized for sparse nodes.

As a radix tree, it provides the following:
* O(k) operations. In many cases, this can be faster than a hash table since
  the hash function is an O(k) operation, and hash tables have very poor cache locality.
* Minimum / Maximum value lookups
* Ordered iteration

A tree supports using a transaction to batch multiple updates (insert, delete)
in a more efficient manner than performing each operation one at a time.

Documentation
=============

Example
=======

Below is a simple example of usage

```go
// Create a tree
r := adaptive.NewRadixTree[int]()
r, _, _ = r.Insert([]byte("foo"), 1)
r, _, _ = r.Insert([]byte("bar"), 2)
r, _, _ = r.Insert([]byte("foobar"), 2)

// Find the longest prefix match
m, _, _ := r.LongestPrefix([]byte("foozip"))
if string(m) != "foo" {
    panic("should be foo")
}

```

Here is an example of performing a range scan of the keys.

```go
// Create a tree
r := adaptive.NewRadixTree[int]()
r, _, _ = r.Insert([]byte("001"), 1)
r, _, _ = r.Insert([]byte("002"), 2)
r, _, _ = r.Insert([]byte("005"), 5)
r, _, _ = r.Insert([]byte("010"), 10)
r, _, _ = r.Insert([]byte("100"), 10)
```
