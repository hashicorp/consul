package test

const miekNL = `; miek.nl test zone
$TTL    30M
$ORIGIN miek.nl.
@       IN      SOA     linode.atoom.net. miek.miek.nl. (
			     1282630059 ; Serial
                             4H         ; Refresh
                             1H         ; Retry
                             7D         ; Expire
                             4H )       ; Negative Cache TTL
                IN      NS      linode.atoom.net.
		IN	NS	ns-ext.nlnetlabs.nl.
		IN	NS	omval.tednet.nl.
                IN      NS      ext.ns.whyscream.net.

                IN      MX      1  aspmx.l.google.com.
                IN      MX      5  alt1.aspmx.l.google.com.
                IN      MX      5  alt2.aspmx.l.google.com.
                IN      MX      10 aspmx2.googlemail.com.
                IN      MX      10 aspmx3.googlemail.com.

		IN 	A       176.58.119.54
		IN 	AAAA    2a01:7e00::f03c:91ff:fe79:234c
                IN      HINFO "Please stop asking for ANY" "See draft-ietf-dnsop-refuse-any"

a		IN 	A       176.58.119.54
		IN 	AAAA    2a01:7e00::f03c:91ff:fe79:234c
www     	IN 	CNAME 	a
archive         IN      CNAME   a
`
