// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var invalidDNSNameChars = regexp.MustCompile(`[^a-z0-9]`)

// CompactUID returns a crypto random Unique Identifier string consiting of 8
// characters of base36 encoded random value. This has roughly 41 bits of
// entropy so is suitable for infrequently occuring events with low probability
// of collision. It is not suitable for UUIDs for very frequent events. It's
// main purpose is to assign unique values to CA certificate Common Names which
// need to be unique in some providers - see CACN - but without using up large
// amounts of the limited 64 character Common Name. It also makes the values
// more easily digestable by humans considering there are likely to be few of
// them ever in use.
func CompactUID() (string, error) {
	// 48 bits (6 bytes) is enough to fill 8 bytes in base36 but it's simpler to
	// have a whole uint8 to convert from.
	var raw [8]byte
	_, err := rand.Read(raw[:])
	if err != nil {
		return "", err
	}

	i := binary.LittleEndian.Uint64(raw[:])
	return truncateTo(strconv.FormatInt(int64(i), 36), 8), nil
}

// CACN returns the common name for a CA certificate.
// A uniqueID is requires because some providers (e.g.
// Vault) cache by subject and so produce incorrect results - for example they
// won't cross-sign an older CA certificate with the same common name since they
// think they already have a valid cert for that CN and just return the current
// root.
//
// This can be generated by any means but will be truncated to 8 chars and
// sanitised to DNS-safe chars. CompactUID generates suitable UIDs for this
// specific purpose.
//
// Format is:
//
//	{provider}-{uniqueID_first8}.{pri|sec}.ca.<trust_domain_first_8>.consul
//
// trust domain is truncated to keep the whole name short
func CACN(provider, uniqueID, trustDomain string, primaryDC bool) string {
	providerSan := invalidDNSNameChars.ReplaceAllString(strings.ToLower(provider), "")
	typ := "pri"
	if !primaryDC {
		typ = "sec"
	}
	// 32 = 7 bytes for ".consul", 8 bytes for trust domain, 8 bytes for
	// ".pri.ca.", 9 bytes for "-{uniqueID-8-b36}"
	uidSAN := invalidDNSNameChars.ReplaceAllString(strings.ToLower(uniqueID), "")
	return fmt.Sprintf("%s-%s.%s.ca.%s.consul", typ, truncateTo(uidSAN, 8),
		truncateTo(providerSan, 64-32), truncateTo(trustDomain, 8))
}

func truncateTo(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
