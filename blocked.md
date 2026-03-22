# Blocked

I am attempting to implement E2E tests for the REST server, but I am unable to connect to a database for testing. The project explicitly forbids using SQLite as a dependency, and there is no database available in the test environment.

The REST server (specifically the handlers) requires a `*gorm.DB` instance, which is normally created by connecting to a Postgres database. I am blocked on implementing these tests without the ability to either mock the database connection at the GORM level or use an in-memory database driver, both of which are not currently feasible in this environment.

I have tried setting up a `config/postgres.json` with dummy values, but the server naturally fails to connect to the database.
