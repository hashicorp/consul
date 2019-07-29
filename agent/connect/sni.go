package connect

import (
	"fmt"
)

func DatacenterSNI(dc, trustDomain string) string {
	return fmt.Sprintf("%s.internal.%s", dc, trustDomain)
}

func ServiceSNI(service, subset, namespace, datacenter, trustDomain string) string {
	if namespace == "" {
		namespace = "default"
	}

	if subset == "" {
		return fmt.Sprintf("%s.%s.%s.internal.%s", service, namespace, datacenter, trustDomain)
	} else {
		return fmt.Sprintf("%s.%s.%s.%s.internal.%s", subset, service, namespace, datacenter, trustDomain)
	}
}

func QuerySNI(service, datacenter, trustDomain string) string {
	return fmt.Sprintf("%s.default.%s.query.%s", service, datacenter, trustDomain)
}
