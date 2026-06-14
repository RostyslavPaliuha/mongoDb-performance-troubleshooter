# Future Releases (in progress)

### Repository preparation and stabilization
 - provide versioning
 - add docs
 - code style

### Installation and execution (Q2 2026)
  - semi-manual installation with go compiler
### Minimum valuable functionality(Q3 2026)
 - mongoDB 3.6 support

### Installation and execution (Q3 2026)
- Planned distribution targets:
  - Homebrew for macOS.
  - GitHub Releases with prebuilt binaries for macOS.
  - Checksums for every published binary.
  - Automated release builds from Git tags.

The release pipeline should avoid asking users to install Go, update `PATH`
manually, or build from source for normal usage.

`go install` should remain available for Go developers, but it should not be the
main documented installation path once package-manager distribution exists.
