#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -e

trap "trap - SIGTERM && kill -- -$$" SIGINT SIGTERM EXIT


readonly SCRIPT_NAME="$(basename ${BASH_SOURCE[0]})"
readonly SCRIPT_DIR="$(dirname ${BASH_SOURCE[0]})"

# Start a couple dev agents in the background
echo "Starting Dev Agents"
consul agent -dev -hcl 'acl { enabled = true default_policy="allow" tokens { initial_management = "root" } }' >/dev/null 2>&1 &
consul agent -dev -dns-port=9600 -grpc-port=9502 -grpc-tls-port=9503 -http-port=9500 -serf-lan-port=9301 -serf-wan-port=9302 -server-port=9300 >/dev/null 2>&1 &

# should be long enough for the dev agents to be available
sleep 5

# This script expects a consul dev agent with acls enabled in default allow to be running on localhost
#    consul agent -dev -hcl 'acl { enabled = true default_policy="allow" tokens { initial_management = "root" } }'
# It also requires another dev agent running on alternative ports to peer with
#    consul agent -dev  -dns-port=9600 -grpc-port=9502 -grpc-tls-port=9503 -http-port=9500 -serf-lan-port=9301 -serf-wan-port=9302 -server-port=9300

# Just running Consul will cause the following data to be in the snapshot:
# Register
# ConnectCA
# ConnectCAProviderState
# ConnectCAConfig
# Autopilot
# Index
# SystemMetadata
# CoordinateBatchUpdate
# FederationState
# ChunkingState
# FreeVirtualIP
# Partition
# Tombstone

# Ensure a KV entry ends up in the snapshot
echo "Creating KV Entry"
consul kv put foo/bar 1 >/dev/null

# Ensure a tombstone ends up in the snapshot
echo "Forcing KV Tombstone Creation"
consul kv put foo/baz 2 >/dev/null
consul kv delete foo/baz > /dev/null


# Ensure a session ends up in the snapshot
echo "Creating Session"
curl -s -X PUT localhost:8500/v1/session/create >/dev/null

# Ensure a prepared query ends up in the snapshot
echo "Creating Prepared Query"
curl -s -X POST localhost:8500/v1/query -d '{"Name": "test", "Token": "root", "Service": {"Service": "test"}}' >/dev/null

# Ensure an ACL token ends up in the snapshot
echo "Creating ACL Token"
consul acl token create -node-identity=localhost:dc1 >/dev/null

# Ensure an ACL policy ends up in the snapshot
echo "Creating ACL Policy"
consul acl policy create -name=test -rules='node_prefix "" { policy = "write" }' >/dev/null

# Ensure an ACL role ends up in the snapshot
echo "Creating ACL Role"
consul acl role create -name=test -policy-name=test >/dev/null

# Ensure an ACL auth method ends up in the snapshot
echo "Creating ACL Auth Method"
consul acl auth-method create -type jwt -name test -config '{"JWTValidationPubKeys": ["-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAENRw6ZwlBOx5XZKjcc1HhU00sDehc\n8nqeeSnRZLv89yT7M7qUOFDtR29FR/AFUSAEOFl1iIYLqNMElHs2VkgAZA==\n-----END PUBLIC KEY-----"]}' >/dev/null

# Ensure an ACL binding rule ends up in the snapshot
echo "Creating ACL Binding Rule"
consul acl binding-rule create -bind-type="service" -bind-name="service" -method="test" >/dev/null

# Ensure config entries end up in the snapshot
echo "Creating Proxy Default Config Entry"
consul config write - >/dev/null <<EOF
Kind = "proxy-defaults"
Name = "global"
EOF

echo "Creating Service Defaults Config Entries"
consul config write - >/dev/null <<EOF
Kind = "service-defaults"
Name = "web"
Protocol = "http"
EOF

consul config write - >/dev/null <<EOF
Kind = "service-defaults"
Name = "foo"
Protocol = "http"
EOF

echo "Creating Service Router Config Entry"
consul config write - >/dev/null <<EOF
Kind = "service-router"
Name = "foo"
Routes = [
   {
      Match {
         HTTP {
         PathPrefix = "/foo"
         }
      }
      Destination {
         Service = "web"
      }
   }
]
EOF

echo "Creating Service Splitter Config Entry"
consul config write - >/dev/null <<EOF
Kind = "service-splitter"
Name = "web"
Splits = [
   {
      Weight = 100
      Service = "web"
   }
]
EOF

echo "Creating Service Resolver Config Entry"
consul config write - >/dev/null <<EOF
Kind = "service-resolver"
Name = "web"
RequestTimeout = "3s"
EOF

echo "Creating Ingress Gateway Config Entry"
consul config write - >/dev/null <<EOF
Kind = "ingress-gateway"
Name = "api"
EOF

echo "Creating Terminating Gateway Config Entry"
consul config write - >/dev/null <<EOF
Kind = "terminating-gateway"
Name = "external"
Services = [
   {
      Name = "external"
   }
]
EOF

echo "Creating Service Intentions Config Entry"
consul config write - >/dev/null <<EOF
Kind = "service-intentions"
Name = "web"
Sources = [
   {
      Name = "api"
      Action = "allow"
   }
]
EOF

echo "Creating Mesh Config Entry"
consul config write - >/dev/null <<EOF
Kind = "mesh"
AllowEnablingPermissiveMutualTLS = true
EOF

echo "Creating Exported Service Config Entry"
consul config write - >/dev/null <<EOF
Kind = "exported-services"
Name = "default"
Services = [
   {
      Name = "web"
      Consumers = [
         {
            Peer = "other"
         }
      ]
   }
]
EOF

echo "Creating Inline Certificate Config Entry"
consul config write - >/dev/null <<EOF
Kind = "inline-certificate"
Name = "blah"
Certificate = "-----BEGIN CERTIFICATE-----\nMIICnjCCAkSgAwIBAgIQAxVHhSG0wSbdZm+3ToYAkDAKBggqhkjOPQQDAjCBuTEL\nMAkGA1UEBhMCVVMxCzAJBgNVBAgTAkNBMRYwFAYDVQQHEw1TYW4gRnJhbmNpc2Nv\nMRowGAYDVQQJExExMDEgU2Vjb25kIFN0cmVldDEOMAwGA1UEERMFOTQxMDUxFzAV\nBgNVBAoTDkhhc2hpQ29ycCBJbmMuMUAwPgYDVQQDEzdDb25zdWwgQWdlbnQgQ0Eg\nMjgwNzE4MDMxODA1Mjk2OTA1NzQ4MzU3NjI1MTI5ODQ5NDA5NjI3MCAXDTIzMTEw\nMjE1Mjk0NVoYDzIxMjMxMDA5MTUyOTQ1WjAcMRowGAYDVQQDExFjbGllbnQuZGMx\nLmNvbnN1bDBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABKvl1yhbsI9r7IxJxLrt\nZTNYXkCXuFy8q3gsokMqsl/MUynrIBrd9NrZEQA91ZArUYzF1+QlxM6D4hRJc5CR\n3x6jgccwgcQwDgYDVR0PAQH/BAQDAgWgMB0GA1UdJQQWMBQGCCsGAQUFBwMCBggr\nBgEFBQcDATAMBgNVHRMBAf8EAjAAMCkGA1UdDgQiBCCvXve+zMFSJMXNS3l3YL9k\n2QH8zF74wa+TlwFSaQEjGzArBgNVHSMEJDAigCBGa65jF6Wwq9OmdbgJIRCYv++x\nHG8dRBUpwvSk0Mk1+jAtBgNVHREEJjAkghFjbGllbnQuZGMxLmNvbnN1bIIJbG9j\nYWxob3N0hwR/AAABMAoGCCqGSM49BAMCA0gAMEUCIBLqa1Zh3KUE0RiQzWdoYXkU\nwZo5aBw9ujqzLyAqxToFAiEAihWmc4r6lDYRR35X4QB1nTT92POJRClsfLPOTRG5\nrsU=\n-----END CERTIFICATE-----"
PrivateKey = "-----BEGIN EC PRIVATE KEY-----\nMHcCAQEEINE2CQhnu7ipo67FGbEBRXoYRCTM4uJdHgNRTrdkAnHCoAoGCCqGSM49\nAwEHoUQDQgAEq+XXKFuwj2vsjEnEuu1lM1heQJe4XLyreCyiQyqyX8xTKesgGt30\n2tkRAD3VkCtRjMXX5CXEzoPiFElzkJHfHg==\n-----END EC PRIVATE KEY-----"
EOF

echo "Creating API GW Config Entry"
consul config write - >/dev/null <<EOF
Kind = "api-gateway"
Name = "apigw"
Listeners = [
   {
      Name = "tcp"
      Port = 443
      Protocol = "tcp"
      TLS = {
         Certificates = [
            {
               Kind = "inline-certificate"
               Name = "blah"
            }
         ]
      }
   },
   {
      Name = "http"
      Port = 8080
      Protocol = "http"
   }
]
EOF

# write a sameness group config entry if this is enterprise
consul version | rg "\+ent" >/dev/null
if [ $? -eq 0 ]; then
set -e
echo "Creating Sameness Group Config Entry"
consul config write - >/dev/null <<EOF
Kind = "sameness-group"
Name = "default"
DefaultForFailover = true
IncludeLocal = true
Members = [
   {
      Peer = "other"
   }
]
EOF
fi

echo "Creating TCP Route Config Entry"
consul config write - >/dev/null <<EOF
Kind = "tcp-route"
Name = "fake"
Parents = [
   {
      Kind = "api-gateway"
      Name = "apigw"
      SectionName = "tcp"
   }
]
Services = [
   {
      Name = "fake"
   }
]
EOF

echo "Creating HTTP Route Config Entry"
consul config write - >/dev/null <<EOF
Kind = "http-route"
Name = "web"
Parents = [
   {
      Kind = "api-gateway"
      Name = "apigw"
      SectionName = "http"
   }
]
Rules = [
   {
      Services = [
         {
            Name = "fake"
         }
      ]
   }
]
EOF

echo "Creating JWT Provider Config Entry"
consul config write - >/dev/null <<EOF
Kind = "jwt-provider"
Name = "whocare"
JSONWebKeySet {
   Local {
      Filename = "/tmp/jwks.json"
   }
}
EOF

# Ensure a peering data ends up in the snapshot
echo "Creating Peering Config Entry"
consul peering establish -http-addr=localhost:9500 -name other -peering-token="$(consul peering generate-token -name other)" >/dev/null

echo "Saving Snapshot to all.snap"
sleep 2
consul snapshot save "${SCRIPT_DIR}/all.snap" >/dev/null