# Talia

Talia is a lightweight command-line application for checking the availability of `.com` domains by querying a specified WHOIS server. It processes a JSON file containing domain records, updates that file with WHOIS responses, and annotates each domain record with an availability status **and a reason code**.

## Table of Contents

- [Overview](#overview)
- [Disclaimer](#disclaimer)
- [Installation](#installation)
- [Usage](#usage)
  - [JSON Input Format](#json-input-format)
  - [WHOIS Server Selection](#whois-server-selection)
  - [Domain Checks and Rate Limiting](#domain-checks-and-rate-limiting)
  - [Optional Sleep Flag](#optional-sleep-flag)
  - [Verbose Mode](#verbose-mode)
- [How It Works](#how-it-works)
  - [Domain Checking Flow](#domain-checking-flow)
  - [Availability Determination](#availability-determination)
  - [File Updates](#file-updates)
- [Testing](#testing)
- [Configuration](#configuration)
- [Contributing](#contributing)
- [License](#license)

---

## Overview

Talia reads a list of domains from a JSON file and connects to a user-specified WHOIS server to check each domain’s availability. After retrieving the WHOIS response, Talia updates the JSON file with:

- A boolean `available` indicating whether the domain appears to be available.
- A short `reason` code explaining why the domain is or isn’t available (e.g., `NO_MATCH`, `TAKEN`, `ERROR`).
- A `log` field containing the raw WHOIS response or an error message (depending on whether **verbose mode** is enabled or an error occurred).

By default, Talia includes a **2-second delay** between checks to reduce server load, making it suitable for batch queries.

## Disclaimer

- **Terms of Service Compliance**: Please confirm that your chosen WHOIS provider permits automated queries and that you adhere to any usage limitations or policies they enforce.
- **Accuracy**: The WHOIS response may vary by provider, and Talia’s availability check is based on detecting the string `"No match for"`. This may not be reliable for all registrars or TLDs other than `.com`.

## Installation

### Prerequisites

- [Go](https://golang.org/doc/install) (1.16+ recommended)

### Steps

```bash
# Clone the Talia repository and enter the directory:
git clone https://github.com/sustanza/talia.git
cd talia

# Build the executable:
go build -o talia
```

This will produce a binary named `talia` in the current directory.

---

## Usage

Run the tool by specifying a WHOIS server and a path to the JSON file containing the domains you want to check:

```bash
./talia --whois=whois.example.com:43 path/to/domains.json
```

### JSON Input Format

Your JSON file should be an array of objects, each containing at least a `"domain"` field. For example:

```json
[{ "domain": "example.com" }, { "domain": "anotherexample.com" }]
```

Talia will add or update the following fields for each entry:

- **`available`**: A boolean indicating whether the domain appears available.
- **`reason`**: A short code explaining _why_ it’s available or not (`"NO_MATCH"`, `"TAKEN"`, `"ERROR"`, etc.).
- **`log`**: A string with the raw WHOIS response (shown only in verbose mode or if an error occurs).

### WHOIS Server Selection

Use the `--whois` flag to set a custom WHOIS server. If you omit this flag, Talia defaults to its internally configured server. A typical value for `.com` might be `whois.verisign-grs.com:43`.

Example:

```bash
./talia --whois=whois.verisign-grs.com:43 path/to/domains.json
```

### Domain Checks and Rate Limiting

For each domain, Talia:

1. Opens a TCP connection to the WHOIS server.
2. Sends the domain query.
3. Reads the server’s response.
4. Updates the local JSON file immediately.
5. **Sleeps** (default 2 seconds) before proceeding to the next domain.

### Optional Sleep Flag

You can override the default 2-second delay between checks by adding the `--sleep` flag:

```bash
./talia --whois=whois.verisign-grs.com:43 --sleep=5s path/to/domains.json
```

In this example, Talia will sleep **5 seconds** after each domain check.

### Verbose Mode

If you include the `--verbose` flag, the WHOIS response for each domain will be stored in the `"log"` field even if the domain is available (i.e., no error). In **non-verbose** mode, `"log"` remains empty unless an error occurs. For example:

```bash
./talia --whois=whois.verisign-grs.com:43 --verbose path/to/domains.json
```

---

## How It Works

### Domain Checking Flow

1. **JSON Parsing**: Talia parses the input file into a slice of domain records.
2. **WHOIS Query**: For each record, Talia sends the domain to the WHOIS server and reads the response.
3. **Update File**: Talia overwrites the JSON file with the updated results immediately after each check (including `available`, `reason`, and optionally `log`).

### Availability Determination

- If the WHOIS response contains the substring `"No match for"`, Talia marks the domain as **available** and sets `reason` to `NO_MATCH`.
- Otherwise, Talia marks the domain as **unavailable** with `reason` set to `TAKEN`.
- If any error occurs while connecting or reading from the WHOIS server, `available` is set to `false` and `reason` is `ERROR`.

### File Updates

Talia writes the updated JSON data back to the same file after each domain check. Any existing fields in the JSON are preserved unless overwritten by Talia. Make sure the file is writable to avoid errors.

---

## Testing

Talia includes a comprehensive test suite. To run all tests:

```bash
go test -v
```

Key tests include:

- **Unit Tests**: Verify domain availability checks using a mock WHOIS server.
- **Integration Tests**: Confirm the entire CLI flow (reading JSON, querying WHOIS, updating the file) behaves as expected.
- **Edge Cases**: Handle invalid JSON, unreadable files, immediate connection resets, custom sleep durations, and delayed responses.

All tests rely solely on the Go standard library and mock TCP connections; no external dependencies are required.

---

## Configuration

Below are the primary configuration options:

- **`--whois=<host:port>`**: Overrides the default WHOIS server (default is typically `whois.verisign-grs.com:43` for `.com` checks).
- **`--sleep=<duration>`**: Changes the delay between checks. Defaults to 2s.
- **`--verbose`**: Enables storing the WHOIS output in the `"log"` field for each domain (not just on error).
- **Input JSON File**: A path to a `.json` file containing domain records.

Any additional environment variables or build-time configurations can be documented as needed.
