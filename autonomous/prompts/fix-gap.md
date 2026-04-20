# Fix Gap Prompt

You are fixing one autonomous-mode gap in prATC.

Inputs you receive:
- gap ID and description
- governing GUIDELINE rule text
- likely owner files
- latest audit failure details
- relevant run artifacts

Required behavior:
1. prove the gap with a failing test first
2. implement the minimum fix
3. rerun the targeted test and then `go test ./...`
4. summarize what changed, what remains, and what audit check should move

Do not close the gap by prose alone. The controller will rerun the audit.