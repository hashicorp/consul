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

It also possible for the root CA to expire.
In this case, use the instructions [here](Instructions from https://langui.sh/2009/01/18/openssl-self-signed-ca/) to regenerate root.cer and privkey.pem.
Configure the cert as follows:

```
Country Name (2 letter code) [AU]:US
State or Province Name (full name) [Some-State]:CA
Locality Name (eg, city) []:San Francisco
Organization Name (eg, company) [Internet Widgits Pty Ltd]:HashiCorp Test Cert
Organizational Unit Name (eg, section) []:Dev
Common Name (e.g. server FQDN or YOUR name) []:test.internal
Email Address []:test@internal.com
```

Ensure that you run `./test/ca/generate.sh` after recreating the root CA.

## Hostname certificates

if tests like `TestNewDialer_WithALPNWrapper` (or any other test using certificates located in `./test/hostname` ) are failing because of expired certificates, use `./generate.sh` script to regenerate a new set of certificate.

``` bash
cd test/hostname/
# Avoid deleting CertAuth.crt and privkey.pem since they're referenced in myca.conf
rm -rf "[Bonnie|Betty|Bob|Alice].crt" *.key && ./generate.sh
```

It also possible for the root CA to expire.
In this case, use the instructions [here](Instructions from https://langui.sh/2009/01/18/openssl-self-signed-ca/) to regenerate CertAuth.crt and privkey.pem.

```bash
openssl req -newkey rsa:2048 -days 3650 -x509 -nodes -out CertAuth.crt
```

Configure the cert as follows:
```
Country Name (2 letter code) [AU]:US
State or Province Name (full name) [Some-State]:CA
Locality Name (eg, city) []:San Francisco
Organization Name (eg, company) [Internet Widgits Pty Ltd]:HashiCorp Test Cert
Organizational Unit Name (eg, section) []:Test
Common Name (e.g. server FQDN or YOUR name) []:CertAuth
Email Address []:test@internal.com
```