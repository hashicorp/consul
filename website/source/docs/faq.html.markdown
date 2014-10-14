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

