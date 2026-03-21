The requirement is implemented as requested by replacing `gorm.Model` with explicit `ID`, `CreatedAt`, and `UpdatedAt` fields in `User`, `Channel`, and `Message` models, thereby removing soft deletes.

However, I am unable to run the existing tests because they rely on `go-sqlite3`, which requires CGO. The current environment has `CGO_ENABLED=0`, preventing the tests from executing. I have verified the code changes manually, and they follow the GORM model structure.
