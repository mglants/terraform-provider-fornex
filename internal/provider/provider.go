package provider

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/mglants/terraform-provider-fornex/internal/client"
)

// Ensure FornexProvider implements the provider.Provider interface.
var _ provider.Provider = &FornexProvider{}

type FornexProvider struct {
	version string
}

type FornexProviderModel struct {
	APIKey  types.String `tfsdk:"api_key"`
	BaseURL types.String `tfsdk:"base_url"`
	Timeout types.String `tfsdk:"timeout"`
}

func (p *FornexProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "fornex"
	resp.Version = p.version
}

func (p *FornexProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The Fornex provider is used to interact with the many services offered by Fornex. It provides resources to manage DNS records and domains.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Description: "Your Fornex API key. Can also be set via `FORNEX_API_KEY` environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"base_url": schema.StringAttribute{
				Description: "Fornex API base URL. Defaults to `https://fornex.com/api`. Can also be set via `FORNEX_BASE_URL` environment variable.",
				Optional:    true,
			},
			"timeout": schema.StringAttribute{
				Description: "Per-request HTTP timeout, as a Go duration string (e.g. `30s`, `2m`). Each retry attempt gets its own fresh timeout. Defaults to `1m`. Can also be set via `FORNEX_TIMEOUT` environment variable.",
				Optional:    true,
			},
		},
	}
}

func (p *FornexProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data FornexProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := os.Getenv("FORNEX_API_KEY")
	baseURL := os.Getenv("FORNEX_BASE_URL")
	timeoutStr := os.Getenv("FORNEX_TIMEOUT")

	if !data.APIKey.IsNull() {
		apiKey = data.APIKey.ValueString()
	}

	if !data.BaseURL.IsNull() {
		baseURL = data.BaseURL.ValueString()
	}

	if !data.Timeout.IsNull() {
		timeoutStr = data.Timeout.ValueString()
	}

	if apiKey == "" {
		resp.Diagnostics.AddError(
			"Missing API Key",
			"The provider cannot create the Fornex API client as there is no API key. "+
				"Set the api_key provider block field or the FORNEX_API_KEY environment variable.",
		)
	}

	var timeout time.Duration
	if timeoutStr != "" {
		parsed, err := time.ParseDuration(timeoutStr)
		if err != nil {
			resp.Diagnostics.AddError(
				"Invalid Timeout",
				fmt.Sprintf("The timeout value %q is not a valid Go duration (e.g. %q, %q): %s", timeoutStr, "30s", "2m", err),
			)
		} else if parsed <= 0 {
			resp.Diagnostics.AddError(
				"Invalid Timeout",
				fmt.Sprintf("The timeout value %q must be greater than zero.", timeoutStr),
			)
		} else {
			timeout = parsed
		}
	}

	if resp.Diagnostics.HasError() {
		return
	}

	c := client.NewClient(apiKey, baseURL, timeout)
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *FornexProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewDomainResource,
		NewRecordResource,
	}
}

func (p *FornexProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewDomainsDataSource,
		NewDomainDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &FornexProvider{
			version: version,
		}
	}
}
