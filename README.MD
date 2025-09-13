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
  
  <!-- TODO(sustanza): Align this with go.mod (currently `go 1.24.3`) and Dockerfile base image (Go 1.24). -->
- golangci‑lint v2.1.6 (declared as a Go tool; Go 1.24+ will auto‑build it on first use)

### Steps

    # Clone the repository:
    git clone https://github.com/sustanza/talia.git
    cd talia

    # Build Talia:
    # TODO(sustanza): Add `cmd/talia/main.go` entrypoint and update build instructions
    # to `make build` or `go build -o talia ./cmd/talia` to match AGENTS.md structure.
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

Talia can use AI (OpenAI or compatible APIs) to generate domain name ideas. This feature requires an **OpenAI API key** and internet access.

- **API Key Requirement:** Set the `OPENAI_API_KEY` environment variable with your OpenAI API key. See [OpenAI API documentation](https://platform.openai.com/docs/api-reference/authentication) for details on obtaining a key.
- **Network Access:** The machine running Talia must be able to reach the OpenAI API endpoint.

Use the `--suggest` flag to create a JSON file containing domain ideas without running any WHOIS checks. The result is written in the same grouped format with an `"unverified"` array so you can immediately feed it back into Talia for checking. You can change the OpenAI model with `--model` if you have access to alternatives.

    OPENAI_API_KEY=... ./talia --suggest=5 --prompt="three word tech names ending in .com" --model=gpt-4o suggestions.json

**Important:** Talia will instruct the AI to only return domain names ending in `.com`. If the AI still returns domains without `.com`, Talia will not automatically fix them. You may need to adjust your prompt or manually add `.com` to the results. This will become more flexible for other TLDs in the future.

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

You can then run Talia with `--whois` against this file to verify the suggested domains:

    ./talia --whois=whois.example.com:43 suggestions.json

**If the API key is missing or the OpenAI request fails,** Talia will print an error and exit without writing suggestions.

**Tip:** You can add new suggestions to an existing grouped file by appending to the `"unverified"` array, then re-running Talia to check them.

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
- **`--model=<name>`**: OpenAI model to use when generating suggestions
- **Input JSON**: The file containing an array of domain objects or a grouped object with unverified domains

---

## Contributing

Pull requests are welcome. All commits must follow the Conventional Commits
specification so that automated release tooling can parse them correctly.
A commit lint check runs in CI to enforce this format. Example prefix types
include `feat`, `fix`, `chore`, and `docs`.

## License

This project is licensed under the terms of the [MIT License](LICENSE).
