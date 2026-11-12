# security-patterns sources

The sources and version notes behind this skill, for `/meta refresh`. Nothing links here, so a session never loads it.

## SKILL.md

### References

- OWASP Top 10: https://owasp.org/www-project-top-ten/
- OWASP Cheat Sheet Series: https://cheatsheetseries.owasp.org/
- OWASP ASVS: https://owasp.org/www-project-application-security-verification-standard/
- MDN Web Security: https://developer.mozilla.org/en-US/docs/Web/Security
- MDN CSP: https://developer.mozilla.org/en-US/docs/Web/HTTP/CSP
- WHATWG Fetch (CORS): https://fetch.spec.whatwg.org/

### Version notes

Checked: 2026-11-10 against https://slsa.dev/spec/v1.2/build-track-basics, https://docs.djangoproject.com/en/stable/ref/csp/, https://docs.npmjs.com/cli/v12/commands/npm-audit, https://github.com/gitleaks/gitleaks and https://cheatsheetseries.owasp.org/cheatsheets/JSON_Web_Token_Cheat_Sheet.html

- SLSA 1.2: current spec; 1.0 retired. Build track is L0 (no requirements) through L3.
- Django 6.0: built-in CSP middleware (`SECURE_CSP`, `csp_nonce`); 6.1 adds the `{% csp_nonce_attr %}` tag.
- npm 12 (12.2.2): current CLI docs; v10 and v11 docs marked Legacy. Also corrected: gitleaks `detect` is deprecated for `gitleaks git`, the OWASP JWT cheat sheet moved, and its session advice is quoted.

### Left to linters

Checked: 2026-11-11 with ruff 0.17.4 defaults (`uvx ruff check --isolated`, no config, no `--select`), ESLint 10.14.0 with `js.configs.recommended` from `@eslint/js` 10.0.1, go vet from Go 1.27.0, and golangci-lint 2.14.0 defaults, on a probe in each language. From `S`, ruff's default set has only `S102`, `S110`, and `S112`, and no rule was dropped.

ruff `UP031` reports the `%`-formatted SQL strings, but only as old-style formatting, and its suggested fix keeps the injection, so the SQL rule stays.

Not caught by defaults, so kept: SQL built by concatenation, an f-string, or `%` in Python (`S608` is off), JS, or Go (go vet and golangci-lint defaults report nothing on `db.Query` with `+` or `fmt.Sprintf`), a hardcoded password or API key (`S105` is off), `mark_safe()` (`S308` is off), `raw()`, `extra()`, and `RawSQL` (`S611` is off), `.filter(**request.GET)`, an `innerHTML` assignment, a `javascript:` URL (`no-script-url` isn't in recommended), `window.location = userValue`, and a token in `localStorage`.

## reference/anti-patterns.md

### References

- OWASP SQL Injection: https://owasp.org/www-community/attacks/SQL_Injection
- OWASP XSS Prevention: https://cheatsheetseries.owasp.org/cheatsheets/Cross_Site_Scripting_Prevention_Cheat_Sheet.html
- OWASP CSRF Prevention: https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html
- OWASP Session Management: https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html
- CSP nonce/hash guidance: https://developer.mozilla.org/en-US/docs/Web/HTTP/CSP
- DOMPurify: https://github.com/cure53/DOMPurify

## reference/backend-django.md

### References

- Django security topics: https://docs.djangoproject.com/en/stable/topics/security/
- Django deployment checklist: https://docs.djangoproject.com/en/stable/howto/deployment/checklist/
- DRF permissions: https://www.django-rest-framework.org/api-guide/permissions/

## reference/backend-node.md

### References

- OWASP CSRF Cheat Sheet: https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html
- Express security best practices: https://expressjs.com/en/advanced/best-practice-security.html
- Helmet: https://helmetjs.github.io/
- WHATWG Fetch (CORS): https://fetch.spec.whatwg.org/

## reference/dependencies.md

### References

- OSV-Scanner: https://google.github.io/osv-scanner/
- SLSA spec: https://slsa.dev/spec/v1.2/build-track-basics
- npm audit: https://docs.npmjs.com/cli/v12/commands/npm-audit
- pip-audit: https://pypi.org/project/pip-audit/
- bundler-audit: https://github.com/rubysec/bundler-audit

### Claims

- Audit tools: lockfile vs full tree, OSV-Scanner queries the OSV federated database, which agg...: https://google.github.io/osv-scanner/
- SLSA basics, SLSA (Supply-chain Levels for Software Artifacts) is a fr...: https://slsa.dev/spec/v1.2/build-track-basics

## reference/frontend.md

### References

- OWASP Top 10: https://owasp.org/www-project-top-ten/
- OWASP HTML5 Security: https://cheatsheetseries.owasp.org/cheatsheets/HTML5_Security_Cheat_Sheet.html
- OWASP CSRF Prevention: https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html
- OWASP Session Management: https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html
- OWASP JWT Cheat Sheet: https://cheatsheetseries.owasp.org/cheatsheets/JSON_Web_Token_Cheat_Sheet.html
- MDN CSP: https://developer.mozilla.org/en-US/docs/Web/HTTP/CSP

## reference/secrets.md

### References

- gitleaks: https://github.com/gitleaks/gitleaks
- trufflehog: https://github.com/trufflesecurity/trufflehog
- GitHub Secret Scanning: https://docs.github.com/en/code-security/secret-scanning/

### Claims

- Detection in CI, **gitleaks**: regex-plus-entropy scanner for git repos an...: https://github.com/gitleaks/gitleaks
- Detection in CI, **trufflehog**: credential-aware scanner that goes one st...: https://github.com/trufflesecurity/trufflehog
- Detection in CI, **Secret-scanning push protection**: enabled at the repo...: https://docs.github.com/en/code-security/secret-scanning/
