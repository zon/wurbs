# Blocked: End-to-end testing for REST server

## Problem
I am unable to implement end-to-end tests for the REST server because it requires a database connection (GORM + Postgres driver). 

## Constraints
1. I cannot connect to a real Postgres instance.
2. I cannot use SQLite (as it's a forbidden dependency).
3. I cannot add new dependencies like `go-sqlmock` to `go.mod`.

## What I tried
1. Analyzed `rest/main.go` and `core/pg/pg.go` to understand the database initialization.
2. Attempted to create a simple HTTP health check test in `rest/e2e_test.go` to verify I can run the server in test mode, which succeeded.
3. Investigated GORM mocking strategies (`sqlmock`), but it is not available in the project and I am restricted from adding dependencies.
4. Explored `core/config/dir.go` for test mode settings, but it only affects configuration file resolution, not database initialization.

The REST server fundamentally requires a functional GORM database connection, and without a way to mock it in-process, I cannot satisfy the requirement to run E2E tests against a server that "does not connect directly to the database" if that implies the server itself shouldn't connect to one. If the server *is* allowed to connect to a mock database, I lack the tools to create such a mock.
