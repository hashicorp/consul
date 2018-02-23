# plz

PLZ has three parts

* plz: observability, test helper, message format, utility
* plz.sql: use sql to query anything
* plz.service: junction for distributed computing

Observability is the primary goal:

* countlog: log event, record how state change over the time
* dump: take snapshot of data, record the moment
* witch: a WEB UI to make log and snapshot visible