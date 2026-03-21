I am completely blocked because I cannot run the project's tests. The environment is configured with `CGO_ENABLED=0` and lacks a C compiler (`gcc`), while the tests depend on `go-sqlite3` which requires CGO and a C compiler to function correctly. This prevents me from verifying any changes I make, including compliance with the requirement that REST server tests do not connect directly to the database.

I tried running `go test ./...` which failed with an error about missing C compiler. I also tried setting `CGO_ENABLED=1` but it failed with "executable file not found in $PATH" for "gcc".
