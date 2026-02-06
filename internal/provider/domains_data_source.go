package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/mglants/terraform-provider-fornex/internal/client"
)

var _ datasource.DataSource = &DomainsDataSource{}

type DomainsDataSource struct {
	client *client.Client
}

type DomainsDataSourceModel struct {
	Domains []DomainModel `tfsdk:"domains"`
}

type DomainModel struct {
	Name    types.String `tfsdk:"name"`
	Created types.String `tfsdk:"created"`
	Updated types.String `tfsdk:"updated"`
	Tags    types.List   `tfsdk:"tags"`
}

func NewDomainsDataSource() datasource.DataSource {
	return &DomainsDataSource{}
}

func (d *DomainsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domains"
}

func (d *DomainsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Get information about all Fornex domains.",
		Attributes: map[string]schema.Attribute{
			"domains": schema.ListNestedAttribute{
				Description: "List of domains found.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Description: "The domain name.",
							Computed:    true,
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
				},
			},
		},
	}
}

func (d *DomainsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DomainsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DomainsDataSourceModel

	domains, err := d.client.ListDomains()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list domains, got error: %s", err))
		return
	}

	for _, domain := range domains {
		tags, diags := types.ListValueFrom(ctx, types.StringType, domain.Tags)
		resp.Diagnostics.Append(diags...)
		if diags.HasError() {
			return
		}

		data.Domains = append(data.Domains, DomainModel{
			Name:    types.StringValue(domain.Name),
			Created: types.StringValue(domain.Created),
			Updated: types.StringValue(domain.Updated),
			Tags:    tags,
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
