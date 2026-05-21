package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestApexHostNormalizer_PlanModifyString(t *testing.T) {
	cases := []struct {
		name string
		in   types.String
		want types.String
	}{
		{"apex_at_sign_to_empty", types.StringValue("@"), types.StringValue("")},
		{"empty_passthrough", types.StringValue(""), types.StringValue("")},
		{"subdomain_passthrough", types.StringValue("www"), types.StringValue("www")},
		{"nested_subdomain_passthrough", types.StringValue("_imaps._tcp"), types.StringValue("_imaps._tcp")},
		{"null_passthrough", types.StringNull(), types.StringNull()},
		{"unknown_passthrough", types.StringUnknown(), types.StringUnknown()},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := planmodifier.StringRequest{PlanValue: c.in}
			resp := planmodifier.StringResponse{PlanValue: c.in}
			ApexHostNormalizer().PlanModifyString(context.Background(), req, &resp)
			if !resp.PlanValue.Equal(c.want) {
				t.Fatalf("got %s, want %s", resp.PlanValue, c.want)
			}
		})
	}
}
