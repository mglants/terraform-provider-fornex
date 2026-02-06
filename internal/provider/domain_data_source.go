package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/mglants/terraform-provider-fornex/internal/client"
)

var _ datasource.DataSource = &DomainDataSource{}

type DomainDataSource struct {
	client *client.Client
}

type DomainDataSourceModel struct {
	Name    types.String `tfsdk:"name"`
	Created types.String `tfsdk:"created"`
	Updated types.String `tfsdk:"updated"`
	Tags    types.List   `tfsdk:"tags"`
}

func NewDomainDataSource() datasource.DataSource {
	return &DomainDataSource{}
}

func (d *DomainDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (d *DomainDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Get information about a specific Fornex domain.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The domain name to look up.",
				Required:    true,
			},
			"created": schema.StringAttribute{
				Description: "The date and time the domain was created.",
				Computed:    true,
			},
			"updated": schema.StringAttribute{
				Description: "The date and time the domain was last updated.",
				Computed:    true,
			},
			"tags": schema.ListAttribute{
				Description: "List of tags associated with the domain.",
				ElementType: types.StringType,
				Computed:    true,
			},
		},
	}
}

func (d *DomainDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = c
}

func (d *DomainDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DomainDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	domain, err := d.client.GetDomain(data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get domain, got error: %s", err))
		return
	}

	tags, diags := types.ListValueFrom(ctx, types.StringType, domain.Tags)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	data.Created = types.StringValue(domain.Created)
	data.Updated = types.StringValue(domain.Updated)
	data.Tags = tags

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
