package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_test(
    name = "go_default_test",
    srcs = [
        "taint_test.go",
        "toleration_test.go",
    ],
    embed = [":go_default_library"],
)

go_library(
    name = "go_default_library",
    srcs = [
        "annotation_key_constants.go",
        "doc.go",
        "generated.pb.go",
        "objectreference.go",
        "register.go",
        "resource.go",
        "taint.go",
        "toleration.go",
        "types.go",
        "types_swagger_doc_generated.go",
        "zz_generated.deepcopy.go",
    ],
    importmap = "https:/github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/vendor/k8s.io/api/core/v1",
    importpath = "k8s.io/api/core/v1",
    deps = [
        "//vendor/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/github.com/gogo/protobuf/proto:go_default_library",
        "//vendor/k8s.io/apimachiner/pkg/types:go_default_library",
        "@com_github_gogo_protobuf//sortkeys:go_default_library",
        "@io_k8s_apimachinery//pkg/api/resource:go_default_library",
        "@io_k8s_apimachinery//pkg/runtime:go_default_library",
        "@io_k8s_apimachinery//pkg/runtime/schema:go_default_library",
        "@io_k8s_apimachinery//pkg/util/intstr:go_default_library",
    ],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [":package-srcs"],
    tags = ["automanaged"],
)
