#!/usr/bin/env bash

SCRIPT_NAME="$(basename ${BASH_SOURCE[0]})"
pushd $(dirname ${BASH_SOURCE[0]}) > /dev/null
SCRIPT_DIR=$(pwd)
pushd ../.. > /dev/null
SOURCE_DIR=$(pwd)
popd > /dev/null
pushd ../functions > /dev/null
FN_DIR=$(pwd)
popd > /dev/null
popd > /dev/null

source "${SCRIPT_DIR}/functions.sh"

function usage {
cat <<-EOF
Usage: ${SCRIPT_NAME} [<options ...>] <proto filepath>

Description:
   Generate the Go files from protobuf definitions. In addition to
   running the protoc generator it will also fixup build tags in the
   generated code.

Options:
   --protoc-bin             Path to protoc.
   --import-replace         Replace imports of google types with those from the protobuf repo.
   --grpc                   Enable the gRPC plugin
   -h | --help              Print this help text.
EOF
}

function err_usage {
   err "$1"
   err ""
   err "$(usage)"
}

function main {
   local -i grpc=0
   local    proto_path=
   local    protoc_bin=

   while test $# -gt 0
   do
      case "$1" in
         -h | --help )
            usage
            return 0
            ;;
         --grpc )
            grpc=1
            shift
            ;;
         --protoc-bin )
            protoc_bin="$2"
            shift 2
            ;;
         * )
            proto_path="$1"
            shift
            ;;
      esac
   done

   if test -z "${proto_path}"
   then
      err_usage "ERROR: No proto file specified"
      return 1
   fi

   if test -z "${protoc_bin}"
   then
       protoc_bin="$(command -v protoc)"
       if test -z "${protoc_bin}"
       then
           err_usage "ERROR: no proto-bin specified and protoc could not be discovered"
           return 1
       fi
   fi


   go mod download

   local golang_proto_path=$(go list -f '{{ .Dir }}' -m github.com/golang/protobuf)
   local golang_proto_mod_path=$(sed -e 's,\(.*\)github.com.*,\1,' <<< "${golang_proto_path}")

   local proto_go_path=${proto_path%%.proto}.pb.go
   local proto_go_bin_path=${proto_path%%.proto}.pb.binary.go
   local proto_go_rpcglue_path=${proto_path%%.proto}.rpcglue.pb.go
   local mog_input_path="$(dirname "${proto_path}")"

   local go_proto_out="paths=source_relative"
   if is_set "${grpc}"
   then
      go_proto_out="${go_proto_out},plugins=grpc"
   fi

   if test -n "${go_proto_out}"
   then
      go_proto_out="${go_proto_out}:"
   fi

   rm -f "${proto_go_path}" ${proto_go_bin_path}" ${proto_go_rpcglue_path}" "${mog_input_path}/*.gen.go"

   # How we run protoc probably needs some documentation.
   #
   # This is the path to where
   #  -I="${golang_proto_path}/protobuf" \
   local -i ret=0
   status_stage "Generating ${proto_path} into ${proto_go_path} and ${proto_go_bin_path} ${mog_input_path}/*.gen.go"
   echo "debug_run ${protoc_bin} \
          -I=\"${golang_proto_path}\" \
          -I=\"${golang_proto_mod_path}\" \
          -I=\"${SOURCE_DIR}\" \
          --go_out=\"${go_proto_out}${SOURCE_DIR}\" \
          --go-binary_out=\"${SOURCE_DIR}\" \
          \"${proto_path}\""
   debug_run ${protoc_bin} \
      -I="${golang_proto_path}" \
      -I="${golang_proto_mod_path}" \
      -I="${SOURCE_DIR}" \
      --go_out="${go_proto_out}${SOURCE_DIR}" \
      --go-binary_out="${SOURCE_DIR}" \
      "${proto_path}"

   if test $? -ne 0
   then
      err "Failed to run protoc for ${proto_path}"
      return 1
   fi

   debug_run protoc-go-inject-tag \
   	   -input="${proto_go_path}"

   if test $? -ne 0
   then
      err "Failed to run protoc-go-inject-tag for ${proto_path}"
      return 1
   fi

   debug_run mog -source ./${mog_input_path} -tags ${GOTAGS} -ignore-package-load-errors

   if test $? -ne 0
   then
      err "Failed to generate mog outputs from ${mog_input_path}"
      return 1
   fi

   BUILD_TAGS=$(head -n 2 "${proto_path}" | grep '^//go:build\|// +build')
   if test -n "${BUILD_TAGS}"
   then
      echo -e "${BUILD_TAGS}\n" >> "${proto_go_bin_path}.new"
      cat "${proto_go_bin_path}" >> "${proto_go_bin_path}.new"
      mv "${proto_go_bin_path}.new" "${proto_go_bin_path}"
   fi

   # note: this has to run after we fix up the build tags above
   rm -f "${proto_go_rpcglue_path}"
   debug_run go run ./internal/tools/proto-gen-rpc-glue/main.go -path "${proto_go_path}"
   if test $? -ne 0
      then
         err "Failed to generate consul rpc glue outputs from ${proto_path}"
         return 1
   fi

   return 0
}

main "$@"
exit $?
