# bexpr - Boolean Expression Evaluator [![GoDoc](https://godoc.org/github.com/hashicorp/go-bexpr?status.svg)](https://godoc.org/github.com/hashicorp/go-bexpr) [![CircleCI](https://circleci.com/gh/hashicorp/go-bexpr.svg?style=svg)](https://circleci.com/gh/hashicorp/go-bexpr)

`bexpr` is a Go (golang) library to provide generic boolean expression evaluation and filtering for Go data structures.

## Limitations

Currently `bexpr` does not support operating on types with cyclical structures. Attempting to generate the fields
of these types will cause a stack overflow. There are however two means of getting around this. First if you do not
need the nested type to be available during evaluation then you can simply add the  `bexpr:"-"` struct tag to the
fields where that type is referenced and `bexpr` will not delve further into that type. A second solution is implement
the `MatchExpressionEvaluator` interface and provide the necessary field configurations yourself.

Eventually this lib will support handling these cycles automatically.

## Stability

Currently there is a `MatchExpressionEvaluator` interface that can be used to implement custom behavior. This interface should be considered *experimental* and is likely to change in the future. One need for the change is to make it easier for custom implementations to re-invoke the main bexpr logic on subfields so that they do not have to implement custom logic for themselves and every sub field they contain. With the current interface its not really possible.

## Usage (Reflection)

This example program is available in [examples/simple](examples/simple)

```go
package main

import (
   "fmt"
   "github.com/hashicorp/go-bexpr"
)

type Example struct {
   X int

   // Can renamed a field with the struct tag
   Y string `bexpr:"y"`

   // Fields can use multiple names for accessing
   Z bool `bexpr:"Z,z,foo"`

   // Tag with "-" to prevent allowing this field from being used
   Hidden string `bexpr:"-"`

   // Unexported fields are not available for evaluation
   unexported string
}

func main() {
   value := map[string]Example{
      "foo": Example{X: 5, Y: "foo", Z: true, Hidden: "yes", unexported: "no"},
      "bar": Example{X: 42, Y: "bar", Z: false, Hidden: "no", unexported: "yes"},
   }

   expressions := []string{
      "foo.X == 5",
      "bar.y == bar",
      "foo.foo != false",
      "foo.z == true",
      "foo.Z == true",

      // will error in evaluator creation
      "bar.Hidden != yes",

      // will error in evaluator creation
      "foo.unexported == no",
   }

   for _, expression := range expressions {
      eval, err := bexpr.CreateEvaluatorForType(expression, nil, (*map[string]Example)(nil))

      if err != nil {
         fmt.Printf("Failed to create evaluator for expression %q: %v\n", expression, err)
         continue
      }

      result, err := eval.Evaluate(value)
      if err != nil {
         fmt.Printf("Failed to run evaluation of expression %q: %v\n", expression, err)
         continue
      }

      fmt.Printf("Result of expression %q evaluation: %t\n", expression, result)
   }
}
```

This will output:

```
Result of expression "foo.X == 5" evaluation: true
Result of expression "bar.y == bar" evaluation: true
Result of expression "foo.foo != false" evaluation: true
Result of expression "foo.z == true" evaluation: true
Result of expression "foo.Z == true" evaluation: true
Failed to create evaluator for expression "bar.Hidden != yes": Selector "bar.Hidden" is not valid
Failed to create evaluator for expression "foo.unexported == no": Selector "foo.unexported" is not valid
```

## Testing

The [Makefile](Makefile) contains 3 main targets to aid with testing:

1. `make test` - runs the standard test suite
2. `make coverage` - runs the test suite gathering coverage information
3. `make bench` - this will run benchmarks. You can use the [`benchcmp`](https://godoc.org/golang.org/x/tools/cmd/benchcmp) tool to compare
   subsequent runs of the tool to compare performance. There are a few arguments you can
   provide to the make invocation to alter the behavior a bit
   * `BENCHFULL=1` - This will enable running all the benchmarks. Some could be fairly redundant but
     could be useful when modifying specific sections of the code.
   * `BENCHTIME=5s` - By default the -benchtime paramater used for the `go test` invocation is `2s`.
     `1s` seemed like too little to get results consistent enough for comparison between two runs.
     For the highest degree of confidence that performance has remained steady increase this value
     even further. The time it takes to run the bench testing suite grows linearly with this value.
   * `BENCHTESTS=BenchmarkEvalute` - This is used to run a particular benchmark including all of its
     sub-benchmarks. This is just an example and "BenchmarkEvaluate" can be replaced with any
     benchmark functions name.
