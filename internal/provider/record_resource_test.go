package provider

import "testing"

func TestHostForAPI(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"apex_at_sign", "@", ""},
		{"empty_passthrough", "", ""},
		{"subdomain", "www", "www"},
		{"nested_subdomain", "_imaps._tcp", "_imaps._tcp"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := hostForAPI(c.in); got != c.want {
				t.Fatalf("hostForAPI(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestHostFromAPI(t *testing.T) {
	const domain = "example.com"

	cases := []struct {
		name    string
		apiHost string
		want    string
	}{
		{"apex_empty_to_at_sign", "", "@"},
		{"apex_literal_at_sign", "@", "@"},
		{"apex_domain_no_dot", "example.com", "@"},
		{"apex_domain_with_dot", "example.com.", "@"},
		{"wildcard_passthrough", "*", "*"},
		{"short_subdomain_passthrough", "www", "www"},
		{"another_short_subdomain", "mail", "mail"},
		{"underscored_subdomain", "default._domainkey", "default._domainkey"},
		{"misconfigured_fqdn_subdomain_preserved", "default._domainkey.example.com.", "default._domainkey.example.com."},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := hostFromAPI(c.apiHost, domain); got != c.want {
				t.Fatalf("hostFromAPI(%q, %q) = %q, want %q", c.apiHost, domain, got, c.want)
			}
		})
	}
}
