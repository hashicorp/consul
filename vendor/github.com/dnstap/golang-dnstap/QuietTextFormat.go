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
	"time"

	"github.com/miekg/dns"
)

const quietTimeFormat = "15:04:05"

func textConvertTime(s *bytes.Buffer, secs *uint64, nsecs *uint32) {
	if secs != nil {
		s.WriteString(time.Unix(int64(*secs), 0).Format(quietTimeFormat))
	} else {
		s.WriteString("??:??:??")
	}
	if nsecs != nil {
		s.WriteString(fmt.Sprintf(".%06d", *nsecs/1000))
	} else {
		s.WriteString(".??????")
	}
}

func textConvertIP(s *bytes.Buffer, ip []byte) {
	if ip != nil {
		s.WriteString(net.IP(ip).String())
	} else {
		s.WriteString("MISSING_ADDRESS")
	}
}

func textConvertMessage(m *Message, s *bytes.Buffer) {
	isQuery := false
	printQueryAddress := false

	switch *m.Type {
	case Message_CLIENT_QUERY,
		Message_RESOLVER_QUERY,
		Message_AUTH_QUERY,
		Message_FORWARDER_QUERY,
		Message_TOOL_QUERY:
		isQuery = true
	case Message_CLIENT_RESPONSE,
		Message_RESOLVER_RESPONSE,
		Message_AUTH_RESPONSE,
		Message_FORWARDER_RESPONSE,
		Message_TOOL_RESPONSE:
		isQuery = false
	default:
		s.WriteString("[unhandled Message.Type]\n")
		return
	}

	if isQuery {
		textConvertTime(s, m.QueryTimeSec, m.QueryTimeNsec)
	} else {
		textConvertTime(s, m.ResponseTimeSec, m.ResponseTimeNsec)
	}
	s.WriteString(" ")

	switch *m.Type {
	case Message_CLIENT_QUERY,
		Message_CLIENT_RESPONSE:
		{
			s.WriteString("C")
		}
	case Message_RESOLVER_QUERY,
		Message_RESOLVER_RESPONSE:
		{
			s.WriteString("R")
		}
	case Message_AUTH_QUERY,
		Message_AUTH_RESPONSE:
		{
			s.WriteString("A")
		}
	case Message_FORWARDER_QUERY,
		Message_FORWARDER_RESPONSE:
		{
			s.WriteString("F")
		}
	case Message_STUB_QUERY,
		Message_STUB_RESPONSE:
		{
			s.WriteString("S")
		}
	case Message_TOOL_QUERY,
		Message_TOOL_RESPONSE:
		{
			s.WriteString("T")
		}
	}

	if isQuery {
		s.WriteString("Q ")
	} else {
		s.WriteString("R ")
	}

	switch *m.Type {
	case Message_CLIENT_QUERY,
		Message_CLIENT_RESPONSE,
		Message_AUTH_QUERY,
		Message_AUTH_RESPONSE:
		printQueryAddress = true
	}

	if printQueryAddress {
		textConvertIP(s, m.QueryAddress)
	} else {
		textConvertIP(s, m.ResponseAddress)
	}
	s.WriteString(" ")

	if m.SocketProtocol != nil {
		s.WriteString(m.SocketProtocol.String())
	}
	s.WriteString(" ")

	var err error
	msg := new(dns.Msg)
	if isQuery {
		s.WriteString(strconv.Itoa(len(m.QueryMessage)))
		s.WriteString("b ")
		err = msg.Unpack(m.QueryMessage)
	} else {
		s.WriteString(strconv.Itoa(len(m.ResponseMessage)))
		s.WriteString("b ")
		err = msg.Unpack(m.ResponseMessage)
	}

	if err != nil {
		s.WriteString("X ")
	} else {
		s.WriteString("\"" + msg.Question[0].Name + "\" ")
		s.WriteString(dns.Class(msg.Question[0].Qclass).String() + " ")
		s.WriteString(dns.Type(msg.Question[0].Qtype).String())
	}

	s.WriteString("\n")
}

func TextFormat(dt *Dnstap) (out []byte, ok bool) {
	var s bytes.Buffer

	if *dt.Type == Dnstap_MESSAGE {
		textConvertMessage(dt.Message, &s)
		return s.Bytes(), true
	}

	return nil, false
}
