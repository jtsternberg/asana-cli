---
description: "Create a new release with changelog, git tag, and GoReleaser"
argument-hint: "[version] - optional, auto-detected if omitted"
allowed-tools: Bash(git *), Bash(gh release *), Bash(go build *), Bash(go test *), Bash(goreleaser *), Read, Edit, AskUserQuestion
---

Create a new release. Version can be provided as $ARGUMENTS, or auto-detected from commits.

## Steps

1. **Determine version**:
   - Get current version: `git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"`
   - Get commits since last tag: `git log $(git describe --tags --abbrev=0 2>/dev/null || echo "")..HEAD --oneline`
   - If $ARGUMENTS provided, use that version (ensure it has a `v` prefix)
   - Otherwise, analyze commits to suggest version bump:
     - **MAJOR**: commits with "BREAKING", "breaking change", removed commands/options
     - **MINOR**: commits with "add", "new", "feature", "feat:"
     - **PATCH**: commits with "fix", "bug", "patch", "docs", "chore", or any other changes
   - Present the suggested version and let user confirm or override

2. **Run pre-release checks**:
   - `go build ./...` — must compile
   - `go test ./...` — all tests must pass
   - Verify git status is clean (all changes committed)
   - If any check fails, stop and report the failure

3. **Generate changelog content**:
   - Analyze commits since last tag and categorize under: Added, Changed, Fixed, Removed
   - Format as markdown following Keep a Changelog format
   - Store this content to reuse in CHANGELOG.md and GitHub release

4. **Update CHANGELOG.md**:
   - Create the file if it doesn't exist, with a header: `# Changelog`
   - Insert new section `## [X.Y.Z] - YYYY-MM-DD` (use today's date)
   - Include the generated changelog content from step 3

5. **Update plugin version**:
   - Update the `"version"` field in `claude-plugin/.claude-plugin/plugin.json` to `X.Y.Z` (without the `v` prefix)

6. **Commit and tag**:
   ```bash
   git add CHANGELOG.md claude-plugin/.claude-plugin/plugin.json
   git commit -m "Prepare release vX.Y.Z"
   git tag vX.Y.Z
   ```

7. **Push** (ask for confirmation first):
   ```bash
   git push origin main --tags
   ```

8. **Run GoReleaser** (ask for confirmation first):
   - Check if goreleaser is installed: `which goreleaser || brew install goreleaser`
   ```bash
   goreleaser release --clean
   ```
   - This builds cross-platform binaries, creates archives, and publishes a GitHub release
   - If goreleaser fails, fall back to manual GitHub release:
     ```bash
     gh release create vX.Y.Z --title "vX.Y.Z" --notes "$CHANGELOG_CONTENT"
     ```

9. **Update README** if this is the first release:
   - Add back installation methods that depend on releases (pre-built binaries, install script)
