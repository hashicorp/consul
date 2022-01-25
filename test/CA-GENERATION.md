# CA certificate generation procedure

## Client certificates
if tests like `TestAPI_ClientTLSOptions` (or any other test using certificates located in `./test/client_certs` ) are failing because of expired certificates, use `./generate.sh` script to regenerate a new set of certificate.

``` bash
cd test/client_certs/
rm -rf *.pem *.crt *.key && ./generate.sh
```

## CA certificates
if tests like `TestAgent_ReloadConfigTLSConfigFailure` (or any other test using certificates located in `./test/ca` ) are failing because of expired certificates, use `./generate.sh` script to regenerate a new set of certificate.

``` bash
cd test/ca/
rm -rf *.pem *.crt *.key && ./generate.sh
```

## Hostname certificates

if tests like `TestNewDialer_WithALPNWrapper` (or any other test using certificates located in `./test/hostname` ) are failing because of expired certificates, use `./generate.sh` script to regenerate a new set of certificate.

``` bash
cd test/hostname/
rm -rf *.pem *.crt *.key && ./generate.sh
```