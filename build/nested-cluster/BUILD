package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_docker//docker:docker.bzl", "docker_build", "docker_bundle")

docker_build(
    name = "nested-kubelet-build",
    base = "@nested-kubelet-base//image",
    debs = [
        "//build/debs:kubelet",
        "//build/debs:kubectl",
        "//build/debs:kubeadm",
    ],
    entrypoint = ["/init-wrapper.sh"],
    files = [
        "init-wrapper.sh",
        "start.sh",
        "//build:master-components-amd64",
    ],
)

docker_bundle(
    name = "nested-kubelet-bundle",
    images = {"gcr.io/google-containers/nested-kubelet:0.1": ":nested-kubelet-build"},
)

docker_build(
    name = "encapsulated-cluster-build",
    base = "nested-kubelet-build",
    files = [":nested-kubelet-bundle.tar"],
)

docker_bundle(
    name = "encapsulated-cluster-bundle",
    images = {"gcr.io/google-containers/encapsulated-cluster:0.1": "encapsulated-cluster-build"},
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
    visibility = ["//visibility:public"],
)
