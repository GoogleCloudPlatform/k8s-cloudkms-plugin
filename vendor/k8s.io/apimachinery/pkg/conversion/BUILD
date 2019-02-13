package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "converter.go",
        "deep_equal.go",
        "doc.go",
        "helper.go",
    ],
    importmap = "https:/github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/vendor/k8s.io/apimachinery/pkg/conversion",
    importpath = "k8s.io/apimachinery/pkg/conversion",
    deps = ["//vendor/k8s.io/apimachinery/third_party/forked/golang/reflect:go_default_library"],
)
