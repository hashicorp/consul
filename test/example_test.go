package test

const exampleOrg = `; example.org test file
$TTL 3600
@		IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2015082541 7200 3600 1209600 3600
@		IN	NS	b.iana-servers.net.
@		IN	NS	a.iana-servers.net.
@		IN	A	127.0.0.1
@		IN	A	127.0.0.2
short	1	IN	A	127.0.0.3

*.w        3600 IN      TXT     "Wildcard"
a.b.c.w    IN      TXT     "Not a wildcard"
cname      IN      CNAME   www.example.net.
service    IN      SRV     8080 10 10 @
`
