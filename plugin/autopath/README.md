# autopath

*autopath* allows CoreDNS to perform server side search path completion.

If it sees a query that matches the first element of the configured search path, *autopath* will
follow the chain of search path elements and returns the first reply that is not NXDOMAIN. On any
failures the original reply is returned. Because *autopath* returns a reply for a name that wasn't
the original question it will add a CNAME that points from the original name (with the search path
element in it) to the name of this answer.

## Syntax

~~~
autopath [ZONE...] RESOLV-CONF
~~~

* **ZONES** zones *autopath* should be authoritative for.
* **RESOLV-CONF** points to a `resolv.conf` like file or uses a special syntax to point to another
  plugin. For instance `@kubernetes`, will call out to the kubernetes plugin (for each
  query) to retrieve the search list it should use.

Currently the following set of plugin has implemented *autopath*:

* *kubernetes*
* *erratic*

## Metrics
 
If monitoring is enabled (via the *prometheus* directive) then the following metric is exported:
 
* `coredns_autopath_success_count_total{}` - counter of successfully autopath-ed queries.

## Examples

~~~
autopath my-resolv.conf
~~~

Use `my-resolv.conf` as the file to get the search path from. This file only needs so have one line:
`search domain1 domain2 ...`

~~~
autopath @kubernetes
~~~

Use the search path dynamically retrieved from the kubernetes plugin.

## Bugs

Replies from this plugin are not cached, as the *cache* plugin is configured after this one (see
plugin.cfg).
