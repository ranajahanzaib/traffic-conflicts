# Contributing

Contributions that improve the scientific methods, data provenance, accessibility, reliability, or documentation are welcome. For substantial changes, open an issue first to discuss scope and design.

## Development setup

Install Go 1.27 or later, Python 3.10 or later, Node.js 22 or later, and pnpm 12. Clone the repository and enter its directory. The Python pipeline uses only the standard library; Go and pnpm resolve their dependencies from the committed module and package lockfiles.

```sh
git clone https://github.com/ranajahanzaib/traffic-conflicts.git
cd traffic-conflicts
pnpm --dir apps/web install --frozen-lockfile
make dev
```

Run the local checks before submitting a change:

```sh
make test
make build
make validate
make frontend-lint
```

## Contribution guidelines

- Add or update tests for behavior changes.
- Preserve source annotations and provenance; keep unknown values unknown.
- Keep synthetic fixtures labeled and out of research aggregates.
- Do not commit raw datasets, credentials, private keys, or generated local indexes.
- Before adding or redistributing data, review its source-specific license, attribution, map terms, and derived-data permissions. The InterHub code license does not grant rights to its underlying source datasets.
- Update user-facing documentation when commands, configuration, API behavior, or scientific assumptions change.

Pull requests should explain the user-visible or scientific impact, note any compatibility changes, and report the checks that were run. Use the issue and pull request templates to provide reproducible details.
