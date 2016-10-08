package test

const exampleOrg = `; example.org test file
example.org.		IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2015082541 7200 3600 1209600 3600
example.org.		IN	NS	b.iana-servers.net.
example.org.		IN	NS	a.iana-servers.net.
example.org.		IN	A	127.0.0.1
example.org.		IN	A	127.0.0.2
*.w.example.org.        IN      TXT     "Wildcard"
a.b.c.w.example.org.    IN      TXT     "Not a wildcard"
`
