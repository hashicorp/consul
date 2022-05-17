// From Go src/crypto/x509/root_bsd.go
//go:build dragonfly || freebsd || netbsd || openbsd

package xds

var certFiles = []string{
	"/usr/local/etc/ssl/cert.pem",            // FreeBSD
	"/etc/ssl/cert.pem",                      // OpenBSD
	"/usr/local/share/certs/ca-root-nss.crt", // DragonFly
	"/etc/openssl/certs/ca-certificates.crt", // NetBSD
}
