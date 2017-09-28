load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["internal_container_lifecycle.go"],
    visibility = ["//visibility:public"],
    deps = [
        "//pkg/features:go_default_library",
        "//pkg/kubelet/cm/cpumanager:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/feature:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "container_manager_unsupported_test.go",
        "device_plugin_handler_test.go",
    ] + select({
        "@io_bazel_rules_go//go/platform:linux_amd64": [
            "cgroup_manager_linux_test.go",
            "cgroup_manager_test.go",
            "container_manager_linux_test.go",
            "helpers_linux_test.go",
            "node_container_manager_test.go",
        ],
        "//conditions:default": [],
    }),
    library = ":go_default_library",
    deps = [
        "//pkg/kubelet/apis/deviceplugin/v1alpha1:go_default_library",
        "//pkg/util/mount:go_default_library",
        "//vendor/github.com/stretchr/testify/assert:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/api/resource:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/sets:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/uuid:go_default_library",
    ] + select({
        "@io_bazel_rules_go//go/platform:linux_amd64": [
            "//pkg/kubelet/apis/kubeletconfig:go_default_library",
            "//pkg/kubelet/eviction/api:go_default_library",
            "//vendor/github.com/stretchr/testify/require:go_default_library",
        ],
        "//conditions:default": [],
    }),
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [
        ":package-srcs",
        "//pkg/kubelet/cm/cgroupmanager:all-srcs",
        "//pkg/kubelet/cm/containerlifecycle:all-srcs",
        "//pkg/kubelet/cm/containermanager:all-srcs",
        "//pkg/kubelet/cm/cpumanager:all-srcs",
        "//pkg/kubelet/cm/cpuset:all-srcs",
        "//pkg/kubelet/cm/deviceplugin:all-srcs",
        "//pkg/kubelet/cm/helper:all-srcs",
        "//pkg/kubelet/cm/podcontainer:all-srcs",
        "//pkg/kubelet/cm/qoscontainer:all-srcs",
        "//pkg/kubelet/cm/types:all-srcs",
        "//pkg/kubelet/cm/util:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
