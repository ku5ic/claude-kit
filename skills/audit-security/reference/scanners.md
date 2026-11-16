# Secret scanners

- trufflehog checks a found credential against its issuer's API unless `--no-verification` is given. Without the flag, a found key is tried live.
- Under `--no-verification` no hit is verified, so the report says the hits are unverified and never reads them as inactive.
