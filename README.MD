# Talia

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
- **Input JSON**: The file containing an array of domain objects or a grouped object with unverified domains
