// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package pbproxystate

func (s *L4Principal) ToL7Principal() *L7Principal {
	out := &L7Principal{
		Spiffe: &Spiffe{
			Regex: s.SpiffeRegex,
		},
	}

	for _, regex := range s.ExcludeSpiffeRegexes {
		out.ExcludeSpiffes = append(out.ExcludeSpiffes, &Spiffe{
			Regex: regex,
		})
	}

	return out
}
