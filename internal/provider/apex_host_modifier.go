package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// apexHostNormalizer rewrites a planned host value of "@" to "". The Fornex
// API stores apex records with an empty host, so without this normalization
// state would always drift one direction from configuration written in the
// common DNS "@" convention.
type apexHostNormalizer struct{}

// ApexHostNormalizer returns a plan modifier that canonicalizes "@" to "" for
// host attributes, matching how the Fornex API represents apex records.
func ApexHostNormalizer() planmodifier.String {
	return apexHostNormalizer{}
}

func (apexHostNormalizer) Description(_ context.Context) string {
	return `Canonicalizes "@" to "" for apex records to match the Fornex API.`
}

func (n apexHostNormalizer) MarkdownDescription(ctx context.Context) string {
	return n.Description(ctx)
}

func (apexHostNormalizer) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}
	if req.PlanValue.ValueString() == "@" {
		resp.PlanValue = types.StringValue("")
	}
}
