---
name: git-workflow
description: Git and GitHub workflow best practices. Use for commits, branches, PRs, merging, and version control operations.
---

# Git Workflow Skill

Git best practices for version control, branching strategies, and GitHub collaboration.

## When to Use This Skill

- Creating commits with proper messages
- Managing branches
- Creating and reviewing pull requests
- Resolving merge conflicts
- Git history management

---

# 📝 Commit Messages

## Conventional Commits Format

```
<type>(<scope>): <subject>

[optional body]

[optional footer(s)]
```

## Commit Types

| Type | Description |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation only |
| `style` | Formatting, no code change |
| `refactor` | Code restructuring |
| `perf` | Performance improvement |
| `test` | Adding/updating tests |
| `chore` | Maintenance tasks |
| `ci` | CI/CD changes |
| `build` | Build system changes |

## Examples

```bash
# Feature
feat(auth): add JWT token refresh mechanism

# Bug fix
fix(proxy): resolve connection leak in upstream handler

# With body
feat(api): add rate limiting for /v1/chat endpoint

Implements token bucket algorithm with configurable
rate and burst size. Limits are per API key.

Closes #123

# Breaking change
feat(config)!: change yaml config structure

BREAKING CHANGE: config.yaml format changed.
See migration guide in docs/migration-v2.md
```

---

# 🌳 Branching Strategy

## Git Flow

```
main (production)
  │
  └── develop (integration)
        │
        ├── feature/user-auth
        ├── feature/dashboard
        ├── fix/login-bug
        └── release/v1.2.0
              │
              └── hotfix/critical-fix → main
```

## Branch Naming

```bash
# Features
feature/add-user-authentication
feature/JIRA-123-dashboard-charts

# Bug fixes
fix/login-validation-error
fix/JIRA-456-memory-leak

# Releases
release/v1.2.0
release/2024-01-sprint

# Hotfixes
hotfix/critical-security-patch
```

---

# 🔄 Pull Request Workflow

## Creating a PR

```bash
# 1. Create feature branch
git checkout -b feature/my-feature develop

# 2. Make commits
git add .
git commit -m "feat: implement feature"

# 3. Push and create PR
git push -u origin feature/my-feature

# 4. Create PR via GitHub CLI
gh pr create --title "feat: implement feature" \
  --body "Description of changes" \
  --base develop
```

## PR Template

```markdown
## Summary
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Changes Made
- Change 1
- Change 2

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests pass
- [ ] Manual testing completed

## Screenshots (if UI changes)

## Checklist
- [ ] Code follows style guidelines
- [ ] Self-reviewed
- [ ] Comments added for complex logic
- [ ] Documentation updated
```

## Merging Strategies

```bash
# Squash merge (recommended for features)
# Combines all commits into one clean commit
gh pr merge --squash

# Merge commit (for releases)
# Preserves full history
gh pr merge --merge

# Rebase (for small changes)
# Linear history, no merge commit
gh pr merge --rebase
```

---

# 🔧 Common Git Operations

## Stashing Changes

```bash
# Save current work
git stash save "WIP: feature description"

# List stashes
git stash list

# Apply most recent stash
git stash pop

# Apply specific stash
git stash apply stash@{2}

# Clear all stashes
git stash clear
```

## Rebasing

```bash
# Interactive rebase last 5 commits
git rebase -i HEAD~5

# Rebase onto main
git rebase main

# Continue after resolving conflicts
git rebase --continue

# Abort rebase
git rebase --abort
```

## Cherry-picking

```bash
# Apply specific commit to current branch
git cherry-pick abc123

# Cherry-pick without committing
git cherry-pick --no-commit abc123

# Cherry-pick range
git cherry-pick abc123..def456
```

## Undoing Changes

```bash
# Undo last commit (keep changes)
git reset --soft HEAD~1

# Undo last commit (discard changes)
git reset --hard HEAD~1

# Revert a commit (creates new commit)
git revert abc123

# Restore file to last commit
git checkout -- filename

# Unstage file
git reset HEAD filename
```

---

# 🔀 Merge Conflict Resolution

## Process

```bash
# 1. Update your branch
git fetch origin
git rebase origin/main

# 2. If conflicts occur, resolve them
# Edit conflicting files, then:
git add resolved-file.go
git rebase --continue

# 3. Force push (if rebased)
git push --force-with-lease
```

## Conflict Markers

```
<<<<<<< HEAD
Your changes
=======
Their changes
>>>>>>> feature-branch
```

## Tips

- Keep changes focused and PRs small
- Rebase frequently to avoid large conflicts
- Use `git mergetool` for complex conflicts
- Communicate with team before force pushing

---

# 📊 Git History Management

## Viewing History

```bash
# Pretty log
git log --oneline --graph --all

# Show changes in commits
git log -p -3

# Search commits by message
git log --grep="fix"

# Show commits by author
git log --author="name"

# Show files changed
git log --stat
```

## Finding Bugs with Bisect

```bash
# Start bisect
git bisect start

# Mark current as bad
git bisect bad

# Mark known good commit
git bisect good v1.0.0

# Git checks out middle commit, test it, then:
git bisect good  # or git bisect bad

# When done
git bisect reset
```

---

# 🏷️ Tagging Releases

```bash
# Create annotated tag
git tag -a v1.2.0 -m "Release version 1.2.0"

# Push tags
git push origin v1.2.0
git push --tags

# List tags
git tag -l "v1.*"

# Delete tag
git tag -d v1.2.0
git push origin --delete v1.2.0
```

---

# 📚 References

- [Conventional Commits](https://www.conventionalcommits.org/)
- [Git Flow](https://nvie.com/posts/a-successful-git-branching-model/)
- [fvadicamo/dev-agent-skills](https://github.com/fvadicamo/dev-agent-skills)
- [obra/finishing-a-development-branch](https://github.com/obra/superpowers/blob/main/skills/finishing-a-development-branch/SKILL.md)
- [obra/using-git-worktrees](https://github.com/obra/superpowers/blob/main/skills/using-git-worktrees/SKILL.md)
