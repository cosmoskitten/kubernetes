package(default_visibility = ["//visibility:public"])

licenses(["notice"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = [
        "backup.go",
        "k8sutil.go",
        "pod_util.go",
        "self_hosted.go",
        "tls.go",
        "tpr_util.go",
    ],
    tags = ["automanaged"],
    deps = [
        "//vendor/github.com/coreos/etcd-operator/pkg/backup/backupapi:go_default_library",
        "//vendor/github.com/coreos/etcd-operator/pkg/backup/env:go_default_library",
        "//vendor/github.com/coreos/etcd-operator/pkg/backup/s3/s3config:go_default_library",
        "//vendor/github.com/coreos/etcd-operator/pkg/spec:go_default_library",
        "//vendor/github.com/coreos/etcd-operator/pkg/util/constants:go_default_library",
        "//vendor/github.com/coreos/etcd-operator/pkg/util/etcdutil:go_default_library",
        "//vendor/github.com/coreos/etcd-operator/pkg/util/retryutil:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/api/errors:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/api/resource:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/labels:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/intstr:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/strategicpatch:go_default_library",
        "//vendor/k8s.io/client-go/kubernetes:go_default_library",
        "//vendor/k8s.io/client-go/pkg/api:go_default_library",
        "//vendor/k8s.io/client-go/pkg/api/v1:go_default_library",
        "//vendor/k8s.io/client-go/pkg/apis/apps/v1beta1:go_default_library",
        "//vendor/k8s.io/client-go/pkg/apis/storage/v1beta1:go_default_library",
        "//vendor/k8s.io/client-go/plugin/pkg/client/auth/gcp:go_default_library",
        "//vendor/k8s.io/client-go/rest:go_default_library",
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
