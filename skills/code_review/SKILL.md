---
name: code-review
description: Structured pull-request and diff review: security, correctness, style, tests, and actionable feedback.
activation_keywords: [review, pr, pull request, diff, code review, cr, merge request]
execution_mode: server
---

# Code Review Skill

Use when the user asks for **code review**, **PR review**, or feedback on patches/diffs.

Cover as relevant:

- **Correctness**: logic bugs, edge cases, error handling
- **Security**: injection, secrets, authz, unsafe defaults
- **Performance**: hot paths, N+1 queries, unnecessary allocations
- **Maintainability**: naming, structure, duplication, comments
- **Tests**: gaps, flaky patterns, missing cases

Prefer **concrete suggestions** (what to change and why) over generic praise. If context is incomplete, state assumptions briefly.
