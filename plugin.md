# Plugins

## Writing Plugins

See a couple of blog posts on how to write and add plugin to CoreDNS:

* <https://blog.coredns.io/2017/03/01/how-to-add-plugin-to-coredns/>
* <https://blog.coredns.io/2016/12/19/writing-plugin-for-coredns/>, slightly older, but useful.

## Metrics

When exporting metrics the *Namespace* should be `plugin.Namespace` (="coredns"), and the
*Subsystem* should be the name of the plugin. The README.md for the plugin should then also contain
 a *Metrics* section detailing the metrics. If the plugin supports dynamic health reporting it
 should also have *Health* section detailing on some of its inner workings.

## Documentation

Each plugin should have a README.md explaining what the plugin does and how it is configured. The
file should have the following layout:

* Title: use the plugin's name
* Subsection titled: "Syntax"
* Subsection titled: "Examples"

More sections are of course possible.

### Style

We use the Unix manual page style:

* The name of plugin in the running text should be italic: *plugin*.
* all CAPITAL: user supplied argument, in the running text references this use strong text: `**`:
  **EXAMPLE**.
* Optional text: in block quotes: `[optional]`.
* Use three dots to indicate multiple options are allowed: `arg...`.
* Item used literal: `literal`.

### Example Domain Names

Please be sure to use `example.org` or `example.net` in any examples you provide. These are the
standard domain names created for this purpose.

## Fallthrough

In a perfect world the following would be true for plugin: "Either you are responsible for a zone or
not". If the answer is "not", the plugin should call the next plugin in the chain. If "yes" it
should handle *all* names that fall in this zone and the names below - i.e. it should handle the
entire domain.

~~~ txt
. {
    file example.org db.example
}
~~~

In this example the *file* plugin is handling all names below (and including) `example.org`. If
a query comes in that is not a subdomain (or equal to) `example.org` the next plugin is called.

Now, the world isn't perfect, and there are good reasons to "fallthrough" to the next middlware,
meaning a plugin is only responsible for a subset of names within the zone. The first of these
to appear was the *reverse* plugin that synthesis PTR and A/AAAA responses (useful with IPv6).

The nature of the *reverse* plugin is such that it only deals with A,AAAA and PTR and then only
for a subset of the names. Ideally you would want to layer *reverse* **in front off** another
plugin such as *file* or *auto* (or even *proxy*). This means *reverse* handles some special
reverse cases and **all other** request are handled by the backing plugin. This is exactly what
"fallthrough" does. To keep things explicit we've opted that plugins implement such behavior
should implement a `fallthrough` keyword.

## Qualifying for main repo

Plugins for CoreDNS can live out-of-tree, `plugin.cfg` defaults to CoreDNS' repo but other
repos work just as well. So when do we consider the inclusion of a new plugin in the main repo?

* First, the plugin should be useful for other people. "Useful" is a subjective term. We will
  probably need to further refine this.
* It should be sufficiently different from other plugin to warrant inclusion.
* Current internet standards need be supported: IPv4 and IPv6, so A and AAAA records should be
  handled (if your plugin is in the business of dealing with address records that is).
* It must have tests.
* It must have a README.md for documentation.
