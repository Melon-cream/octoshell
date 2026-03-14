# Changelog

All notable changes to this project will be documented in this file.

The format follows Keep a Changelog, and versions are intended to follow Semantic Versioning.

## [0.1.0] - 2026-03-15

### Added

- Initial Go CLI for Octopus Energy Japan electricity usage retrieval
- GraphQL client with token authentication and email/password token acquisition
- Monthly aggregation based on `halfHourlyReadings`
- JSON and CSV output support
- Verbose GraphQL debugging with sensitive value redaction
- Static tests for CLI parsing, aggregation logic, and GraphQL error handling
- Public project documentation in English and Japanese
- GitHub Actions for push validation, release note synchronization, and binary release builds
