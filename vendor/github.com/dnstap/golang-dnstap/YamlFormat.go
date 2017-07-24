/*
 * Copyright (c) 2013-2014 by Farsight Security, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package dnstap

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
)

const yamlTimeFormat = "2006-01-02 15:04:05.999999999"

func yamlConvertMessage(m *Message, s *bytes.Buffer) {
	s.WriteString(fmt.Sprint("  type: ", m.Type, "\n"))

	if m.QueryTimeSec != nil && m.QueryTimeNsec != nil {
		t := time.Unix(int64(*m.QueryTimeSec), int64(*m.QueryTimeNsec)).UTC()
		s.WriteString(fmt.Sprint("  query_time: !!timestamp ", t.Format(yamlTimeFormat), "\n"))
	}

	if m.ResponseTimeSec != nil && m.ResponseTimeNsec != nil {
		t := time.Unix(int64(*m.ResponseTimeSec), int64(*m.ResponseTimeNsec)).UTC()
		s.WriteString(fmt.Sprint("  response_time: !!timestamp ", t.Format(yamlTimeFormat), "\n"))
	}

	if m.SocketFamily != nil {
		s.WriteString(fmt.Sprint("  socket_family: ", m.SocketFamily, "\n"))
	}

	if m.SocketProtocol != nil {
		s.WriteString(fmt.Sprint("  socket_protocol: ", m.SocketProtocol, "\n"))
	}

	if m.QueryAddress != nil {
		s.WriteString(fmt.Sprint("  query_address: ", net.IP(m.QueryAddress), "\n"))
	}

	if m.ResponseAddress != nil {
		s.WriteString(fmt.Sprint("  response_address: ", net.IP(m.ResponseAddress), "\n"))
	}

	if m.QueryPort != nil {
		s.WriteString(fmt.Sprint("  query_port: ", *m.QueryPort, "\n"))
	}

	if m.ResponsePort != nil {
		s.WriteString(fmt.Sprint("  response_port: ", *m.ResponsePort, "\n"))
	}

	if m.QueryZone != nil {
		name, _, err := dns.UnpackDomainName(m.QueryZone, 0)
		if err != nil {
			s.WriteString("  # query_zone: parse failed\n")
		} else {
			s.WriteString(fmt.Sprint("  query_zone: ", strconv.Quote(name), "\n"))
		}
	}

	if m.QueryMessage != nil {
		msg := new(dns.Msg)
		err := msg.Unpack(m.QueryMessage)
		if err != nil {
			s.WriteString("  # query_message: parse failed\n")
		} else {
			s.WriteString("  query_message: |\n")
			s.WriteString("    " + strings.Replace(strings.TrimSpace(msg.String()), "\n", "\n    ", -1) + "\n")
		}
	}
	if m.ResponseMessage != nil {
		msg := new(dns.Msg)
		err := msg.Unpack(m.ResponseMessage)
		if err != nil {
			s.WriteString(fmt.Sprint("  # response_message: parse failed: ", err, "\n"))
		} else {
			s.WriteString("  response_message: |\n")
			s.WriteString("    " + strings.Replace(strings.TrimSpace(msg.String()), "\n", "\n    ", -1) + "\n")
		}
	}
	s.WriteString("---\n")
}

func YamlFormat(dt *Dnstap) (out []byte, ok bool) {
	var s bytes.Buffer

	s.WriteString(fmt.Sprint("type: ", dt.Type, "\n"))
	if dt.Identity != nil {
		s.WriteString(fmt.Sprint("identity: ", strconv.Quote(string(dt.Identity)), "\n"))
	}
	if dt.Version != nil {
		s.WriteString(fmt.Sprint("version: ", strconv.Quote(string(dt.Version)), "\n"))
	}
	if *dt.Type == Dnstap_MESSAGE {
		s.WriteString("message:\n")
		yamlConvertMessage(dt.Message, &s)
	}
	return s.Bytes(), true
}
