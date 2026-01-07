Study @SPEC.md
Study @TECH_SPECH.md

Run `tk ready` and pick the single highest priority task to work on.
Start the work on the task by running `tk start <id>` to get detailed task information.

Do not assume that something has been implemented, always compare
the current state of the code with the specification.

Implement the task, ensure all acceptance criteria are met, and all e2e tests are authored for new functionality.

All functioanlity should have tests that erorr handling works as expected.

Test names must be in the pattern of `Test_Foo_Does_Bar_When_Baz`

Each command gets its own test file, e.g. `create_test.go`, `list_test.go`, etc.

Run `make lint`, `make modernize` and `make test` frequently.

When you think you are done, run `make check` for comprehensive tests.

Then, complete the task by running `tk close <id>`, and then commit your changes and the ticket itself with git.
Use conventional commit messages, and reference the ticket in the first line of your commit message.

Then, pick the next task by running `tk ready` again. Repeat up to 5 times (5 tasks in total).
