load("@bazel_gazelle//:def.bzl", "gazelle")

# gazelle:prefix https://github.com/GoogleCloudPlatform/k8s-cloudkms-plugin
# gazelle:resolve go github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/plugin //plugin:go_default_library
# gazelle:resolve go github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/testutils/kmspluginclient //testutils/kmspluginclient:go_default_library
# gazelle:resolve go github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/testutils/fakekms //testutils/fakekms:go_default_library
# gazelle:resolve go github.com/GoogleCloudPlatform/k8s-cloudkms-plugin/testutils/fakekubeapi //testutils/fakekubeapi:go_default_library

# gazelle:resolve go k8s.io/apimachinery/pkg/api/resource //vendor/k8s.io/apimachinery/pkg/api/resource:go_default_library
# gazelle:resolve go k8s.io/apimachinery/pkg/runtime //vendor/k8s.io/apimachinery/pkg/runtime:go_default_library
# gazelle:resolve go k8s.io/apimachinery/pkg/selection //vendor/k8s.io/apimachinery/pkg/selection:go_default_library
# gazelle:resolve go k8s.io/apimachinery/pkg/watch //vendor/k8s.io/apimachinery/pkg/watch:go_default_library
# gazelle:resolve go k8s.io/apimachinery/pkg/label //vendor/k8s.io/apimachinery/pkg/label:go_default_library
# gazelle:resolve go k8s.io/apimachinery/pkg/fields //vendor/k8s.io/apimachinery/pkg/fields:go_default_library

# gazelle:resolve go k8s.io/apimachinery/pkg/util/sets //vendor/k8s.io/apimachinery/pkg/util/sets:go_default_library
# gazelle:resolve go k8s.io/apimachinery/pkg/util/runtime //vendor/k8s.io/apimachinery/pkg/util/runtime:go_default_library
# gazelle:resolve go k8s.io/apimachinery/pkg/util/validation //vendor/k8s.io/apimachinery/pkg/util/validation:go_default_library
# gazelle:resolve go k8s.io/apimachinery/pkg/util/json //vendor/k8s.io/apimachinery/pkg/util/json:go_default_library
# gazelle:resolve go k8s.io/apimachinery/pkg/util/errors //vendor/k8s.io/apimachinery/pkg/util/errors:go_default_library
# gazelle:resolve go k8s.io/apimachinery/pkg/util/net //vendor/k8s.io/apimachinery/pkg/util/net:go_default_library

# gazelle:resolve go k8s.io/apimachinery/third_party/forked/golang/reflect //vendor/k8s.io/apimachinery/third_party/forked/golang/reflect:go_default_library

gazelle(name = "gazelle")
