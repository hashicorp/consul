## Contributing to CoreDNS

Welcome! Our community focuses on helping others and making CoreDNS the best it
can be. We gladly accept contributions and encourage you to get involved!

### Bug reports

First, please [search this repository](https://github.com/miekg/coredns/search?q=&type=Issues&utf8=%E2%9C%93)
with a variety of keywords to ensure your bug is not already reported.

If not, [open an issue](https://github.com/miekg/coredns/issues) and answer the
questions so we can understand and reproduce the problematic behavior.

The burden is on you to convince us that it is actually a bug in CoreDNS. This is
easiest to do when you write clear, concise instructions so we can reproduce
the behavior (even if it seems obvious). The more detailed and specific you are,
the faster we will be able to help you. Check out
[How to Report Bugs Effectively](http://www.chiark.greenend.org.uk/~sgtatham/bugs.html).

Please be kind. :smile: Remember that CoreDNS comes at no cost to you, and you're
getting free help.


### Minor improvements and new tests

Submit [pull requests](https://github.com/miekg/coredns/pulls) at any time. Make
sure to write tests to assert your change is working properly and is thoroughly
covered.


### Proposals, suggestions, ideas, new features

First, please [search](https://github.com/miekg/coredns/search?q=&type=Issues&utf8=%E2%9C%93)
with a variety of keywords to ensure your suggestion/proposal is new.

If so, you may open either an issue or a pull request for discussion and
feedback.

The advantage of issues is that you don't have to spend time actually
implementing your idea, but you should still describe it thoroughly. The
advantage of a pull request is that we can immediately see the impact the change
will have on the project, what the code will look like, and how to improve it.
The disadvantage of pull requests is that they are unlikely to get accepted
without significant changes, or it may be rejected entirely. Don't worry, that
won't happen without an open discussion first.

If you are going to spend significant time implementing code for a pull request,
best to open an issue first and "claim" it and get feedback before you invest
a lot of time.


### Vulnerabilities

If you've found a vulnerability that is serious, please email me: <miek@miek.nl>.
If it's not a big deal, a pull request will probably be faster.

## Thank you

Thanks for your help! CoreDNS would not be what it is today without your contributions.

## Git Hook

We use `golint` and `go vet` as tools to warn use about things (noted golint is obnoxious sometimes,
but still helpful). Add the following script as a git `post-commit` in `.git/hooks/post-commit` and
make it executable.

~~~ sh
#!/bin/bash

# <https://git-scm.com/docs/githooks>:
# The script takes no parameters and its exit status does not affect the commit in any way.  You can
# use git # rev-parse HEAD to get the new commit’s SHA1 hash, or you can use git log -l HEAD to get
# all of its # information.

for d in *; do
    if [[ "$d" == "vendor" ]]; then
        continue
    fi
    if [[ "$d" == "logo" ]]; then
        continue
    fi
    if [[ ! -d "$d" ]]; then
        continue
    fi
    golint "$d"/...
done
~~~
