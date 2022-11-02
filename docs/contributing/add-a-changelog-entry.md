# Adding a Changelog Entry

Any change that a Consul user might need to know about should have a changelog entry.

What doesn't need a changelog entry?
- Docs changes
- Typos fixes, unless they are in a public-facing API
- Code changes we are certain no Consul users will need to know about

To include a [changelog entry](../.changelog) in a PR, commit a text file
named `.changelog/<PR#>.txt`, where `<PR#>` is the number associated with the open
PR in Github. The text file should describe the changes in the following format:

````
```release-note:<change type>
<code area>: <brief description of the improvement you made here>
```
````

Valid values for `<change type>` include:
- `feature`: for the addition of a new feature
- `improvement`: for an improvement (not a bug fix) to an existing feature
- `bug`: for a bug fix
- `security`: for any Common Vulnerabilities and Exposures (CVE) resolutions
- `breaking-change`: for any change that is not fully backwards-compatible
- `deprecation`: for functionality which is now marked for removal in a future release

`<code area>` is meant to categorize the functionality affected by the change.
Some common values are:
- `checks`: related to node or service health checks
- `cli`: related to the command-line interface and its commands
- `config`: related to configuration changes (e.g., adding a new config option)
- `connect`: catch-all for the Connect subsystem that provides service mesh functionality
  if no more specific `<code area>` applies
- `http`: related to the HTTP API interface and its endpoints
- `dns`: related to DNS functionality
- `ui`: any change related to the built-in Consul UI (`website/` folder)

Look in the [`.changelog/`](../.changelog) folder for examples of existing changelog entries.

If a PR deserves multiple changelog entries, just add multiple entries separated by a newline
in the format described above to the `.changelog/<PR#>.txt` file.
