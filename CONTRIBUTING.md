# Contributing to Thruster

Thruster is a Go application. It is packaged as a Ruby gem to make it easy to use with Rails applications.


## Running the tests

You can run the test suite using the Makefile:

    make test

You can also run individual tests using Go's test runner. For example:

    go test -v -run ^TestVariantMatches_multiple_headers$ ./...


## Running & building the application

You can run the application using `go run`:

    go run ./cmd/thrust

You can also build for the current environment using the Makefile:

    make build

This will create a binary in the `bin/` directory.

To build binaries for all supported architectures and operating systems, use:

    make dist

This will create a `dist/` directory with binaries for each platform.


## Publishing a release

In order to ship the platform-specific binary inside a gem, we actually build
multiple gems, one for each platform. The `rake release` task will build all the
necessary gems.

The comlete steps for releasing a new version are:

- Update the version & changelog:
  - [ ] update `lib/thruster/version.rb`
  - [ ] update `CHANGELOG.md`
  - [ ] commit and create a git tag (prefix the version with `v`, e.g. `v0.1.0`)

- Build the native gems:
  - [ ] `rake clobber` (to clean up any old packages)
  - [ ] `rake package`

- Push gems:
  - [ ] `for g in pkg/*.gem ; do gem push $g ; done`
  - [ ] `git push`

