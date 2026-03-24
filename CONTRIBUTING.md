# Contributing

## Commit convention

Use semantic commits in English following the Conventional Commits style:

```text
<type>(optional-scope): <short imperative summary>
```

Examples:

- `feat(api): add webhook signature validation`
- `fix(chatbot): prevent empty user messages`
- `docs(readme): clarify local swagger access`
- `refactor(usecase): extract reply generation flow`
- `test(chatbot): cover allowed phone number validation`
- `chore(gitignore): ignore coverage artifacts`

Recommended types:

- `feat`: new feature
- `fix`: bug fix
- `refactor`: internal code improvement without behavior change
- `docs`: documentation only
- `test`: tests only
- `chore`: maintenance, tooling, housekeeping
- `build`: build or dependency changes
- `ci`: CI pipeline changes

Rules:

- Write the message in English.
- Keep the summary short and imperative.
- Prefer one logical change per commit.
- Use a scope when it helps identify the area changed.
