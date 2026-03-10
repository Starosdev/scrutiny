# Sync Branches

Synchronize develop and master branches with local and remote repositories.

**CRITICAL:** Only fetch from `origin`. The `upstream` and `venku` remotes exist for historical reference only -- never fetch from or push to them.

## Step 1: Check Initial State

1. **Check current branch and save it for later:**

   ```bash
   ORIGINAL_BRANCH=$(git branch --show-current)
   echo "Current branch: $ORIGINAL_BRANCH"
   ```

2. **Check working tree status:**

   ```bash
   git status --short
   ```

   If uncommitted changes exist, ask the user:

   ```
   You have uncommitted changes:
   [list files]

   Options:
   1. Commit them now
   2. Stash them
   3. Discard them
   4. Abort sync

   What should I do?
   ```

3. **Show branch relationships:**

   ```bash
   git log --oneline --graph --all --decorate -10
   ```

## Step 2: Fetch from Origin

```bash
git fetch origin --prune
```

Verify remote branches exist:

```bash
git branch -r | grep origin
```

Confirm `origin/master` and `origin/develop` are present. Ignore any `upstream/` or `venku/` branches.

## Step 3: Sync Master

1. **Switch to master and pull:**

   ```bash
   git checkout master
   git pull origin master
   ```

2. **Verify:**

   ```bash
   git status
   ```

   Should say "up to date with 'origin/master'".

3. **Push any local-only commits (if needed):**

   ```bash
   git push origin master
   ```

   Only if local master has commits not on remote.

## Step 4: Sync Develop

1. **Switch to develop and pull:**

   ```bash
   git checkout develop
   git pull origin develop
   ```

2. **Check if develop is behind master:**

   ```bash
   git log develop..master --oneline
   ```

   If master has commits not in develop, **first check for a squash-merge artifact** before merging:

   ```bash
   git diff origin/master origin/develop --stat
   ```

   **If the diff is empty (no file differences):** develop's commits are already in master via a squash merge — their content is identical but the hashes differ. Do NOT merge. Instead, reset develop to master:

   ```bash
   # Temporarily allow force-push via GitHub API (if branch protection is enabled)
   gh api repos/Starosdev/scrutiny/branches/develop/protection \
     --method PUT \
     --field required_status_checks=null \
     --field enforce_admins=false \
     --field required_pull_request_reviews=null \
     --field restrictions=null \
     --field allow_force_pushes=true

   git reset --hard origin/master
   git push --force origin develop

   # Re-enable branch protection
   gh api repos/Starosdev/scrutiny/branches/develop/protection \
     --method PUT \
     --field required_status_checks=null \
     --field enforce_admins=false \
     --field required_pull_request_reviews=null \
     --field restrictions=null \
     --field allow_force_pushes=false
   ```

   **If the diff is non-empty (real file differences):** master has genuine new commits (hotfix or direct-to-master PR). Merge master into develop:

   ```bash
   git merge master --no-edit
   git push origin develop
   ```

3. **Check if develop is ahead of master:**

   ```bash
   git log master..develop --oneline
   ```

   This is normal during development. Report the count but do not merge.

4. **Push any local-only commits (if needed):**

   ```bash
   git push origin develop
   ```

## Step 5: Verify Synchronization

1. **Check tracking relationships:**

   ```bash
   git branch -vv
   ```

2. **Compare branches:**

   ```bash
   # Commits in develop but not master
   git log master..develop --oneline | wc -l

   # Commits in master but not develop (should be 0)
   git log develop..master --oneline | wc -l
   ```

Present results:

```
Branch Sync Complete

Master:
  Local: Up to date with origin/master
  Last commit: <hash> <message>

Develop:
  Local: Up to date with origin/develop
  Ahead of master by X commits (or "in sync")

Branch Relationship:
  Develop is X commits ahead of master (or "in sync")
  Master has no commits not in develop [OK]

Current Branch: <original branch>
Working Tree: Clean
```

## Step 6: Return to Original Branch

Switch back to whatever branch the user was on before sync started:

```bash
git checkout $ORIGINAL_BRANCH
```

## Conflict Resolution

If conflicts occur when merging master into develop:

1. **Identify conflicted files:**

   ```bash
   git status
   ```

2. **Ask the user how to resolve each conflict:**

   **Option A: Accept master's version (theirs)**
   ```bash
   git checkout --theirs <file>
   git add <file>
   ```

   **Option B: Keep develop's version (ours)**
   ```bash
   git checkout --ours <file>
   git add <file>
   ```

   **Option C: Manual resolution**
   - Open file and resolve conflicts between `<<<<<<<` and `>>>>>>>`
   - Remove conflict markers
   - `git add <file>`

3. **Complete the merge:**

   ```bash
   git commit -m "merge: resolve conflicts from master into develop"
   git push origin develop
   ```

## Advanced: Force Sync (Dangerous)

Only use if local and remote are completely out of sync and local commits can be discarded.

**Reset develop to match remote:**
```bash
git checkout develop
git fetch origin
git reset --hard origin/develop
```

**Reset master to match remote:**
```bash
git checkout master
git fetch origin
git reset --hard origin/master
```

WARNING: This discards all local commits not on the remote!

## Common Scenarios

### Scenario 1: Develop Ahead of Master (Normal)

```
Develop: A - B - C - D - E
Master:  A - B - C

This is normal during development.
Master will catch up after production deployment.
```

### Scenario 2: Master Ahead of Develop — Real Changes (Problem)

```
Develop: A - B - C
Master:  A - B - C - D

git diff origin/master origin/develop --stat => non-empty

Master has genuine new commits (hotfix or direct-to-master PR).
Action: Merge master into develop immediately.
```

### Scenario 3: Branches Diverged (Problem)

```
Develop: A - B - C - D
Master:  A - B - E - F

This happens if commits were made directly to master (hotfixes, direct-to-master PRs).
Action: Merge master into develop to reconcile.
```

### Scenario 4: Squash-Merge Artifact (Looks Like Problem, Is Not)

```
Develop: A - B - C - D - E  (original feature commits)
Master:  A - B - S          (S = squash of B..E, different hash)

git log master..develop --oneline => shows B, C, D, E
git diff origin/master origin/develop --stat => EMPTY (no file differences)

Develop "appears" ahead of master but all content is identical.
These are ghost commits left over from a squash merge — do NOT merge master
into develop (that would bring S into develop, creating divergence forever).
Action: Reset develop to master using force-push.
```

## Quick Status Check

```bash
git fetch origin && \
git log master..develop --oneline | wc -l && \
git log develop..master --oneline | wc -l
```

## When to Use

- Before starting new work
- After merging PRs to master
- When develop falls behind master
- Before creating feature branches
- After direct-to-master hotfixes or PRs
- When branch state is unclear
