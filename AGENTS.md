# Agent instructions

## Naming

- Follow established Go conventions where they apply.
- Prefer complete, immediately understandable names over abbreviations.
- Use an abbreviation only when it is the clearer established Go term, such as `HTTP`, `URL`, or `ID`.
- Name commands and concepts so a reader can understand their purpose without knowing Unix shorthand or project history.

## Behaviour-driven development

- Define each change as an observable user, domain, or protocol behaviour before implementing it.
- Implement one small vertical behaviour slice at a time.
- Cover important cross-component journeys with a few BDD-style end-to-end tests using clear Given, When, and Then structure.
- Use the project's existing test tools. Do not introduce a BDD framework unless a present need justifies it.
- Test outcomes and public contracts rather than implementation details.

## Test-driven development

Use RED, GREEN, REFACTOR for every feature, bug fix, refactor, and behaviour change:

1. **RED:** Write one minimal automated test for the next behaviour before changing production code.
2. Run that specific test and confirm it fails for the expected reason, not because the test is broken.
3. **GREEN:** Write only enough production code to make that test pass.
4. Run the specific test again, then run the relevant wider suite to detect regressions.
5. **REFACTOR:** Improve the code only while all tests remain green.
6. Repeat with the next behaviour rather than writing all tests or all implementation at once.

If production code was written first, remove it and restart from the failing test. Tests added after implementation are regression coverage, not TDD. Exceptions for throwaway experiments, generated code, or configuration-only changes require explicit agreement and must be stated in the final report.

Prefer real collaborators and externally visible results. Use mocks only where a real boundary would be impractical, unsafe, or non-deterministic.

## Verification

- Run `npm run build` before any Go command in a fresh checkout. The Go server deliberately fails to compile when its embedded frontend is missing.
- During RED and GREEN, report the specific test command and its observed result.
- Before completion, run `npm test`, `npm run typecheck`, and `npm run build` for frontend or JavaScript changes.
- Before completion, run `go test -race ./...` and `go vet ./...` for Go changes.
- Run all applicable commands for cross-stack changes.
- Do not claim TDD unless the RED failure was actually observed.