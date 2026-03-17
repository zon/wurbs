# Testing

## General Rules

Tests must not write local config files or apply any Kubernetes resources to any cluster.

## wurbctl

`wurbctl` should only have unit tests that can run without side effects — no integration tests, no end-to-end tests. Tests must not require a running database or any other external service.

