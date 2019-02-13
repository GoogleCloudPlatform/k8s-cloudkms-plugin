package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "generated.pb.go",
        "group_version.go",
        "interfaces.go",
    ],
    importmap = "https:/github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/vendor/k8s.io/apimachinery/pkg/runtime/schema",
    importpath = "k8s.io/apimachinery/pkg/runtime/schema",
    deps = ["//vendor/github.com/gogo/protobuf/proto:go_default_library"],
)
