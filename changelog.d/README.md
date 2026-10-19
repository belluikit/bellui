# changelog.d: one file per change

A change that users would notice adds **one file here** instead of editing
`CHANGELOG.md`. Two pull requests that add different files never conflict,
where two that edited the Unreleased section always did (#511). The release
folds the files into a section of `CHANGELOG.md` and deletes them.

Changes to the `bellui` command go in
[`cmd/bellui/changelog.d/`](../cmd/bellui/changelog.d/) instead; the command
has its own changelog and its own tags.

## Naming

    changelog.d/<issue>.<category>.md

- `<issue>`: the issue the change closes. A change that closes none uses its
  pull request's number.
- `<category>`: one of `security`, `added`, `changed`, `deprecated`,
  `removed`, `fixed`. Anything else is an error, not a new heading.
- A change with two entries in one category adds a counter:
  `427.fixed-2.md`.

Entries are listed by category, then by number with the highest first.

## Contents

One bullet in the changelog's usual style, with no heading:

```markdown
* **Typing into a large text field no longer leaks memory**
  ([#508](https://github.com/belluikit/bellui/issues/508)). Say what users
  saw, what changed, and anything deliberately left as it was.
```

## Commands

    go run ./hack/changelog check                           # validate every fragment
    go run ./hack/changelog preview [-C cmd/bellui]         # print the Unreleased section
    go run ./hack/changelog release [-C cmd/bellui] vX.Y.Z  # fold into CHANGELOG.md, delete fragments

CI runs `check`. It does not require a pull request to add a fragment.
