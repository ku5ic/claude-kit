# Code comments and docstrings

When to comment at all is `rules/change.md` section 5. This file covers how a comment reads once it has earned its place.

<!-- chat: skip -->

The guard scans only comment and docstring text, only on lines a write adds. Code and string literals are never scanned.

<!-- chat: end -->

| Tell                                                               | Instead                                                                       |
| ------------------------------------------------------------------ | ----------------------------------------------------------------------------- |
| "This function ..." / "This method is responsible for ..."         | Start with the verb: "Loads ...", "Returns ..."                               |
| An "-ing" tail: "Retries the call, ensuring robustness."           | The real constraint: "Retries once: the API drops the first call after idle." |
| "serves as", "acts as a wrapper around"                            | "is", "wraps"                                                                 |
| A changelog in a comment: "Updated to handle X", "Now also does Y" | The current behavior; history belongs in the commit                           |
| "Note:" or "Important:" leading every other comment                | Only the surprising one, and no label                                         |
| Praise for the code: "robust", "comprehensive", "efficient"        | The property with its number or condition                                     |
| A docstring that repeats the signature in prose                    | The contract the signature can't show: units, errors, invariants              |

## Wrapped lines

A tell split across two comment lines isn't caught; the scan reads one line at a time. Hold the same standard by eye.
