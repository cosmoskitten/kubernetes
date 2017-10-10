package(default_visibility = ["//visibility:public"])

licenses(["notice"])

load("@io_bazel_rules_docker//docker:docker.bzl", "docker_build", "docker_push")

docker_build(
    name = "image",
    base = "@debian//image",
    entrypoint = ["/kubemark"],
    files = [":kubemark"],
)

docker_push(
    name = "push",
    image = ":image",
    registry = "gcr.io",
    repository = "google_containers/kubemark",
    tag = "latest",
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
