# Talia

Talia finds registrable `.com` names by generating candidates with an LLM and checking each one against a WHOIS server.

## Language

### Domains and status

**Domain**:
A second-level `.com` name of the form `label.com`.
_Avoid_: URL, hostname, site

**Domain file**:
The JSON file holding a set of domains and their status, read and rewritten in place across runs.
_Avoid_: database, results file

**Array file**:
A domain file shaped as a flat list of domain records, each carrying its own availability.

**Grouped file**:
A domain file shaped as buckets: `available`, `unavailable`, and optionally `unverified`.
_Avoid_: extended file, grouped output

**Bucket**:
One of the three status lists in a grouped file.
_Avoid_: section, category

**Available**:
A domain whose check found no existing registration.
_Avoid_: free, open

**Unavailable**:
A domain that cannot be treated as registrable: either taken or its check errored.
_Avoid_: taken (as a bucket name), registered

**Unverified**:
A domain that has not been checked yet, usually a fresh suggestion.
_Avoid_: pending, new, unchecked

**Reason**:
The outcome code of a check: `NO_MATCH` (available), `TAKEN`, or `ERROR`.
_Avoid_: status, result

### Workflow

**Check**:
One WHOIS query for one domain, producing a reason.
_Avoid_: lookup, verification (for a single domain)

**Suggestion**:
A domain proposed by the LLM; it enters the domain file as unverified.
_Avoid_: idea, generation, candidate

**Exclusion list**:
The domains already in the file, sent with a suggestion request so the model avoids repeating them.

**Auto-verification**:
Checking new suggestions immediately after they are written.

**Normalization**:
Rewriting a raw string into canonical `label.com` form, or rejecting it.
_Avoid_: sanitizing, validation

**Clean**:
Normalizing and deduplicating a domain file in place.

**Merge**:
Combining several domain files into one, without duplicates.

**Export**:
Writing the available bucket to a plain text list.

**Lightspeed**:
Checking domains in parallel instead of sequentially with a sleep between checks.
_Avoid_: turbo, fast mode
