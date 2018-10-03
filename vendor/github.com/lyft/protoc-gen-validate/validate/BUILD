# gazelle:exclude validate.pb.go

load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")
load("@com_google_protobuf//:protobuf.bzl", "py_proto_library")

proto_library(
    name = "validate_proto",
    srcs = ["validate.proto"],
    deps = [
        "@com_google_protobuf//:descriptor_proto",
        "@com_google_protobuf//:duration_proto",
        "@com_google_protobuf//:timestamp_proto",
    ],
    visibility = ["//visibility:public"],
)

cc_proto_library(
    name = "validate_cc",
    deps = [":validate_proto"],
    visibility = ["//visibility:public"],
)

py_proto_library(
    name = "validate_py",
    srcs = ["validate.proto"],
    protoc = "@com_google_protobuf//:protoc",
    default_runtime = "@com_google_protobuf//:protobuf_python",
    deps = ["@com_google_protobuf//:protobuf_python"],
    visibility = ["//visibility:public"],
)

go_proto_library(
    name = "go_default_library",
    proto = ":validate_proto",
    deps = [
        "@com_github_golang_protobuf//ptypes/duration:go_default_library",
        "@com_github_golang_protobuf//ptypes/timestamp:go_default_library",
        "@io_bazel_rules_go//proto/wkt:descriptor_go_proto",
    ],
    importpath = "github.com/lyft/protoc-gen-validate/validate",
    visibility = ["//visibility:public"],
)

cc_library(
    name = "cc_validate",
    hdrs = ["validate.h"],
    visibility = ["//visibility:public"],
)
