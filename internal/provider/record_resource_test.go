package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

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

// ttlValidators pulls the validators wired onto the "ttl" attribute in the
// record resource schema so tests exercise the same chain the provider uses
// at plan time.
func ttlValidators(t *testing.T) []validator.Int64 {
	t.Helper()

	ctx := context.Background()
	resp := &resource.SchemaResponse{}
	NewRecordResource().(*RecordResource).Schema(ctx, resource.SchemaRequest{}, resp)

	attr, ok := resp.Schema.Attributes["ttl"].(schema.Int64Attribute)
	if !ok {
		t.Fatalf("ttl attribute is not Int64Attribute, got %T", resp.Schema.Attributes["ttl"])
	}
	return attr.Validators
}

func runTTLValidators(t *testing.T, val types.Int64) bool {
	t.Helper()

	req := validator.Int64Request{
		Path:           path.Root("ttl"),
		PathExpression: path.MatchRoot("ttl"),
		ConfigValue:    val,
	}
	resp := &validator.Int64Response{}
	for _, v := range ttlValidators(t) {
		v.ValidateInt64(context.Background(), req, resp)
	}
	return resp.Diagnostics.HasError()
}

func TestRecordTTLValidator(t *testing.T) {
	allowed := []int64{120, 300, 600, 900, 1800, 3600, 7200, 18000, 43200, 86400}
	for _, v := range allowed {
		t.Run("allowed/"+itoa(v), func(t *testing.T) {
			if runTTLValidators(t, types.Int64Value(v)) {
				t.Fatalf("ttl=%d should be accepted but validator errored", v)
			}
		})
	}

	rejected := []int64{0, 1, 60, 119, 121, 200, 1200, 7199, 90000}
	for _, v := range rejected {
		t.Run("rejected/"+itoa(v), func(t *testing.T) {
			if !runTTLValidators(t, types.Int64Value(v)) {
				t.Fatalf("ttl=%d should be rejected but validator accepted it", v)
			}
		})
	}

	t.Run("null_is_auto", func(t *testing.T) {
		if runTTLValidators(t, types.Int64Null()) {
			t.Fatal("null ttl should pass validation (represents auto)")
		}
	})

	t.Run("unknown_passes", func(t *testing.T) {
		if runTTLValidators(t, types.Int64Unknown()) {
			t.Fatal("unknown ttl should pass validation")
		}
	})
}

func itoa(v int64) string {
	return types.Int64Value(v).String()
}
