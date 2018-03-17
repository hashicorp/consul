package dnsserver

import "fmt"

// startUpZones create the text that we show when starting up:
// grpc://example.com.:1055
// example.com.:1053 on 127.0.0.1
func startUpZones(protocol, addr string, zones map[string]*Config) string {
	s := ""

	for zone := range zones {
		// split addr into protocol, IP and Port
		_, ip, port, err := SplitProtocolHostPort(addr)

		if err != nil {
			// this should not happen, but we need to take care of it anyway
			s += fmt.Sprintln(protocol + zone + ":" + addr)
			continue
		}
		if ip == "" {
			s += fmt.Sprintln(protocol + zone + ":" + port)
			continue
		}
		// if the server is listening on a specific address let's make it visible in the log,
		// so one can differentiate between all active listeners
		s += fmt.Sprintln(protocol + zone + ":" + port + " on " + ip)
	}
	return s
}
