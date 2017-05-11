Testing
=======

## Automated tests

We're using Ginkgo and Gomega for testing our code. Since we're making extensive use of interfaces, external dependencies are mocked for all unit tests.

### Unit Tests
We'll try to keep testing coverage as high as possible. To run unit tests simply use:

```
make unit
```

To check the test coverage use:

```
make test-coverage-html  # opens a html file
```

or

```
make test-coverage-func  # prints code coverage to the console
```

### Integration Tests

TO BE DONE.

We also have a few integration tests that actually connect to maestro external dependencies. For these you'll need to have docker installed.

To run integration tests run:

```
make integration
```
