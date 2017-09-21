# Kubernetes Conformance Test Suite - v1.7

## Summary
This document provides a summary of the tests included in the Kubernetes conformance test suite.
Each test lists a set of formal requirements that a platform that meets conformance requirements  must adhere to.

The tests are a subset of the "e2e" tests that make up the Kubernetes testing infrastructure.
Each test is identified by the presence of the `[Conformance]` keyword in the ginkgo descriptive function calls.
The contents of this document are extracted from comments preceding those `[Conformance]` keywords
and those comments are expected to include a descriptive overview of what the test is validating using
RFC2119 keywords. This will provide a clear distinction between which bits of code in the tests are
there for the purposes of validating the platform rather than simply infrastructure logic used to setup, or
clean up the tests.

Example:
/*
  Testname: Kubelet-OutputToLogs
  Description: By default the stdout and stderr from the process
           being executed in a pod MUST be sent to the pod's logs.
*/
// Note this test needs to be fixed to also test for stderr
It("it should print the output to logs [Conformance]", func() {

would generate the following documentation for the test. Note that the "TestName" from the Documentation above will
be used to document the test which make it more human readable. The "Description" field will be used as the
documentation for that test.

## [Kubelet-OutputToLogs](https://github.com/kubernetes/kubernetes/tree/master/test/e2e_node/kubelet_test.go#L48)

By default the stdout and stderr from the process
being executed in a pod MUST be sent to the pod's logs.
Note this test needs to be fixed to also test for stderr
