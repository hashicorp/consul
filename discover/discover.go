package discover

import (
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/hashicorp/discover/aws"
	"github.com/hashicorp/discover/azure"
	"github.com/hashicorp/discover/gce"
)

func Discover(cfg string, l *log.Logger) ([]string, error) {
	c, err := parse(cfg)
	if err != nil {
		return nil, err
	}
	switch cfg := c.(type) {
	case nil:
		return nil, nil
	case *aws.Config:
		return aws.Discover(cfg, l)
	case *azure.Config:
		return azure.Discover(cfg, l)
	case *gce.Config:
		return gce.Discover(cfg, l)
	default:
		// if we get here then we updated parse but not Discover
		panic(fmt.Sprintf("discover: unknown provider config: %T", cfg))
	}
}

func parse(cfg string) (interface{}, error) {
	cfg = strings.TrimSpace(cfg)
	if cfg == "" {
		return nil, nil
	}

	m := map[string]string{}
	for _, v := range strings.Fields(cfg) {
		p := strings.SplitN(v, "=", 2)
		if len(p) != 2 {
			return nil, fmt.Errorf("discover: invalid format: %s", v)
		}
		key := p[0]
		val, err := url.QueryUnescape(p[1])
		if err != nil {
			return nil, fmt.Errorf("discover: invalid format: %s", v)
		}
		m[key] = val
	}

	switch m["provider"] {
	case "aws":
		return &aws.Config{
			Region:          m["region"],
			TagKey:          m["tag_key"],
			TagValue:        m["tag_value"],
			AccessKeyID:     m["access_key_id"],
			SecretAccessKey: m["secret_access_key"],
		}, nil

	case "azure":
		return &azure.Config{
			TagName:         m["tag_name"],
			TagValue:        m["tag_value"],
			SubscriptionID:  m["subscription_id"],
			TenantID:        m["tenant_id"],
			ClientID:        m["client_id"],
			SecretAccessKey: m["secret_access_key"],
		}, nil

	case "gce":
		return &gce.Config{
			ProjectName:     m["project_name"],
			ZonePattern:     m["zone_pattern"],
			TagValue:        m["tag_value"],
			CredentialsFile: m["credentials_file"],
		}, nil

	default:
		return nil, fmt.Errorf("discover: unknown provider: %q", m["provider"])
	}
}
