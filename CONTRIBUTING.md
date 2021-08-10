# Contributing
Thanks for your interest in contributing to Maestro. This document will contain
all the necessary information about contributing and some project overview to help
you get started.

# Code of conduct
Maestro follows the [CNFN Code of Conduct](https://github.com/cncf/foundation/blob/master/code-of-conduct.md).

# Assignees
After submitting an issue or pull request, one of the members will be assigned
to it. The designated member will be responsible for:
* Reviewing the request;
* Providing clarification on why the request changes were made or why the
  submission was closed/refused;
* Requesting review from other members, if necessary;
* Assigning the request to another member if they are not able to complete it;

## Answer latency
Currently, we have a small number of members responsible for reviewing all
requests made, so it might take some time to start looking at it.

# Reporting issues
We use GitHub issues tracking to track our issues (questions, bugs, and
feature requests).

## Questions
Even if it is not standard on most open source projects to use the issues
tracking for questions, we're doing it mostly because our volume of questions is
low. We might change this flow if we have any problem with this approach.

While asking questions, remember to give as much information as possible and
make it strict to the point. If you have multiple questions unrelated to each
other, feel free to open various issues. While opening an issue, choose the
"Question" template.

## Reporting bugs
We provide a template for opening a bug report, and please fill it with the
necessary information. Having a well-described bug with examples of reproducing
it helps the core team get into it a lot.

## Feature requests
There is also a template for it. Fill in the sections to help us understand what
you're requesting. Every request will be analyzed and prioritized by the
members. You could also open a PR with your feature request implementation.

# Opening pull requests
There is no necessity of opening an issue before submitting a Pull request, but
we'll require it to have as much detail as an issue of the same type.
In addition, we provide templates for enhancements and bug fixes.

## Workflow
Every pull request is required to pass on our entire workflow executed on
Github Actions. The workflow consists on:
1. Run the lint;
    * golangci-lint, for Golang code;
    * buf (TODO), for Protobuf files;
    * License check for ensuring files have the license head;
2. Execute unit tests;
3. Execute integration tests;

## Before submitting
This is a checklist to do before submitting your pull request to reduce workflow
failures and change requests:

* Perform the generate, to update the generated files based on your changes:
`make generate`
* Run tests locally:
    * `make run/unit-tests`
    * `make run/integration-tests`
* Run go imports before committing (`make goimports`)
* Run lint (`make lint`)

## Best practices
Here is a small list of best practices while developing to Maestro:

* [Reviewers take into count some common Golang style mistakes](https://github.com/golang/go/wiki/CodeReviewComments);
* Keep your pull request small and focused on one bug or feature. It is
  preferable to open multiple requests to complete a feature, instead of a large
  containing everything;
* [Comment your code](http://blog.golang.org/godoc-documenting-go-code). It makes it easier for the reviewer to understand why the
  code is there and what is being done;
* Keep the documentation up-to-date. If you are changing a code that has
  documentation related to it, also remember to update it;
* [Write good git commit messages](https://chris.beams.io/posts/git-commit/). We don't enforce any style, but we recommend
  using [conventional commits](https://www.conventionalcommits.org/en/v1.0.0/);
* Always test your code. If you're fixing a bug, adding a test case that
  reproduces it prevents it from appearing again;

## Code review and approvals
The Pull requests must have at least one approval from the members before being
merged. The assigned member will be the one responsible for approving and
merging the request.

# Code
Overall the project follows some [hexagonal architecture principles](https://en.wikipedia.org/wiki/Hexagonal_architecture_(software)).

## Project structure
Here is a basic overview of how the project packages are structured.
```
internal/
  adapters/    - Implementation of the Ports interfaces.
  core/
    entities/  - Package containing all the entities (little to no logic on this package)
    ports/     - Definition of the system ports interfaces.
    services/  - Business logic that uses entities and ports.
```

## Tools we use
Here is a list of tools that we use on the project:

* [Buf](https://github.com/bufbuild/buf): To compile the Protobuffers/gRPC to Golang;
* [Wire](https://github.com/google/wire): Used to define dependency injection;
* [golangci-lint](https://github.com/golangci/golangci-lint): Lint solution;
* [golang-migrate](https://github.com/golang-migrate/migrate): To perform relational database migrations;
* [goimports](https://pkg.go.dev/golang.org/x/tools/cmd/goimports): Perform code format;
* [Go mock](https://github.com/golang/mock): To generate mocks based on interfaces;
* [addlicense](https://github.com/google/addlicense): Check and add license header to files;

## Code generation
All code generation is specified on a separated package `gen` located on the
project root. It is executed using the target `make generate`.

## Tests
### Unit
Our unit tests are tagged with `unit`. To unit test services that use ports,
we use mocks.

### Integration
Most of the integration tests are placed on the adapters. We use a library
called [gnomock](https://github.com/orlangure/gnomock) to spin the test dependencies inside the test (not depending on
a docker-compose or some other solution).

### e2e
TODO

## Logging
TODO

## Tracing
TODO
