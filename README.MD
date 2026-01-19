# Talia

[![CI](https://github.com/sustanza/talia/actions/workflows/ci.yml/badge.svg)](https://github.com/sustanza/talia/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/sustanza/talia?logo=github&sort=semver)](https://github.com/sustanza/talia/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/sustanza/talia.svg)](https://pkg.go.dev/github.com/sustanza/talia)

Talia is a lightweight command-line application for checking the availability of domains (e.g. `.com`) by querying a user-specified WHOIS server. It processes a JSON file containing domain records and updates them with WHOIS responses (availability, reason, logs), or optionally produces a separate grouped output.

## Table of Contents

- [Overview](#overview)
- [Disclaimer](#disclaimer)
- [Installation](#installation)
- [Usage](#usage)
  - [Required WHOIS Server](#required-whois-server)
  - [JSON Input Format](#json-input-format)
  - [Domain Checks and Rate Limiting](#domain-checks-and-rate-limiting)
  - [Optional Sleep Flag](#optional-sleep-flag)
  - [Generating Domain Suggestions](#generating-domain-suggestions)
  - [Merging Domain Files](#merging-domain-files)
  - [Cleaning Domain Files](#cleaning-domain-files)
  - [Verbose Mode](#verbose-mode)
  - [Grouped Output](#grouped-output)
  - [Unverified Domains in Grouped Files](#unverified-domains-in-grouped-files)
- [How It Works](#how-it-works)
- [Testing](#testing)
- [Configuration](#configuration)
- [Contributing](#contributing)
- [License](#license)

---

## Overview

Talia reads a list of domains from a JSON file and connects to a **user-specified** WHOIS server to check each domain's availability. After each check, Talia can:

1. **Non-grouped mode (default)**:  
   Talia updates the **original file** in an array format, for example:

   [
   {
   "domain": "filterguard.com",
   "available": false,
   "reason": "TAKEN"
   },
   {
   "domain": "vetterhub.com",
   "available": true,
   "reason": "NO_MATCH"
   }
   ]

2. **Grouped mode** (`--grouped-output`):

   - **No `--output-file`**: Talia **overwrites** the input file with a single JSON object containing `"available"` and `"unavailable"` arrays, each filled with `{ "domain": "...", "reason": "..." }`. For example:

     {
     "available": [
     {
     "domain": "vetterhub.com",
     "reason": "NO_MATCH"
     }
     ],
     "unavailable": [
     {
     "domain": "filterguard.com",
     "reason": "TAKEN"
     }
     ]
     }

   - **With `--output-file`**: Talia leaves the original file alone and writes/merges a grouped JSON file at the specified path:
     {
     "available": [
     { "domain": "vetterhub.com", "reason": "NO_MATCH" }
     ],
     "unavailable": [
     { "domain": "filterguard.com", "reason": "TAKEN" }
     ]
     }

---

## Disclaimer

- **No Default WHOIS Server**: You must specify a WHOIS server via `--whois=<host:port>`. If you use `whois.verisign-grs.com:43` or similar, confirm you comply with its TOS.
- **Accuracy**: Talia identifies availability by scanning for the substring `"No match for"`. This may not work for every TLD or WHOIS provider.

## Installation

### Prerequisites

- Go 1.16 or later
- golangci‑lint v2.1.6 (declared as a Go tool; Go 1.24+ will auto‑build it on first use)

### Steps

    # Clone the repository:
    git clone https://github.com/sustanza/talia.git
    cd talia

    # Build Talia:
    go build -o talia

---

## Usage

### Required WHOIS Server

You **must** specify a WHOIS server:

    ./talia --whois=whois.example.com:43 path/to/domains.json

If you omit this flag, Talia exits with an error.

### JSON Input Format

Your JSON file should be an array of objects, each having a `"domain"` field, e.g.:

    [
      { "domain": "filterguard.com" },
      { "domain": "vetterhub.com" }
    ]

Alternatively, you can now use a grouped file with an `"unverified"` array as input (see [Unverified Domains in Grouped Files](#unverified-domains-in-grouped-files)).

### Domain Checks and Rate Limiting

By default, Talia sleeps 2 seconds (`--sleep=2s`) between each domain WHOIS query to reduce server load. You can override this with:

    ./talia --whois=whois.example.com:43 --sleep=500ms path/to/domains.json

### Optional Sleep Flag

As shown above, `--sleep` changes how long Talia waits between domain checks.

### Generating Domain Suggestions

Talia can use AI (OpenAI, Gemini, or other OpenAI-compatible APIs) to generate domain name ideas. This feature requires an API key and internet access.

- **API Key Requirement:** Set the `OPENAI_API_KEY` environment variable with your API key, or add it to a `.env` file in the current directory (Talia loads `.env` automatically).
- **Network Access:** The machine running Talia must be able to reach the API endpoint.
- **Alternative APIs:** Use `--api-base` or the `OPENAI_API_BASE` environment variable to point to OpenAI-compatible APIs like Google Gemini.

Example `.env` file:
```
OPENAI_API_KEY=your-api-key
OPENAI_API_BASE=https://generativelanguage.googleapis.com/v1beta/openai
```

Use the `--suggest` flag to generate domain ideas. If you also provide `--whois`, Talia will automatically verify the suggestions via WHOIS (with a fast 100ms delay between checks). Use `--no-verify` to skip auto-verification and just write suggestions. You can change the model with `--model`.

    # Generate and auto-verify suggestions
    OPENAI_API_KEY=... ./talia --suggest=5 --whois=whois.verisign-grs.com:43 --prompt="three word tech names" suggestions.json

    # Generate suggestions only (no verification)
    OPENAI_API_KEY=... ./talia --suggest=5 --no-verify --prompt="short brandable names" suggestions.json

    # Use Google Gemini
    OPENAI_API_KEY=... ./talia --suggest=5 --whois=whois.verisign-grs.com:43 --api-base="https://generativelanguage.googleapis.com/v1beta/openai" --model=gemini-3.0-flash suggestions.json

**Important:** Talia will instruct the AI to only return domain names ending in `.com`. Talia automatically validates and normalizes all suggestions, fixing common issues like repeated `.com` suffixes (e.g., `example.com.com.com`) and double dots. Domains that can't be normalized to valid `name.com` format are silently discarded.

**Example output (`suggestions.json`):**

```json
{
  "unverified": [
    { "domain": "techtriplewave.com" },
    { "domain": "cloudbytefusion.com" },
    { "domain": "datastreampulse.com" },
    { "domain": "logicmeshzone.com" },
    { "domain": "pixelcorelabs.com" }
  ]
}
```

**If the API key is missing or the OpenAI request fails,** Talia will print an error and exit without writing suggestions.

**Tip:** You can add new suggestions to an existing grouped file by appending to the `"unverified"` array, then re-running Talia to check them.

### Merging Domain Files

Use `--merge` to combine domains from one file into another:

    ./talia --merge=source.json target.json

This merges all domains from `source.json` into `target.json`, preserving their categories (available, unavailable, unverified). Duplicates are automatically skipped.

### Cleaning Domain Files

Use `--clean` to sanitize an existing domain file. This normalizes all domains, removes invalid entries, and deduplicates:

    ./talia --clean suggestions.json

The clean operation:
- Converts domains to lowercase
- Fixes repeated `.com` suffixes (e.g., `example.com.com.com` → `example.com`)
- Removes double dots (e.g., `example..com` → `example.com`)
- Removes domains that don't match the `name.com` format
- Removes duplicates

Invalid domains are listed in the output so you can see what was removed.

### Verbose Mode

By default, Talia only writes the WHOIS response to the `"log"` field if there's an error. If you set `--verbose`, Talia stores the full WHOIS response for every domain:

    ./talia --whois=whois.example.com:43 --verbose path/to/domains.json

### Grouped Output

Use `--grouped-output` to produce a grouped JSON. The behavior depends on `--output-file`:

1. **No `--output-file`**: The input JSON file is overwritten with:

   {
   "available": [
   { "domain": "...", "reason": "NO_MATCH" }
   ],
   "unavailable": [
   { "domain": "...", "reason": "TAKEN" }
   ]
   }

2. **With `--output-file=somefile.json`**: The input file stays intact, and Talia writes/merges grouped results into `somefile.json`.

### Unverified Domains in Grouped Files

Talia now supports using a grouped file with an `"unverified"` array as input. This is useful for adding new domains to an existing grouped file:

```json
{
  "available": [{ "domain": "existing-available.com", "reason": "NO_MATCH" }],
  "unavailable": [{ "domain": "existing-unavailable.com", "reason": "TAKEN" }],
  "unverified": [
    { "domain": "new-domain1.com" },
    { "domain": "new-domain2.com" }
  ]
}
```

When Talia processes this file:

1. It checks all domains in the `"unverified"` array
2. Moves them into either `"available"` or `"unavailable"` based on WHOIS results
3. Clears the `"unverified"` array
4. Preserves existing domains in the `"available"` and `"unavailable"` arrays

This allows you to incrementally add and verify new domains while maintaining your existing results.

### Static Analysis (golangci‑lint)

To lint the codebase, run:

```bash
go tool golangci-lint run
```

Go automatically builds and caches the version pinned in `go.mod` the first time you invoke the command.  
If you already have a global `golangci-lint` on your PATH, you can instead run:

```bash
golangci-lint run
```

A clean, zero‑error run is required before submitting a pull request.

---

## How It Works

1. **Parse Input**: Talia reads the JSON from disk into Go structs.
2. **Check WHOIS**: For each domain, Talia queries the WHOIS server.
   - If "No match for" → `available=true, reason=NO_MATCH`
   - Otherwise → `available=false, reason=TAKEN`
   - If error → `available=false, reason=ERROR`
3. **Write**:
   - **Non-grouped**: Talia updates `[{"domain":"x","available":true,"reason":"NO_MATCH"},...]` in the same file.
   - **Grouped w/o output-file**: Talia overwrites your file with `{ "available": [...], "unavailable": [...] }`.
   - **Grouped w/ output-file**: Leaves input alone, merges or creates grouped JSON in the separate file.
   - **Grouped with unverified**: Processes unverified domains and moves them to the appropriate lists.

---

## Testing

    go test -v

Talia's test suite covers:

- **Unit Tests**: checking domain availability with mock WHOIS responses
- **Integration Tests**: verifying inline updates, grouped mode, merges, and file rewrites
- **Error Handling**: invalid JSON, read/write failures, missing WHOIS server, etc.

---

## Configuration

- **`--whois=<host:port>`**: Required WHOIS server
- **`--sleep=<duration>`**: Wait time between queries (default 2s)
- **`--verbose`**: Store WHOIS data in `"log"` even for successful checks
- **`--grouped-output`**: Switch to grouped output
- **`--output-file=<path>`**: If provided, Talia merges/writes grouped JSON to this file and leaves the input alone
- **`--suggest=<n>`**: Generate <n> domain suggestions instead of running WHOIS checks
- **`--prompt=<text>`**: Optional text to influence domain suggestion themes
- **`--model=<name>`**: Model to use when generating suggestions (default: gpt-4o)
- **`--api-base=<url>`**: Base URL for OpenAI-compatible API (default: https://api.openai.com/v1). Can also be set via `OPENAI_API_BASE` environment variable.
- **`--fresh`**: Don't pass existing domains to AI when generating suggestions (allows duplicates, starts fresh)
- **`--no-verify`**: Skip WHOIS verification after generating suggestions (by default, suggestions are verified if `--whois` is provided)
- **`--merge=<file>`**: Merge domains from another file into the target file
- **`--clean`**: Clean and normalize domains in the file (removes invalid domains, fixes malformed entries)
- **Input JSON**: The file containing an array of domain objects or a grouped object with unverified domains

---

## Contributing

Pull requests are welcome. All commits must follow the Conventional Commits
specification so that automated release tooling can parse them correctly.
A commit lint check runs in CI to enforce this format. Example prefix types
include `feat`, `fix`, `chore`, and `docs`.

## License

This project is licensed under the terms of the [MIT License](LICENSE).
