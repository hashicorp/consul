package etcd

/*
func (s *server) AddressRecords(q dns.Question, name string, previousRecords []dns.RR, state middleware.State) (records []dns.RR, err error) {
	services, err := s.backend.Records(name, false)
	if err != nil {
		return nil, err
	}

	services = msg.Group(services)

	for _, serv := range services {
		ip := net.ParseIP(serv.Host)
		switch {
		case ip == nil:
			// Try to resolve as CNAME if it's not an IP, but only if we don't create loops.
			if q.Name == dns.Fqdn(serv.Host) {
				// x CNAME x is a direct loop, don't add those
				continue
			}

			newRecord := serv.NewCNAME(q.Name, dns.Fqdn(serv.Host))
			if len(previousRecords) > 7 {
				logf("CNAME lookup limit of 8 exceeded for %s", newRecord)
				// don't add it, and just continue
				continue
			}
			if s.isDuplicateCNAME(newRecord, previousRecords) {
				logf("CNAME loop detected for record %s", newRecord)
				continue
			}

			nextRecords, err := s.AddressRecords(dns.Question{Name: dns.Fqdn(serv.Host), Qtype: q.Qtype, Qclass: q.Qclass},
				strings.ToLower(dns.Fqdn(serv.Host)), append(previousRecords, newRecord), state)
			if err == nil {
				// Only have we found something we should add the CNAME and the IP addresses.
				if len(nextRecords) > 0 {
					records = append(records, newRecord)
					records = append(records, nextRecords...)
				}
				continue
			}
			// This means we can not complete the CNAME, try to look else where.
			target := newRecord.Target
			if dns.IsSubDomain(s.config.Domain, target) {
				// We should already have found it
				continue
			}
			m1, e1 := s.Lookup(target, q.Qtype, bufsize, dnssec)
			if e1 != nil {
				logf("incomplete CNAME chain: %s", e1)
				continue
			}
			// Len(m1.Answer) > 0 here is well?
			records = append(records, newRecord)
			records = append(records, m1.Answer...)
			continue
		case ip.To4() != nil && (q.Qtype == dns.TypeA || both):
			records = append(records, serv.NewA(q.Name, ip.To4()))
		case ip.To4() == nil && (q.Qtype == dns.TypeAAAA || both):
			records = append(records, serv.NewAAAA(q.Name, ip.To16()))
		}
	}
	return records, nil
}

// NSRecords returns NS records from etcd.
func (s *server) NSRecords(q dns.Question, state middleware.State) (records []dns.RR, extra []dns.RR, err error) {
	services, err := s.backend.Records(name, false)
	if err != nil {
		return nil, nil, err
	}

	services = msg.Group(services)

	for _, serv := range services {
		ip := net.ParseIP(serv.Host)
		switch {
		case ip == nil:
			return nil, nil, fmt.Errorf("NS record must be an IP address")
		case ip.To4() != nil:
			serv.Host = msg.Domain(serv.Key)
			records = append(records, serv.NewNS(q.Name, serv.Host))
			extra = append(extra, serv.NewA(serv.Host, ip.To4()))
		case ip.To4() == nil:
			serv.Host = msg.Domain(serv.Key)
			records = append(records, serv.NewNS(q.Name, serv.Host))
			extra = append(extra, serv.NewAAAA(serv.Host, ip.To16()))
		}
	}
	return records, extra, nil
}

// SRVRecords returns SRV records from etcd.
// If the Target is not a name but an IP address, a name is created.
func (s *server) SRVRecords(s middleware.State) (records []dns.RR, extra []dns.RR, err error) {
	services, err := s.backend.Records(name, false)
	if err != nil {
		return nil, nil, err
	}

	services = msg.Group(services)

	// Looping twice to get the right weight vs priority
	w := make(map[int]int)
	for _, serv := range services {
		weight := 100
		if serv.Weight != 0 {
			weight = serv.Weight
		}
		if _, ok := w[serv.Priority]; !ok {
			w[serv.Priority] = weight
			continue
		}
		w[serv.Priority] += weight
	}
	lookup := make(map[string]bool)
	for _, serv := range services {
		w1 := 100.0 / float64(w[serv.Priority])
		if serv.Weight == 0 {
			w1 *= 100
		} else {
			w1 *= float64(serv.Weight)
		}
		weight := uint16(math.Floor(w1))
		ip := net.ParseIP(serv.Host)
		switch {
		case ip == nil:
			srv := serv.NewSRV(q.Name, weight)
			records = append(records, srv)

			if _, ok := lookup[srv.Target]; ok {
				break
			}

			lookup[srv.Target] = true

			if !dns.IsSubDomain(s.config.Domain, srv.Target) {
				m1, e1 := s.Lookup(srv.Target, dns.TypeA, bufsize, dnssec)
				if e1 == nil {
					extra = append(extra, m1.Answer...)
				}
				m1, e1 = s.Lookup(srv.Target, dns.TypeAAAA, bufsize, dnssec)
				if e1 == nil {
					// If we have seen CNAME's we *assume* that they are already added.
					for _, a := range m1.Answer {
						if _, ok := a.(*dns.CNAME); !ok {
							extra = append(extra, a)
						}
					}
				}
				break
			}
			// Internal name, we should have some info on them, either v4 or v6
			// Clients expect a complete answer, because we are a recursor in their
			// view.
			addr, e1 := s.AddressRecords(dns.Question{srv.Target, dns.ClassINET, dns.TypeA},
				srv.Target, nil, bufsize, dnssec, true)
			if e1 == nil {
				extra = append(extra, addr...)
			}
		case ip.To4() != nil:
			serv.Host = msg.Domain(serv.Key)
			srv := serv.NewSRV(q.Name, weight)

			records = append(records, srv)
			extra = append(extra, serv.NewA(srv.Target, ip.To4()))
		case ip.To4() == nil:
			serv.Host = msg.Domain(serv.Key)
			srv := serv.NewSRV(q.Name, weight)

			records = append(records, srv)
			extra = append(extra, serv.NewAAAA(srv.Target, ip.To16()))
		}
	}
	return records, extra, nil
}

// MXRecords returns MX records from etcd.
// If the Target is not a name but an IP address, a name is created.
func (s *server) MXRecords(q dns.Question, name string, s middleware.State) (records []dns.RR, extra []dns.RR, err error) {
	services, err := s.backend.Records(name, false)
	if err != nil {
		return nil, nil, err
	}

	lookup := make(map[string]bool)
	for _, serv := range services {
		if !serv.Mail {
			continue
		}
		ip := net.ParseIP(serv.Host)
		switch {
		case ip == nil:
			mx := serv.NewMX(q.Name)
			records = append(records, mx)
			if _, ok := lookup[mx.Mx]; ok {
				break
			}

			lookup[mx.Mx] = true

			if !dns.IsSubDomain(s.config.Domain, mx.Mx) {
				m1, e1 := s.Lookup(mx.Mx, dns.TypeA, bufsize, dnssec)
				if e1 == nil {
					extra = append(extra, m1.Answer...)
				}
				m1, e1 = s.Lookup(mx.Mx, dns.TypeAAAA, bufsize, dnssec)
				if e1 == nil {
					// If we have seen CNAME's we *assume* that they are already added.
					for _, a := range m1.Answer {
						if _, ok := a.(*dns.CNAME); !ok {
							extra = append(extra, a)
						}
					}
				}
				break
			}
			// Internal name
			addr, e1 := s.AddressRecords(dns.Question{mx.Mx, dns.ClassINET, dns.TypeA},
				mx.Mx, nil, bufsize, dnssec, true)
			if e1 == nil {
				extra = append(extra, addr...)
			}
		case ip.To4() != nil:
			serv.Host = msg.Domain(serv.Key)
			records = append(records, serv.NewMX(q.Name))
			extra = append(extra, serv.NewA(serv.Host, ip.To4()))
		case ip.To4() == nil:
			serv.Host = msg.Domain(serv.Key)
			records = append(records, serv.NewMX(q.Name))
			extra = append(extra, serv.NewAAAA(serv.Host, ip.To16()))
		}
	}
	return records, extra, nil
}

func (s *server) CNAMERecords(q dns.Question, state middleware.State) (records []dns.RR, err error) {
	services, err := s.backend.Records(name, true)
	if err != nil {
		return nil, err
	}

	services = msg.Group(services)

	if len(services) > 0 {
		serv := services[0]
		if ip := net.ParseIP(serv.Host); ip == nil {
			records = append(records, serv.NewCNAME(q.Name, dns.Fqdn(serv.Host)))
		}
	}
	return records, nil
}

func (s *server) TXTRecords(q dns.Question, state middleware.State) (records []dns.RR, err error) {
	services, err := s.backend.Records(name, false)
	if err != nil {
		return nil, err
	}

	services = msg.Group(services)

	for _, serv := range services {
		if serv.Text == "" {
			continue
		}
		records = append(records, serv.NewTXT(q.Name))
	}
	return records, nil
}

func isDuplicateCNAME(r *dns.CNAME, records []dns.RR) bool {
	for _, rec := range records {
		if v, ok := rec.(*dns.CNAME); ok {
			if v.Target == r.Target {
				return true
			}
		}
	}
	return false
}

// Move to state.go somehow?
func (s *server) NameError(req *dns.Msg) *dns.Msg {
	m := new(dns.Msg)
	m.SetRcode(req, dns.RcodeNameError)
	m.Ns = []dns.RR{s.NewSOA()}
	m.Ns[0].Header().Ttl = s.config.MinTtl
	return m
}

// overflowOrTruncated writes back an error to the client if the message does not fit.
// It updates prometheus metrics. If something has been written to the client, true
// will be returned.
func (s *server) overflowOrTruncated(w dns.ResponseWriter, m *dns.Msg, bufsize int, sy metrics.System) bool {
	switch isTCP(w) {
	case true:
		if _, overflow := Fit(m, dns.MaxMsgSize, true); overflow {
			metrics.ReportErrorCount(m, sy)
			msgFail := s.ServerFailure(m)
			w.WriteMsg(msgFail)
			return true
		}
	case false:
		// Overflow with udp always results in TC.
		Fit(m, bufsize, false)
		metrics.ReportErrorCount(m, sy)
		if m.Truncated {
			w.WriteMsg(m)
			return true
		}
	}
	return false
}

// etcNameError return a NameError to the client if the error
// returned from etcd has ErrorCode == 100.
func isEtcdNameError(err error, s *server) bool {
	if e, ok := err.(etcd.Error); ok && e.Code == etcd.ErrorCodeKeyNotFound {
		return true
	}
	if err != nil {
		logf("error from backend: %s", err)
	}
	return false
}
*/
