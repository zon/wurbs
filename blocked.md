# Blocked: End-to-end tests for REST server

I am unable to implement end-to-end tests for the REST server because the server requires a functional PostgreSQL database connection to start, and there is no PostgreSQL instance available in the current environment.

## Attempts made:
1.  **Analyzed the code:** Examined `rest/main.go`, `core/pg/pg.go`, and the configuration loading mechanism to understand how the database connection is initialized.
2.  **Environment investigation:** Checked the system for running PostgreSQL processes or docker containers (`ps aux`, `docker ps`), but found none.
3.  **Reviewed documentation:** Checked `docs/testing.md` and the existing `projects/rest-server.yaml` to confirm the requirements and constraints.
4.  **Researched dependencies:** Checked `go.mod` to see if there were any alternative in-memory databases (SQLite was explicitly removed, so it is not an option).
5.  **Analyzed test behavior:** Investigated `core/pg/pg_test.go` and determined that `gorm.Open` is lazy and does not fail on invalid connection strings, but the actual REST server (`rest/main.go`) checks for error, so it crashes if a connection cannot be established.

## Why it didn't work:
The server's entry point (`rest/main.go`) hard-exits if `pg.Open()` fails. Without a PostgreSQL instance or an alternative in-memory driver, the server cannot start. The prohibition on adding new dependencies (like SQLite) limits the options for a mockable database within the testing environment. Mocking the database layer in the server code itself would require significant structural changes or writing a custom GORM dialect, which is not feasible within the scope of this task.
