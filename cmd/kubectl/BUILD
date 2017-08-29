load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["kubectl.go"],
    visibility = ["//visibility:private"],
    deps = ["//cmd/kubectl/app:go_default_library"],
)

go_binary(
    name = "kubectl",
    library = ":go_default_library",
    visibility = ["//visibility:public"],
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
        "//cmd/kubectl/app:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
