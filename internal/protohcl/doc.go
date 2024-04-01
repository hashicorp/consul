// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// The protohcl package aims to define a canonical way to translate between
// protobuf and HCL encodings of data.
//
// Similar to google.golang.org/protobuf/encoding/protojson it intends to be
// opinionated about what the canonical HCL representation of a protobuf type
// should be.
//
// As HCL is a user centric data format as opposed to JSON/Protobuf which
// are intended to be used more by machines, efficiency is not a primary goal
// of this package as it is expected that users are either doing the encoding to and
// decoding from HCL at the edge (such as within the Consul CLI) or that even
// when done on servers, the rate that servers perform these translations should
// be low enough to have any inefficiency produce a tangible performance impact.
//
// HCL has two different syntaxes that could be used to represent data: attribute and block
//
// This implementation chooses to represent primitive values, enums and the well known wrapper types and
// collections of these values (maps and repeated fields) with attribute syntax. Other messages and collections
// with message value types will be represented with block syntax for example
//
// message Foo {
//    map<string, int32> map_of_ints = 1;
//	   map<string, OtherMessage> map_of_messages = 2;
// }
//
// would have HCL like:
//
// map_of_ints = {
//    "foo": 1,
//    "bar": 2,
// }
// map_of_messages "foo" {
//    ...other fields
// }
// map_of_messages "bar" {
//    ...other fields
// }
//
// Similar goes for list of primitives vs list of messages (except the block syntax uses no labels). The differences
// between primitive fields outside of a collection and a message field really just amounts to not having to specify
// the "=" between the field name and the "{" character.
//
// Field Mapping
// | proto3                 | HCL Type      | example | notes                                                                           |
// |------------------------+---------------+---------+---------------------------------------------------------------------------------+
// | message                | Object        |         | Represented as a block                                                          |
// | enum                   | String        |         |                                                                                 |
// | map<K,V>               | Map           |         | All keys are converted to/from strings.                                         |
// | repeated V             | List          |         |                                                                                 |
// | bool                   | Bool          |         |                                                                                 |
// | string                 | String        |         |                                                                                 |
// | bytes                  | base64 String |         |                                                                                 |
// | int32, fixed32, uint32 | Number        |         |                                                                                 |
// | int64, fixed64, uint64 | Number        |         |                                                                                 |
// | float, double          | Number        |         |                                                                                 |
//
// ----- Well Known Types -----
//
// | Any                    | Object        |         |                                                                                 |
// | Timestamp              | String        |         | RFC 3339 compliant                                                              |
// | Duration               | String        |         | String form is what would be accepted by time.ParseDuration                     |
// | Struct                 | Map           |         |                                                                                 |
// | Empty                  | Object        |         | An object with no fields                                                        |
// | BoolValue              | Bool          |         | Mostly the same as a regular bool except null values in the HCL are preserved   |
// | BytesValue             | Bytes         |         | Mostly the same as a regular bytes except null values in the HCL are preserved  |
// | StringValue            | String        |         | Mostly the same as a regular string except null values in the HCL are preserved |
// | FloatValue             | Number        |         | Mostly the same as a regular float except null values in the HCL are preserved  |
// | DoubleValue            | Number        |         | Mostly the same as a regular double except null values in the HCL are preserved |
// | Int32Value             | Number        |         | Mostly the same as a regular int32 except null values in the HCL are preserved  |
// | UInt32Value            | Number        |         | Mostly the same as a regular uint32 except null values in the HCL are preserved |
// | Int64Value             | Number        |         | Mostly the same as a regular int64 except null values in the HCL are preserved  |
// | UInt64Value            | Number        |         | Mostly the same as a regular uint64 except null values in the HCL are preserved |
// | FieldMask              | String        |         | Each string of the FieldMask will be joined with a '.'                          |
//

package protohcl
