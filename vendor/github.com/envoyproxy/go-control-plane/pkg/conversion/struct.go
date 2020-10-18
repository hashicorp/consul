// Copyright 2018 Envoyproxy Authors
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

// Package conversion contains shared utility functions for converting xDS resources.
package conversion

import (
	"bytes"
	"errors"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	pstruct "github.com/golang/protobuf/ptypes/struct"
)

// MessageToStruct encodes a protobuf Message into a Struct. Hilariously, it
// uses JSON as the intermediary
// author:glen@turbinelabs.io
func MessageToStruct(msg proto.Message) (*pstruct.Struct, error) {
	if msg == nil {
		return nil, errors.New("nil message")
	}

	buf := &bytes.Buffer{}
	if err := (&jsonpb.Marshaler{OrigName: true}).Marshal(buf, msg); err != nil {
		return nil, err
	}

	pbs := &pstruct.Struct{}
	if err := jsonpb.Unmarshal(buf, pbs); err != nil {
		return nil, err
	}

	return pbs, nil
}

// StructToMessage decodes a protobuf Message from a Struct.
func StructToMessage(pbst *pstruct.Struct, out proto.Message) error {
	if pbst == nil {
		return errors.New("nil struct")
	}

	buf := &bytes.Buffer{}
	if err := (&jsonpb.Marshaler{OrigName: true}).Marshal(buf, pbst); err != nil {
		return err
	}

	return jsonpb.Unmarshal(buf, out)
}
