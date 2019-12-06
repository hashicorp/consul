package sign

import (
	"strings"
	"testing"
	"time"
)

func TestResignInception(t *testing.T) {
	then := time.Date(2019, 7, 18, 22, 50, 0, 0, time.UTC)
	// signed yesterday
	zr := strings.NewReader(`miek.nl.	1800	IN	RRSIG	SOA 13 2 1800 20190808191936 20190717161936 59725 miek.nl. eU6gI1OkSEbyt`)
	if x := resign(zr, then); x != nil {
		t.Errorf("Expected RRSIG to be valid for %s, got invalid: %s", then.Format(timeFmt), x)
	}
	// inception starts after this date.
	zr = strings.NewReader(`miek.nl.	1800	IN	RRSIG	SOA 13 2 1800 20190808191936 20190731161936 59725 miek.nl. eU6gI1OkSEbyt`)
	if x := resign(zr, then); x == nil {
		t.Errorf("Expected RRSIG to be invalid for %s, got valid", then.Format(timeFmt))
	}
}

func TestResignExpire(t *testing.T) {
	then := time.Date(2019, 7, 18, 22, 50, 0, 0, time.UTC)
	// expires tomorrow
	zr := strings.NewReader(`miek.nl.	1800	IN	RRSIG	SOA 13 2 1800 20190717191936 20190717161936 59725 miek.nl. eU6gI1OkSEbyt`)
	if x := resign(zr, then); x == nil {
		t.Errorf("Expected RRSIG to be invalid for %s, got valid", then.Format(timeFmt))
	}
	// expire too far away
	zr = strings.NewReader(`miek.nl.	1800	IN	RRSIG	SOA 13 2 1800 20190731191936 20190717161936 59725 miek.nl. eU6gI1OkSEbyt`)
	if x := resign(zr, then); x != nil {
		t.Errorf("Expected RRSIG to be valid for %s, got invalid: %s", then.Format(timeFmt), x)
	}
	// expired yesterday
	zr = strings.NewReader(`miek.nl.	1800	IN	RRSIG	SOA 13 2 1800 20190721191936 20190717161936 59725 miek.nl. eU6gI1OkSEbyt`)
	if x := resign(zr, then); x == nil {
		t.Errorf("Expected RRSIG to be invalid for %s, got valid", then.Format(timeFmt))
	}
}
