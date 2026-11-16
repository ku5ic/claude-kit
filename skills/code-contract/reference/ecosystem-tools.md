# Ecosystem API tools

Only a tool the project already has installed and configured. Never install one.

- TypeScript, api-extractor. `api-extractor run` writes the `.api.md` report, tracked in git, so the report's diff between base and head is the surface change.
- Rust, cargo-semver-checks. `cargo semver-checks --baseline-rev <rev>`. It has no lint for some breaks, so a clean run isn't proof.
- Go, gorelease. `gorelease -base=<version>` reports API changes and the lowest valid next version. apidiff calls a change incompatible when it could stop client code compiling.
- Python, griffe. `griffe check <package> -a <base>` reports removed objects and changed parameters and defaults.
- Java, japicmp. `--semantic-versioning` reports which part of the version to bump, and tells source from binary compatibility.
- .NET. `EnablePackageValidation` checks a package against its last stable release at pack time.
