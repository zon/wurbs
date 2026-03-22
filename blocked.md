# REST Server E2E Test Blockers

The REST server handlers are tightly coupled to `*gorm.DB` and directly call GORM methods, which requires a real PostgreSQL database driver at runtime for the server (even in test mode).

The current test environment has the following constraints:
1. No Docker or container orchestration is available to spin up a temporary Postgres instance.
2. The `rest/handler/` packages depend directly on `*gorm.DB` struct (not an interface), making it difficult to inject a mock DB without architectural changes.
3. Adding SQLite (a common in-memory DB alternative) is restricted for this project.

### Attempted Solutions
1. Attempted to initialize the REST server using the `--test` flag, hoping it would allow a mock database, but it only affects the configuration directory.
2. Investigated potential for in-memory database drivers for Postgres, none of which are compatible without architectural changes.
3. Reviewed handler implementations; all depend directly on `*gorm.DB` with no abstraction layer to allow dependency injection of a mock database.

### Conclusion
We are blocked on implementing E2E tests for the REST server until a viable strategy for mocking database interactions is identified (e.g., refactoring handlers to use an interface) or a supported in-memory test database is approved for use.
