# Code Quality

## After editing files

Run the relevant quality checks after making changes. Do not leave formatting, lint, or type errors behind.

If `just` is installed, run `just format check test` after making changes. Otherwise, the individual commands can be found in @justfile

Also run `just test-e2e` to make sure container tests pass. These should pass within 5mins - if not, there is a failing test somewhere.

Always fix issues before committing.
