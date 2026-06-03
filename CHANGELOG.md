## 1.0.0

BREAKING CHANGES:
  fornex_domain: remove the `ip` attribute. Fornex's `POST /dns/domain/`
  still requires an IP server-side and uses it to seed a default zone
  template (A records for `""`, `*`, `www`, `mail` plus `MX "" -> "mail"`).
  The provider now sends a placeholder IP and immediately deletes those
  auto-created records during Create, so the value never reached anyone's
  resolver and the schema attribute had no observable effect. Remove
  `ip = "..."` from any `fornex_domain` blocks; no state migration is
  needed (the attribute was create-only and not tracked in state).

## 0.1.0

FEATURES:
  edit dns records
  edit domains
