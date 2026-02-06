# Fornex Terraform Provider

Terraform provider for managing Fornex services. Currently supports DNS management.

## Example Usage

```hcl
provider "fornex" {
  api_key = "your-api-key"
}

resource "fornex_domain" "example" {
  name = "example.com"
  ip   = "1.2.3.4"
}

resource "fornex_record" "www" {
  domain_name = fornex_domain.example.name
  host        = "www"
  type        = "A"
  value       = "1.2.3.4"
  ttl         = 3600
}

data "fornex_domains" "all" {}

data "fornex_domain" "example" {
  name = "example.com"
}

output "domain_names" {
  value = data.fornex_domains.all.domains[*].name
}
```

## Argument Reference

### Provider

* `api_key` (String, Sensitive) Your Fornex API key. Can also be set via `FORNEX_API_KEY` environment variable.
* `base_url` (String) Optional. Fornex API base URL. Defaults to `https://fornex.com/api`. Can also be set via `FORNEX_BASE_URL` environment variable.

### fornex_domain (Resource)

* `name` (String, Required) The domain name to manage.
* `ip` (String, Required) Initial IP address for the domain.

### fornex_record (Resource)

* `domain_name` (String, Required) The domain name this record belongs to.
* `host` (String, Required) The host part of the record (e.g., "www").
* `type` (String, Required) The type of the record (A, AAAA, CAA, CNAME, MX, NS, SRV, TXT).
* `value` (String, Required) The value of the record.
* `ttl` (Number, Optional) Time to live for the record.

### fornex_domain (Data Source)

* `name` (String, Required) The domain name to look up.

### fornex_domains (Data Source)

* `domains` (List of Objects) List of domains found.

## Development

### Build

To build the provider binary:

```bash
nix-shell -p go --run "go build -o terraform-provider-fornex ./cmd/terraform-provider-fornex"
```

### Test

To run unit tests:

```bash
nix-shell -p go --run "go test ./internal/client/..."
```
