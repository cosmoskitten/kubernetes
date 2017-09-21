# Kubernetes Conformance Test Suite - v1.7

## Summary
This document provides a summary of the tests included in the Kubernetes conformance test suite.
Each test lists a set of formal requirements that a conformance platform must adhere to.

The tests are a subset of the "e2e" tests that make up the Kubernetes testing infrastructure. 
Each test is identified by the presence of the `[Conformance]` keyword in the ginkgo descriptive function calls.
The contents of this document are extracted from comments preceding those `[Conformance]` keywords
and those comments are expected to include a descriptive overview of what the test is validating using
RFC2119 keywords. This will provide a clear distinction between which bits of code in the tests are
there for the purposes of validating the platform rather than simply infrastructure logic used to setup, or
clean up, the tests.

