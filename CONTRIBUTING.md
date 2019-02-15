# Contributing to CoreDNS

Welcome! Our community focuses on helping others and making CoreDNS the best it can be. We gladly
accept contributions and encourage you to get involved!

## Bug Reports

First, please [search this
repository](https://github.com/coredns/coredns/search?q=&type=Issues&utf8=%E2%9C%93) with a variety
of keywords to ensure your bug is not already reported.

If not, [open an issue](https://github.com/coredns/coredns/issues) and answer the questions so we
can understand and reproduce the problematic behavior.

The burden is on you to convince us that it is actually a bug in CoreDNS. This is easiest to do when
you write clear, concise instructions so we can reproduce the behavior (even if it seems obvious).
The more detailed and specific you are, the faster we will be able to help you. Check out [How to
Report Bugs Effectively](https://www.chiark.greenend.org.uk/~sgtatham/bugs.html).

Please be kind. :smile: Remember that CoreDNS comes at no cost to you, and you're getting free help.

## Minor Improvements and New Tests

Submit [pull requests](https://github.com/coredns/coredns/pulls) at any time. Make sure to write
tests to assert your change is working properly and is thoroughly covered.

## New Features

First, please [search](https://github.com/coredns/coredns/search?q=&type=Issues&utf8=%E2%9C%93) with
a variety of keywords to ensure your suggestion/proposal is new.

If so, you may open either an issue or a pull request for discussion and feedback.

If you are going to spend significant time implementing code for a pull request, best to open an
issue first and "claim" it and get feedback before you invest a lot of time.

If possible make a pull request as small as possible, or submit multiple pull request to complete a
feature. Smaller means: easier to understand and review. This in turn means things can be merged
faster.

## Updating Dependencies

We use Golang's [`dep`](https://github.com/golang/dep) as the tool to manage vendor dependencies.
The tool could be obtained through:

```sh
$ go get -u github.com/golang/dep/cmd/dep
```

Use the following to update the locked versions of all dependencies
```sh
$ make dep-ensure
```

After the dependencies have been updated or added, you might run the following to
prune vendored packages:
```sh
$ dep prune
```

Please refer to Golang's [`dep`](https://github.com/golang/dep) for more details.

# Thank You

Thanks for your help! CoreDNS would not be what it is today without your contributions.
