# dil - Dependency Injection Linter

`dil` is a Go linter that detects places where external packages are used directly in functions. Inline dependencies make a function difficult to test in isolation. Instead, the function should accept an interface that defines the functionality it needs. This allows the function to be tested with a mock implementation of the interface.

## Installation

```shell
go install github.com/jefflinse/dil@latest
```

## Usage

```shell
dil <package-path>
```
