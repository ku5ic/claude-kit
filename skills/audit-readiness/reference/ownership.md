# Ownership

- A launch has one named owning team and an on-call route. A team accepts responsibility for production through the review, so no owner is the top finding.
- CODEOWNERS (in `.github/`, the root, or `docs/`) names who is responsible for code and gets review requests. It proves review ownership, not operational ownership.
- A Backstage `catalog-info.yaml` `spec.owner` is the entity, usually a team, that bears ultimate responsibility for the component. Its links and annotations may carry the on-call, runbook, and dashboard URLs, under the project's own keys.
- An owner that's one person, or a CODEOWNERS team the catalog doesn't name, is a finding.
