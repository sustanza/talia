# The two merge paths are deliberately different

`--merge` combines files with **first-write-wins**: once a domain appears in any bucket, later occurrences are ignored, so input order decides. Writing grouped results to an existing `--output-file` is **newest-wins**: a fresh check result replaces the old one and can move a domain between `available` and `unavailable`. The first is for combining lists; the second is for keeping a running record current as registrations change. Don't unify them without preserving both behaviours.
