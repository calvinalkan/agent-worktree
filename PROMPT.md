Study @SPEC.md
Study @TECH_SPECH.md

Run `tk ready` and pick the single highest priority task to work on.
Tell the user what task you are picking (the title of the ticket).
Start the work on the task by running `tk start <id>` to get detailed task information.

Always compare the current state of the code with the specification.
If you think that a ticket is no longer applicable, you can close it and commit with a message
that explains why. If you discover new issues, you can create a new ticket (`tk create --help`)

Implement the task, ensure all acceptance criteria are met, and all e2e tests are authored for new functionality.

All functioanlity should have tests that erorr handling works as expected.

Test names must be in the pattern of `Test_Foo_Does_Bar_When_Baz`

Each command gets its own test file, e.g. `create_test.go`, `list_test.go`, etc.

Run `make lint`, `make modernize` and `make test` frequently.

When you think you are done, run `make check` for comprehensive tests.

Then, complete the task by running `tk close <id>`, and then commit your changes and the ticket itself with git.
Use conventional commit messages, and reference the ticket in the first line of your commit message.

Then tell the user what you accomplished as a quick tl;dr, and stop.
