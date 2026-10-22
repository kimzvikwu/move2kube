# Threat model: move2kube

_Date: 2026-10-22 · Commit: 7012d98_

## Summary

move2kube is a CLI that reads an application's source code and writes Kubernetes and Docker files. The biggest risk is running it on a repository you don't trust. Two problems confirmed by running them can quietly copy secrets from your machine into the generated output, which people normally commit and push. With a remote `git+...` output path, move2kube pushes the output itself. The next three are confirmed by reading the code: an unauthenticated question server, git URLs that can escape the temp directory, and a gap in the Starlark sandbox.

## Remediation status (branch `fix/threat-model-hardening`)

| # | Finding | Status |
|---|---|---|
| 1 | Symlinks dereferenced into output | **Fixed.** Symlinks inside the source are recreated as links, and links that resolve outside the source are skipped with a warning. |
| 2 | Compose reads arbitrary host files | **Fixed** for `secrets.file`, `configs.file` (including symlinks inside config dirs) and `env_file`: paths that resolve outside the source are ignored. **Open:** `${VAR}` interpolation from the host environment is still the default (`--ignore-env` is opt-in), because that flag also controls git, registry and Q&A behaviour and flipping it is a product decision. |
| 3 | Q&A HTTP engine | **Fixed:** listens on 127.0.0.1 only, rejects non-loopback `Host` headers (DNS rebinding), requires `Content-Type: application/json` on POST (CSRF), and guards `currentProblem` with a mutex. **Open:** no per-run auth token, which needs a coordinated change in move2kube-api. |
| 4 | `git+` URL path traversal / `RemoveAll` | **Fixed.** `..` segments are rejected, and `Clone` checks containment before touching the filesystem. |
| 5 | Starlark `IsPathValid` symlink bypass | **Fixed.** Symlinks are resolved before the containment check. |
| 6 | Unbounded downloads | **Partly fixed:** 5 min timeout, 100 MiB limit, and a warning for plain `http://`. **Open:** default clone size is still unlimited. |
| 7 | `bash -c` in the cluster collector | **Fixed.** Uses argument lists. |

Verified end to end: running `move2kube transform --qa-skip` on a hostile sample (compose secret and `env_file` with absolute host paths, plus a symlink to a host file) leaked the host values into 16 generated files on `main` and into none on this branch.

## What it touches

**Assets at risk:** files on the host (SSH keys, cloud credentials, `~/.kube`), environment variables, registry/git/SSH passwords entered in Q&A, the generated output (often committed or pushed), and the host itself.

**Trust boundaries:**

| # | Boundary | Who controls the input |
|---|---|---|
| B1 | Source dir or `git+` source URL → parsers (compose, pom, Dockerfile, .csproj…) | Whoever wrote the repo being converted; **least trusted** |
| B2 | Customizations dir or URL → transformers (Starlark, executable, WASM) | Customization author. Running their code is by design, so they're effectively trusted. |
| B3 | Q&A HTTP engine (`--qa-port`) | Anyone who can reach the port |
| B4 | Remote fetches (git clone, `http.Get` config download) | The network or remote host |
| B5 | Output dir or remote `git+` output (auto push) | Where data leaves the machine |
| B6 | `collect` → kubectl/oc/docker | The cluster API |

## Findings, most severe first

### 1. Symlinks in the source repo are followed, and the target's contents are copied into the output (High, confirmed by running it)

`filesystem/processor.go:46` calls `os.Stat`, which follows symlinks, so the `os.ModeSymlink` branch at `:55` can never run. A repo containing `config.txt -> ~/.aws/credentials` gets the real credentials copied into the workspace and the output `source/` folder.

- **Verified:** running `Replicate` on such a link produced a regular file (`-rw-r--r--`) containing the target's text.
- A symlink to a directory (for example `/` or `$HOME`) is walked recursively. That leaks whole directories and can also exhaust disk space.
- **Fix:** use `os.Lstat`, then either recreate links relative to the source or refuse links whose resolved target is outside the source root.

### 2. Docker Compose files can read any host file and embed it in generated Secrets/ConfigMaps (High, confirmed in code)

- `transformer/compose/v3.go:453` and `:478–490`: `secrets.<x>.file` and `configs.<x>.file` go straight to `os.ReadFile`, or the whole directory is read. Absolute paths and `../` are accepted. Example: `secrets: {k: {file: /home/u/.ssh/id_rsa}}` ends up base64-encoded in `deploy/yamls/*-secret.yaml`.
- `transformer/compose/utils.go:336`: `${VAR}` placeholders are filled from **the whole host environment** unless `--ignore-env` is set. `environment: [X=${GITHUB_TOKEN}]` leaks the token into a Deployment manifest.
- **Fix:** only allow `file:` paths inside the source root, after resolving symlinks. Make `--ignore-env` the default, or require an explicit list of allowed variables.

### 3. The Q&A HTTP engine has no authentication and listens on every network interface (High when `--qa-port` is used)

- `qaengine/httprestengine.go:79`: it listens on `":"+port`, which means 0.0.0.0, not localhost as the log message says.
- Anyone on the network can read the current question and answer it before you do. That includes registry URLs (pointing the push scripts at their registry), image names, and password prompts (`registrypreprocessor.go:166`, `vcs/git.go:176`, `sshkeys.go:192`).
- Browser attack (CSRF/DNS rebinding): `:179` decodes JSON without checking `Content-Type`, so any web page can send a cross-origin `text/plain` POST. Question IDs are easy to guess.
- `currentProblem` is read and written from several handlers with no lock, which is a data race.
- **Fix:** bind to 127.0.0.1 by default, require a random per-run bearer token, require `application/json`, check the `Host` header, and add a mutex.

### 4. `git+` URLs can point outside the temp directory, and the target can be deleted (Medium, confirmed by running it)

- The regex at `common/vcs/git.go:56` accepts `/..` after the host. `GitRepoPath` is joined without cleaning or a containment check (`:217`).
- **Verified:** `git+https://github.com/../../../../tmp/victim` resolves to the clone path `/tmp/victim`.
- For source, output and customization paths `overwrite=true` (`lib/planner.go:36,40`, `lib/utils.go:35`), so `os.RemoveAll(repoPath)` (`:233`) **deletes that directory**. `PathWithinRepo` (`:86`) can also point outside the repo with `..`.
- For a single CLI user this mostly means hurting yourself. It becomes serious when the URL comes from a plan file or from a multi-user front end such as move2kube-api.
- **Fix:** reject `..` path segments and check that the resolved path stays inside `RemoteTempPath`.

### 5. The Starlark sandbox check ignores symlinks (Medium)

`environment/environment.go:507`: `IsPathValid` cleans the path as text and checks its prefix, but never resolves symlinks. `fs.write` and `fs.read` follow a symlink inside the source or output dir to anywhere on the host. Starlark customizations are partly trusted, but this undermines the only containment they have. A malicious source repo can also plant the symlink for a harmless transformer to write through.

- **Fix:** run `filepath.EvalSymlinks` before the parent check, or open files with an `O_NOFOLLOW`-style approach.

### 6. Fetching remote content has no limits (Medium/Low)

- `common/download/httpdownload.go:58`: plain `http.Get` with no timeout and no size limit, and `http://` is allowed. Remote Q&A config can be tampered with in transit or stall the run.
- `common/vcs/vcs.go:54`: `maxRepoCloneSize` defaults to -1 (unlimited).
- **Fix:** add timeouts, `io.LimitReader`, require HTTPS, and set a sensible default clone size.

### 7. Shell command built from cluster data in `collect` (Low)

`collector/clustercollector.go:580,601` runs `bash -c` with `kind`/`gvStr`/`name` taken from `kubectl api-resources` output. Kubernetes name validation limits this today, but switch to `exec.Command(cmd, args...)`.

### 8. Risky by design: worth documenting

- Customizations, including `git+` URLs, run arbitrary code (executable transformers on the local host, Starlark, WASM). Treat a customization URL like `curl | sh`.
- `--qa-persist-passwords` writes secrets in plain text to `m2kconfig.yaml`. It's opt-in, but that file tends to get committed.
- Remote output paths auto-push (`vcs/git.go:129`), which turns findings 1 and 2 into exfiltration with no human review step.

## Areas that held up

- Transformers are loaded only from the built-in assets and customizations, never from the source tree, so a source repo can't add its own transformers.
- Maven and Gradle are not run on the host during analysis; they only appear in generated Dockerfiles.
- Temp dirs use `MkdirTemp`, and a recent commit already recovers from panics in the compose parser.

## Suggested order

1 → 2 → 3 are the cheapest fixes that close the "convert an untrusted repo and leak secrets" scenario. Fix 4 before anything multi-user (like move2kube-api) passes URLs through.
