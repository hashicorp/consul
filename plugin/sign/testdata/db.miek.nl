$TTL    30M
$ORIGIN miek.nl.
@       IN      SOA     linode.atoom.net. miek.miek.nl. ( 1282630060 4H 1H 7D 4H )
                IN      NS      linode.atoom.net.
                IN      MX      1  aspmx.l.google.com.
		IN 	AAAA    2a01:7e00::f03c:91ff:fe79:234c
		IN	DNSKEY  257 3 13 sfzRg5nDVxbeUc51su4MzjgwpOpUwnuu81SlRHqJuXe3SOYOeypR69tZ52XLmE56TAmPHsiB8Rgk+NTpf0o1Cw==

a		IN 	AAAA    2a01:7e00::f03c:91ff:fe79:234c
www     	IN 	CNAME 	a


bla                 IN  NS      ns1.bla.com.
ns3.blaaat.miek.nl. IN  AAAA    ::1 ; non-glue, should be signed.
; in baliwick nameserver that requires glue, should not be signed
bla                 IN  NS      ns2.bla.miek.nl.
ns2.bla.miek.nl.    IN  A       127.0.0.1
