# Blocked

I am unable to implement the end-to-end tests for the REST server because the current environment is configured with `CGO_ENABLED=0`, which prevents the `gorm.io/driver/sqlite` driver (which depends on `go-sqlite3`, a CGO-based SQLite driver) from running. The environment also lacks a C compiler (`gcc`), so I cannot enable CGO.

I attempted to use the existing testing patterns in `rest/rest_test.go`, but they are designed to skip DB-dependent tests in this environment.

To make progress, I need an environment that either supports CGO or provides an alternative way to run tests against a database (e.g., using `modernc.org/sqlite` or an external PostgreSQL instance).
