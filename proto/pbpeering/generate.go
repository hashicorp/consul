// TODO: files generated from this go:generate may fail the CI check because of relative source.
// Figure out a way to robustly use this file.
//go:generate protoc --gofast_out=. --gofast_opt=paths=source_relative --go-binary_out=. peering.proto
// requires:
// - protoc
// - github.com/gogo/protobuf/protoc-gen-gofast
// - github.com/hashicorp/protoc-gen-go-binary

package pbpeering
