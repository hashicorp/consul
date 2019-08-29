// Package sign implements a zone signer as a plugin.
package sign

import (
	"path/filepath"
	"time"
)

// Sign contains signers that sign the zones files.
type Sign struct {
	signers []*Signer
}

// OnStartup scans all signers and signs or resigns zones if needed.
func (s *Sign) OnStartup() error {
	for _, signer := range s.signers {
		why := signer.resign()
		if why == nil {
			log.Infof("Skipping signing zone %q in %q: signatures are valid", signer.origin, filepath.Join(signer.directory, signer.signedfile))
			continue
		}
		go signAndLog(signer, why)
	}
	return nil
}

// Various duration constants for signing of the zones.
const (
	DurationExpireDays              = 7 * 24 * time.Hour  // max time allowed before expiration
	DurationResignDays              = 6 * 24 * time.Hour  // if the last sign happenend this long ago, sign again
	DurationSignatureExpireDays     = 32 * 24 * time.Hour // sign for 32 days
	DurationRefreshHours            = 5 * time.Hour       // check zones every 5 hours
	DurationJitter                  = -18 * time.Hour     // default max jitter
	DurationSignatureInceptionHours = -3 * time.Hour      // -(2+1) hours, be sure to catch daylight saving time and such, jitter is substracted
)

const timeFmt = "2006-01-02T15:04:05.000Z07:00"
