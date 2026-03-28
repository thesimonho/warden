# Code Quality

## After editing files

Run the relevant quality checks after making changes. Do not leave formatting, lint, or type errors behind.

If `just` is installed, run `just format check test` after making changes. Otherwise, the individual commands can be found in @justfile

Before a PR or merge, also run `just test-e2e-matrix` to make sure both runtimes pass.

Always fix issues before committing.
