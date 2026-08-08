# AGENTS.md

### Project

Nina is a CLI AI pair-programming companion for people learning to code: the AI navigates and reviews, the human types. See `nina-design-doc.md` for the original design rationale (it predates the Python port and still describes the Go stack that was retired) and `README.md` for a project overview.

The implementation lives entirely under [`python/`](python/) — see [`python/AGENTS.md`](python/AGENTS.md) for the stack, layout, build/check commands, and working conventions. There is no other implementation in this repo.

### Phased Development Workflow

* When creating an implementation plan, break the work into small, independently deliverable phases. Each phase should result in a working, testable increment.
* Commit all completed changes at the end of every phase using a clear, descriptive commit message before moving on to the next phase.

### Testing

* Add or update tests for non-trivial behaviour changes.
* Use a test-first approach whenever possible.

### Naming

* Use clear, consistent, descriptive names; avoid unnecessary abbreviations unless they are widely understood.
