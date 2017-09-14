package test

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

// TempFile will create a temporary file on disk and returns the name and a cleanup function to remove it later.
func TempFile(dir, content string) (string, func(), error) {
	f, err := ioutil.TempFile(dir, "go-test-tmpfile")
	if err != nil {
		return "", nil, err
	}
	if err := ioutil.WriteFile(f.Name(), []byte(content), 0644); err != nil {
		return "", nil, err
	}
	rmFunc := func() { os.Remove(f.Name()) }
	return f.Name(), rmFunc, nil
}

// WritePEMFiles creates a tmp dir with ca.pem, cert.pem, and key.pem and the func to remove it
func WritePEMFiles(dir string) (string, func(), error) {
	tempDir, err := ioutil.TempDir(dir, "go-test-pemfiles")
	if err != nil {
		return "", nil, err
	}

	data := `-----BEGIN CERTIFICATE-----
MIIC9zCCAd+gAwIBAgIJALGtqdMzpDemMA0GCSqGSIb3DQEBCwUAMBIxEDAOBgNV
BAMMB2t1YmUtY2EwHhcNMTYxMDE5MTU1NDI0WhcNNDQwMzA2MTU1NDI0WjASMRAw
DgYDVQQDDAdrdWJlLWNhMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA
pa4Wu/WkpJNRr8pMVE6jjwzNUOx5mIyoDr8WILSxVQcEeyVPPmAqbmYXtVZO11p9
jTzoEqF7Kgts3HVYGCk5abqbE14a8Ru/DmV5avU2hJ/NvSjtNi/O+V6SzCbg5yR9
lBR53uADDlzuJEQT9RHq7A5KitFkx4vUcXnjOQCbDogWFoYuOgNEwJPy0Raz3NJc
ViVfDqSJ0QHg02kCOMxcGFNRQ9F5aoW7QXZXZXD0tn3wLRlu4+GYyqt8fw5iNdLJ
t79yKp8I+vMTmMPz4YKUO+eCl5EY10Qs7wvoG/8QNbjH01BRN3L8iDT2WfxdvjTu
1RjPxFL92i+B7HZO7jGLfQIDAQABo1AwTjAdBgNVHQ4EFgQUZTrg+Xt87tkxDhlB
gKk9FdTOW3IwHwYDVR0jBBgwFoAUZTrg+Xt87tkxDhlBgKk9FdTOW3IwDAYDVR0T
BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEApB7JFVrZpGSOXNO3W7SlN6OCPXv9
C7rIBc8rwOrzi2mZWcBmWheQrqBo8xHif2rlFNVQxtq3JcQ8kfg/m1fHeQ/Ygzel
Z+U1OqozynDySBZdNn9i+kXXgAUCqDPp3hEQWe0os/RRpIwo9yOloBxdiX6S0NIf
VB8n8kAynFPkH7pYrGrL1HQgDFCSfa4tUJ3+9sppnCu0pNtq5AdhYx9xFb2sn+8G
xGbtCkhVk2VQ+BiCWnjYXJ6ZMzabP7wiOFDP9Pvr2ik22PRItsW/TLfHFXM1jDmc
I1rs/VUGKzcJGVIWbHrgjP68CTStGAvKgbsTqw7aLXTSqtPw88N9XVSyRg==
-----END CERTIFICATE-----`
	path := filepath.Join(tempDir, "ca.pem")
	if err := ioutil.WriteFile(path, []byte(data), 0644); err != nil {
		return "", nil, err
	}
	data = `-----BEGIN CERTIFICATE-----
MIICozCCAYsCCQCRlf5BrvPuqjANBgkqhkiG9w0BAQsFADASMRAwDgYDVQQDDAdr
dWJlLWNhMB4XDTE2MTAxOTE2MDUxOFoXDTE3MTAxOTE2MDUxOFowFTETMBEGA1UE
AwwKa3ViZS1hZG1pbjCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAMTw
a7wCFoiCad/N53aURfjrme+KR7FS0yf5Ur9OR/oM3BoS9stYu5Flzr35oL5T6t5G
c2ey78mUs/Cs07psnjUdKH55bDpJSdG7zW9mXNyeLwIefFcj/38SS5NBSotmLo8u
scJMGXeQpCQtfVuVJSP2bfU5u5d0KTLSg/Cor6UYonqrRB82HbOuuk8Wjaww4VHo
nCq7X8o948V6HN5ZibQOgMMo+nf0wORREHBjvwc4W7ewbaTcfoe1VNAo/QnkqxTF
ueMb2HxgghArqQSK8b44O05V0zrde25dVnmnte6sPjcV0plqMJ37jViISxsOPUFh
/ZW7zbIM/7CMcDekCiECAwEAATANBgkqhkiG9w0BAQsFAAOCAQEAYZE8OxwRR7GR
kdd5aIriDwWfcl56cq5ICyx87U8hAZhBxk46a6a901LZPzt3xKyWIFQSRj/NYiQ+
/thjGLZI2lhkVgYtyAD4BNxDiuppQSCbkjY9tLVDdExGttEVN7+UYDWJBHy6X16Y
xSG9FE3Dvp9LI89Nq8E3dRh+Q8wu52q9HaQXjS5YtzQOtDFKPBkihXu/c6gEHj4Y
bZVk8rFiH8/CvcQxAuvNI3VVCFUKd2LeQtqwYQQ//qoiuA15krTq5Ut9eXJ8zxAw
zhDEPP4FhY+Sz+y1yWirphl7A1aZwhXVPcfWIGqpQ3jzNwUeocbH27kuLh+U4hQo
qeg10RdFnw==
-----END CERTIFICATE-----`
	path = filepath.Join(tempDir, "cert.pem")
	if err = ioutil.WriteFile(path, []byte(data), 0644); err != nil {
		return "", nil, err
	}

	data = `-----BEGIN RSA PRIVATE KEY-----
MIIEpgIBAAKCAQEAxPBrvAIWiIJp383ndpRF+OuZ74pHsVLTJ/lSv05H+gzcGhL2
y1i7kWXOvfmgvlPq3kZzZ7LvyZSz8KzTumyeNR0ofnlsOklJ0bvNb2Zc3J4vAh58
VyP/fxJLk0FKi2Yujy6xwkwZd5CkJC19W5UlI/Zt9Tm7l3QpMtKD8KivpRiieqtE
HzYds666TxaNrDDhUeicKrtfyj3jxXoc3lmJtA6Awyj6d/TA5FEQcGO/Bzhbt7Bt
pNx+h7VU0Cj9CeSrFMW54xvYfGCCECupBIrxvjg7TlXTOt17bl1Weae17qw+NxXS
mWownfuNWIhLGw49QWH9lbvNsgz/sIxwN6QKIQIDAQABAoIBAQDCXq9V7ZGjxWMN
OkFaLVkqJg3V91puztoMt+xNV8t+JTcOnOzrIXZuOFbl9PwLHPPP0SSRkm9LOvKl
dU26zv0OWureeKSymia7U2mcqyC3tX+bzc7WinbeSYZBnc0e7AjD1EgpBcaU1TLL
agIxY3A2oD9CKmrVPhZzTIZf/XztqTYjhvs5I2kBeT0imdYGpXkdndRyGX4I5/JQ
fnp3Czj+AW3zX7RvVnXOh4OtIAcfoG9xoNyD5LOSlJkkX0MwTS8pEBeZA+A4nb+C
ivjnOSgXWD+liisI+LpBgBbwYZ/E49x5ghZYrJt8QXSk7Bl/+UOyv6XZAm2mev6j
RLAZtoABAoGBAP2P+1PoKOwsk+d/AmHqyTCUQm0UG18LOLB/5PyWfXs/6caDmdIe
DZWeZWng1jUQLEadmoEw/CBY5+tPfHlzwzMNhT7KwUfIDQCIBoS7dzHYnwrJ3VZh
qYA05cuGHAAHqwb6UWz3y6Pa4AEVSHX6CM83CAi9jdWZ1rdZybWG+qYBAoGBAMbV
FsR/Ft+tK5ALgXGoG83TlmxzZYuZ1SnNje1OSdCQdMFCJB10gwoaRrw1ICzi40Xk
ydJwV1upGz1om9ReDAD1zQM9artmQx6+TVLiVPALuARdZE70+NrA6w3ZvxUgJjdN
ngvXUr+8SdvaYUAwFu7BulfJlwXjUS711hHW/KQhAoGBALY41QuV2mLwHlLNie7I
hlGtGpe9TXZeYB0nrG6B0CfU5LJPPSotguG1dXhDpm138/nDpZeWlnrAqdsHwpKd
yPhVjR51I7XsZLuvBdA50Q03egSM0c4UXXXPjh1XgaPb3uMi3YWMBwL4ducQXoS6
bb5M9C8j2lxZNF+L3VPhbxwBAoGBAIEWDvX7XKpTDxkxnxRfA84ZNGusb5y2fsHp
Bd+vGBUj8+kUO8Yzwm9op8vA4ebCVrMl2jGZZd3IaDryE1lIxZpJ+pPD5+tKdQEc
o67P6jz+HrYWu+zW9klvPit71qasfKMi7Rza6oo4f+sQWFsH3ZucgpJD+pyD/Ez0
pcpnPRaBAoGBANT/xgHBfIWt4U2rtmRLIIiZxKr+3mGnQdpA1J2BCh+/6AvrEx//
E/WObVJXDnBdViu0L9abE9iaTToBVri4cmlDlZagLuKVR+TFTCN/DSlVZTDkqkLI
8chzqtkH6b2b2R73hyRysWjsomys34ma3mEEPTX/aXeAF2MSZ/EWT9yL
-----END RSA PRIVATE KEY-----`
	path = filepath.Join(tempDir, "key.pem")
	if err = ioutil.WriteFile(path, []byte(data), 0644); err != nil {
		return "", nil, err
	}

	rmFunc := func() { os.RemoveAll(tempDir) }
	return tempDir, rmFunc, nil
}
