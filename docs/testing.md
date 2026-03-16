# Testing

## wurbctl

`wurbctl` should only have unit tests that can run without side effects — no integration tests, no end-to-end tests. Tests must not require a running database, Kubernetes cluster, or any other external service.
