load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "492c3ac68ed9dcf527a07e6a1b2dcbf199c6bf8b35517951467ac32e421c06c1",
    urls = ["https://github.com/bazelbuild/rules_go/releases/download/0.17.0/rules_go-0.17.0.tar.gz"],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "7949fc6cc17b5b191103e97481cf8889217263acf52e00b560683413af204fcb",
    urls = ["https://github.com/bazelbuild/bazel-gazelle/releases/download/0.16.0/bazel-gazelle-0.16.0.tar.gz"],
)

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "aed1c249d4ec8f703edddf35cbe9dfaca0b5f5ea6e4cd9e83e99f3b0d1136c3d",
    strip_prefix = "rules_docker-0.7.0",
    urls = ["https://github.com/bazelbuild/rules_docker/archive/v0.7.0.tar.gz"],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

new_local_repository(
    name = "gcfg",
    build_file = "BUILD.bazel",
    path = "//vendor/gopkg.in/gcfg.v1",
)

new_local_repository(
    name = "warnings",
    build_file = "BUILD.bazel",
    path = "//vendor/gopkg.in/warnings.v0",
)

new_local_repository(
    name = "inf",
    build_file = "BUILD.bazel",
    path = "//vendor/gopkg.in/inf.v0",
)

new_local_repository(
    name = "cmp",
    build_file = "BUILD.bazel",
    path = "//vendor/github.com/google/go-cmp/cmp",
)

new_local_repository(
    name = "freeport",
    build_file = "BUILD.bazel",
    path = "//vendor/github.com/phayes/freeport",
)

new_local_repository(
    name = "corev1",
    build_file = "BUILD.bazel",
    path = "//vendor/k8s.io/api/core/v1",
)

new_local_repository(
    name = "klog",
    build_file = "BUILD.bazel",
    path = "//vendor/k8s.io/klog",
)

new_local_repository(
    name = "gofuzz",
    build_file = "BUILD.bazel",
    path = "//vendor/github.com/google/gofuzz",
)

new_local_repository(
    name = "golgr",
    build_file = "BUILD.bazel",
    path = "//vendor/github.com/go-logr/logr",
)

new_local_repository(
    name = "gotpm",
    build_file = "BUILD.bazel",
    path = "//vendor/github.com/google/go-tpm",
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains()

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")

gazelle_dependencies()

load(
    "@io_bazel_rules_docker//repositories:repositories.bzl",
    container_repositories = "repositories",
)

container_repositories()

load(
    "@io_bazel_rules_docker//container:container.bzl",
    "container_pull",
    "container_push",
)

container_pull(
    name = "distroless_static",
    registry = "gcr.io",
    repository = "distroless/static",
    tag = "latest",
)

load(
    "@io_bazel_rules_docker//go:image.bzl",
    _go_image_repos = "repositories",
)

_go_image_repos()
