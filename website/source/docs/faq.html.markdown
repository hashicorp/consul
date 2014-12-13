---
layout: "docs"
page_title: "Frequently Asked Questions"
sidebar_current: "docs-faq"
---

# Frequently Asked Questions

## Q: Why is virtual memory usage high?

Consul makes use of [LMDB](http://symas.com/mdb/) internally for various data
storage purposes. LMDB relies on using memory-mapping, a technique in which
a sparse file is represented as a contiguous range of memory. Consul configures
high limits for these file sizes, and as a result relies on a large chunks of
virtual memory to be allocated. However, in practice the limits are much larger
than any realistic deployment of Consul would ever use, and the resident memory or
physical memory used is much lower.

## Q: What is Checkpoint? / Does Consul call home?

Consul makes use of a HashiCorp service called [Checkpoint](http://checkpoint.hashicorp.com)
which is used to check for updates and critical security bulletins.
Only anonymous information is sent to Checkpoint, and cannot be used to
identify the user or host. An anonymous ID is sent which helps de-duplicate
warning messages and can be disabled. Using the Checkpoint service is optional
and can be disabled.

See [`disable_anonymous_signature`](/docs/agent/options.html#disable_anonymous_signature)
and [`disable_update_check`](/docs/agent/options.html#disable_update_check).


