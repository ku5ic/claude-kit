# logging-patterns sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

- https://docs.python.org/3/library/logging.html
- https://docs.python.org/3/howto/logging.html
- https://www.structlog.org/en/stable/getting-started.html

### Version notes

Checked: 2026-11-10 against https://pypi.org/project/structlog/, https://www.structlog.org/en/stable/api.html, https://docs.python.org/3/library/logging.html, https://www.python.org/downloads/

- structlog 26.1.0 (2026-06-06) still current: requires Python >= 3.10; no change to the processors used here. Python 3.15.2 current, 3.14.10 the latest 3.14.

### Left to linters

Checked: 2026-11-11 with ruff 0.17.4 defaults (`uvx ruff check --isolated`, no config, no `--select`) on a probe file. No rule here was dropped, because no default rule reports one in every form it covers.

Not caught by defaults, so kept: an f-string in a log call (`G004` is off), `print()` in server code (`T201` is off), library code calling `basicConfig`, `addHandler`, or `setLevel` at module level, and an error log with no identifiers.

Two rules stay even though a default rule fires on one form of them, because the rule also covers forms the check misses. `E722` reports a bare `except:` in a logging loop, but not `except Exception:` with `logger.exception`, and `BLE001` doesn't report that form either. `LOG015` reports `logging.info(...)`, but not `logging.getLogger().info(...)`.
