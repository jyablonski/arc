# arc AI Usage

<img width="691" height="407" alt="Image" src="https://github.com/user-attachments/assets/16fb0381-8db3-440a-80e1-0a7440f96efc" />

`arc ai usage` shows quota / usage for Claude Code (Anthropic), OpenAI Codex, and Cursor. Providers run **in parallel**; one failure does not block the others.

**Authentication model:** `arc` does not log you into any provider (no passwords, OAuth browser flows, or sign-up APIs). It only **reads** credentials already on disk (or keychain) from **Claude Code**, the **Codex CLI**, or **Cursor**, or talks to a **local** `codex app-server` process that reads the same stores. Network traffic is **only** “call vendor usage surfaces with those saved tokens.”

## Credential source vs metrics source

| Provider   | Credential source                                                                                                                                                                 | Re-use vs login                                                                                             | Metrics source                                                                                                                                                                                                                                                            | Network?                             |
| ---------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------ |
| **Claude** | `~/.claude/.credentials.json` (`claudeAiOauth.accessToken`); macOS also Keychain **`Claude Code-credentials`**.                                                                   | **Re-use** (Claude Code wrote this when you signed in). `arc` reads only; never writes tokens to its cache. | `GET https://api.anthropic.com/api/oauth/usage` (OAuth quota JSON; unofficial).                                                                                                                                                                                           | Yes                                  |
| **Codex**  | **`codex app-server`** reads `~/.codex/` (`auth.json`, `config.toml`) and/or OS keyring (`cli_auth_credentials_store`). `arc` never parses tokens.                                | **Re-use** (you ran **`codex login`** beforehand).                                                          | JSON-RPC **`account/rateLimits/read`** inside the App Server protocol (implementation hits OpenAI’s Codex backends).                                                                                                                                                      | Yes (from the child `codex` process) |
| **Cursor** | SQLite **`state.vscdb`** → `cursorAuth/accessToken`. Linux `~/.config/Cursor/User/globalStorage/…`, macOS `~/Library/Application Support/Cursor/…`, Windows `%APPDATA%\Cursor\…`. | **Re-use** (Cursor IDE wrote this after you signed in).                                                     | Primary: **`POST https://api2.cursor.sh`** `…/DashboardService/GetCurrentPeriodUsage` and `…/GetPlanInfo`. Fallback: **`GET https://cursor.com/api/usage`**, optionally **`GET …/auth/stripe`** when the usage payload has no buckets. Undocumented dashboard-style APIs. | Yes                                  |

**Summary:** credentials = **local**; numbers = **live** vendor/quota endpoints (often private).

## Per-run “call budget” (what `arc` initiates)

Counting convention: **0** separate “login” calls from `arc` (credentials are always local reads or inherited by `codex`). **1** = one session/handshake-style step **or** one distinct HTTP/RPC aimed at quotas/metrics/auxiliary dashboard data.

| Provider   | Login / handshake                                            | Quota / metrics / auxiliary                                                                                                                                                                                                                                                                                                                 | Typical total (`arc`-initiated)                                             | Notes                                                                                                                |
| ---------- | ------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| **Claude** | **0** (read token file/keychain only)                        | **1×** HTTP `GET …/api/oauth/usage`                                                                                                                                                                                                                                                                                                         | **1** HTTPS to Anthropic                                                    | No separate auth request.                                                                                            |
| **Codex**  | **1×** JSON-RPC **`initialize`** to local `codex app-server` | **1×** JSON-RPC **`account/rateLimits/read`**                                                                                                                                                                                                                                                                                               | **2** stdin/stdout RPC round-trips to the child                             | The **`codex`** binary may perform **additional** HTTPS to OpenAI *inside* that process; `arc` does not count those. |
| **Cursor** | **0** (read SQLite only)                                     | **Happy path:** **1×** `GetCurrentPeriodUsage` + **1×** `GetPlanInfo`. **REST path:** **1×** `GET …/usage` (+ **0–1×** `GET …/auth/stripe` if usage JSON has no countable buckets). **Hybrid:** dashboard tried first (**2×** POST `api2`) then REST (**+1–2×** `cursor.com`) when rules say to fall back — up to **4** HTTPS in that case. | **2** (`api2` only), **1–2** (REST only), or **up to ~4** (api2 then REST). | Helpers run **concurrently** with other providers, so wall-clock ≠ sum of sequential calls.                          |

**Whole command (all three, cache miss):** expect on the order of **~5–8** outward vendor touches from your machine counting the above (`arc`-visible **1 + 2 + 2 … 4`). Cached runs (**default**, full command, within TTL): **0** vendor calls until the cache expires.

## Command

```bash
arc ai usage              # all providers
arc ai usage -j           # JSON
arc ai usage --provider claude,codex
arc ai usage --no-cache
```

Terminal output is a compact **table per provider**: `window`, a dot usage bar (filled portion = **quota remaining**), a **`% left`** column (**100 − `percent_used`**, clamped), and **resets** as a relative time. It does **not** print footnotes from `detail` or echo `report.extra` (those remain in **`--json` / `-j`** only).

- `--provider`: cache skipped (always live counts above apply per selected provider).
- Exit code: non-zero only if every selected provider fails.

## Cache

- File: `os.UserCacheDir()/arc/ai-usage.json` (e.g. `~/.cache/arc/ai-usage.json` on Linux).
- TTL: **45s**. Cached content is **aggregated numbers only** — not secrets.
- With default cache + full run: TTL window reuse avoids repeating the vendor calls.

## How often to run

- Within **45s** + default cache + full run → mostly **reads cache**, minimal vendor chatter.
- Live runs: Claude’s usage URL can **429** if polled hard; spacing **~1–3+ min** helps if you bypass cache often.
