// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package testutils

import "google.golang.org/grpc/metadata"

type MockServerTransportStream struct {
	MD metadata.MD
}

func (m *MockServerTransportStream) Method() string {
	return ""
}

func (m *MockServerTransportStream) SetHeader(md metadata.MD) error {
	return nil
}

func (m *MockServerTransportStream) SendHeader(md metadata.MD) error {
	m.MD = metadata.Join(m.MD, md)
	return nil
}

func (m *MockServerTransportStream) SetTrailer(md metadata.MD) error {
	return nil
}
