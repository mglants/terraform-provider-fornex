package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/mglants/terraform-provider-fornex/internal/client"
)

var _ resource.Resource = &RecordResource{}
var _ resource.ResourceWithImportState = &RecordResource{}

type RecordResource struct {
	client *client.Client
}

type RecordResourceModel struct {
	ID         types.Int64  `tfsdk:"id"`
	DomainName types.String `tfsdk:"domain_name"`
	Host       types.String `tfsdk:"host"`
	Type       types.String `tfsdk:"type"`
	TTL        types.Int64  `tfsdk:"ttl"`
	Value      types.String `tfsdk:"value"`
	Priority   types.Int64  `tfsdk:"priority"`
}

func NewRecordResource() resource.Resource {
	return &RecordResource{}
}

func (r *RecordResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_record"
}

func (r *RecordResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provides a Fornex DNS record resource. This can be used to create, modify, and delete DNS records.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "The ID of the record.",
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"domain_name": schema.StringAttribute{
				Description: "The domain name this record belongs to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"host": schema.StringAttribute{
				Description: "The host part of the record (e.g., \"www\").",
				Required:    true,
			},
			"type": schema.StringAttribute{
				Description: "The type of the record (A, AAAA, CAA, CNAME, MX, NS, SRV, TXT).",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("A", "AAAA", "CAA", "CNAME", "MX", "NS", "SRV", "TXT"),
				},
			},
			"ttl": schema.Int64Attribute{
				Description: "Time to live for the record.",
				Optional:    true,
			},
			"value": schema.StringAttribute{
				Description: "The value of the record.",
				Required:    true,
			},
			"priority": schema.Int64Attribute{
				Description: "Priority of the record (used for MX, SRV).",
				Optional:    true,
			},
		},
	}
}

func (r *RecordResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = c
}

func (r *RecordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RecordResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	entry := client.Entry{
		Host:  data.Host.ValueString(),
		Type:  data.Type.ValueString(),
		Value: data.Value.ValueString(),
	}

	if !data.TTL.IsNull() {
		ttl := int(data.TTL.ValueInt64())
		entry.TTL = &ttl
	}

	if !data.Priority.IsNull() {
		priority := int(data.Priority.ValueInt64())
		entry.Priority = &priority
	}

	newEntry, err := r.client.CreateEntry(data.DomainName.ValueString(), entry)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create record, got error: %s", err))
		return
	}

	data.ID = types.Int64Value(int64(newEntry.ID))
	if newEntry.TTL != nil {
		data.TTL = types.Int64Value(int64(*newEntry.TTL))
	} else {
		data.TTL = types.Int64Null()
	}

	if newEntry.Priority != nil {
		data.Priority = types.Int64Value(int64(*newEntry.Priority))
	} else {
		data.Priority = types.Int64Null()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RecordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RecordResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	entry, err := r.client.GetEntry(data.DomainName.ValueString(), int(data.ID.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read record, got error: %s", err))
		return
	}

	data.Host = types.StringValue(entry.Host)
	data.Type = types.StringValue(entry.Type)
	data.Value = types.StringValue(entry.Value)
	if entry.TTL != nil {
		data.TTL = types.Int64Value(int64(*entry.TTL))
	} else {
		data.TTL = types.Int64Null()
	}

	if entry.Priority != nil {
		data.Priority = types.Int64Value(int64(*entry.Priority))
	} else {
		data.Priority = types.Int64Null()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RecordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RecordResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	entry := client.Entry{
		Host:  data.Host.ValueString(),
		Type:  data.Type.ValueString(),
		Value: data.Value.ValueString(),
	}

	if !data.TTL.IsNull() {
		ttl := int(data.TTL.ValueInt64())
		entry.TTL = &ttl
	}

	if !data.Priority.IsNull() {
		priority := int(data.Priority.ValueInt64())
		entry.Priority = &priority
	}

	updatedEntry, err := r.client.UpdateEntry(data.DomainName.ValueString(), int(data.ID.ValueInt64()), entry)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update record, got error: %s", err))
		return
	}

	if updatedEntry.TTL != nil {
		data.TTL = types.Int64Value(int64(*updatedEntry.TTL))
	} else {
		data.TTL = types.Int64Null()
	}

	if updatedEntry.Priority != nil {
		data.Priority = types.Int64Value(int64(*updatedEntry.Priority))
	} else {
		data.Priority = types.Int64Null()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RecordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RecordResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteEntry(data.DomainName.ValueString(), int(data.ID.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete record, got error: %s", err))
		return
	}
}

func (r *RecordResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	idParts := strings.Split(req.ID, ":")

	if len(idParts) != 2 || idParts[0] == "" || idParts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: domain_name:record_id. Got: %q", req.ID),
		)
		return
	}

	domainName := idParts[0]
	recordID, err := strconv.ParseInt(idParts[1], 10, 64)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: domain_name:record_id. Got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain_name"), domainName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), recordID)...)
}
