# Blocked: E2E REST Server Tests

## Impediment
The REST server requires a database connection to function, specifically using GORM. The end-to-end tests are required to run against a local dev server without connecting to a real database, and without adding forbidden dependencies like SQLite for testing purposes.

## Attempts
- Tried initializing the test server with an empty/unconfigured `*gorm.DB` struct.
- The handlers rely on GORM functions (`db.Create`, `db.First`, etc.) that require a configured database driver to function. Passing a nil or empty `*gorm.DB` results in panics or errors when these functions are called during API requests.
- Without SQLite as an in-memory database, and without the ability to use a real Postgres instance, it is not possible to satisfy the GORM dependency for the REST server handlers in an E2E test environment.

## Conclusion
We are unable to proceed with implementing E2E tests for the REST server while adhering to the constraint of no direct database connection and no SQLite dependency.
