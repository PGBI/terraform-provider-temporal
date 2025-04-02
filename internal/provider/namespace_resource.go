package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"go.temporal.io/api/enums/v1"
	tpNamespace "go.temporal.io/api/namespace/v1"
	"go.temporal.io/api/operatorservice/v1"
	"go.temporal.io/api/workflowservice/v1"
	temporal "go.temporal.io/sdk/client"
	"google.golang.org/protobuf/types/known/durationpb"
	"strings"
	"terraform-provider-temporal/internal/validators"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &namespaceResource{}
	_ resource.ResourceWithConfigure = &namespaceResource{}
)

func NewNamespaceResource() resource.Resource {
	return &namespaceResource{}
}

type namespaceResource struct {
	client temporal.Client
}

type namespaceResourceModel struct {
	Name                    basetypes.StringValue `tfsdk:"name"`
	ID                      basetypes.StringValue `tfsdk:"id"`
	Description             basetypes.StringValue `tfsdk:"description"`
	RetentionTTL            basetypes.StringValue `tfsdk:"retention_ttl"`
	OwnerEmail              basetypes.StringValue `tfsdk:"owner_email"`
	IsGlobal                basetypes.BoolValue   `tfsdk:"is_global"`
	HistoryArchivalState    basetypes.StringValue `tfsdk:"history_archival_state"`
	HistoryArchivalURI      basetypes.StringValue `tfsdk:"history_archival_uri"`
	VisibilityArchivalState basetypes.StringValue `tfsdk:"visibility_archival_state"`
	VisibilityArchivalURI   basetypes.StringValue `tfsdk:"visibility_archival_uri"`
	Data                    map[string]string     `tfsdk:"data"`
}

func archivalStateValue(diags diag.Diagnostics, str string) enums.ArchivalState {
	var res enums.ArchivalState

	switch strings.ToLower(str) {
	case "unspecified":
		res = enums.ARCHIVAL_STATE_UNSPECIFIED
	case "enabled":
		res = enums.ARCHIVAL_STATE_ENABLED
	case "disabled":
		res = enums.ARCHIVAL_STATE_DISABLED
	default:
		diags.AddError("Invalid archival state value", "Unknown archival state value "+str)
	}

	return res
}

type namespaceResponse interface {
	GetNamespaceInfo() *tpNamespace.NamespaceInfo
	GetConfig() *tpNamespace.NamespaceConfig
	GetIsGlobalNamespace() bool
}

func parseNamespaceResource(namespace namespaceResponse) *namespaceResourceModel {
	return &namespaceResourceModel{
		Name:                    types.StringValue(namespace.GetNamespaceInfo().Name),
		ID:                      types.StringValue(namespace.GetNamespaceInfo().Id),
		Description:             types.StringValue(namespace.GetNamespaceInfo().Description),
		RetentionTTL:            types.StringValue(formatDuration(namespace.GetConfig().WorkflowExecutionRetentionTtl.AsDuration())),
		OwnerEmail:              types.StringValue(namespace.GetNamespaceInfo().OwnerEmail),
		IsGlobal:                types.BoolValue(namespace.GetIsGlobalNamespace()),
		HistoryArchivalState:    types.StringValue(strings.ToLower(namespace.GetConfig().HistoryArchivalState.String())),
		HistoryArchivalURI:      types.StringValue(namespace.GetConfig().GetHistoryArchivalUri()),
		VisibilityArchivalState: types.StringValue(strings.ToLower(namespace.GetConfig().VisibilityArchivalState.String())),
		VisibilityArchivalURI:   types.StringValue(strings.ToLower(namespace.GetConfig().GetVisibilityArchivalUri())),
		Data:                    namespace.GetNamespaceInfo().Data,
	}
}

func (r *namespaceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Add a nil check when handling ProviderData because Terraform
	// sets that data after it calls the ConfigureProvider RPC.
	if req.ProviderData == nil {
		return
	}

	cfg, ok := req.ProviderData.(*providerConfig)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *provider.providerConfig, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = cfg.client
}

// Metadata returns the resource type name.
func (r *namespaceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespace"
}

// Schema defines the schema for the resource.
func (r *namespaceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The `temporal_namespace` resource allows you to create and manage namespaces within a Temporal server. A namespace in Temporal is a logical grouping of workflows, which helps in isolating and organizing workflows and activities.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Namespace ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the Namespace.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Description: "Namespace description.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"retention_ttl": schema.StringAttribute{
				Description: "Workflow execution retention TTL. E.g \"24h\", \"365d\".",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("365d"),
				Validators: []validator.String{
					validators.StringDurationValidator{},
				},
			},
			"owner_email": schema.StringAttribute{
				Description: "Namespace owner email address.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"is_global": schema.BoolAttribute{
				Description: "Whether that namespace should be a global namespace. Global namespaces must be enabled on the cluster to be able to promote a namespace to global.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"history_archival_state": schema.StringAttribute{
				MarkdownDescription: "History archival state. Accepted values: `disabled`, `enabled`. History archival must be enabled at the cluster level first to be able to enable it for a namespace.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("disabled"),
				Validators: []validator.String{
					validators.StringInSliceValidator{
						AllowedValues: []string{"enabled", "disabled"},
					},
				},
			},
			"history_archival_uri": schema.StringAttribute{
				MarkdownDescription: "History Archival URI.",
				Computed:            true,
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"visibility_archival_state": schema.StringAttribute{
				MarkdownDescription: "Visibility archival state. Accepted values: `disabled`, `enabled`. Visibility archival must be enabled at the cluster level first to be able to enable it for a namespace.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("disabled"),
				Validators: []validator.String{
					validators.StringInSliceValidator{
						AllowedValues: []string{"enabled", "disabled"},
					},
				},
			},
			"visibility_archival_uri": schema.StringAttribute{
				MarkdownDescription: "Visibility Archival URI.",
				Computed:            true,
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"data": schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Namespace data in key=value format.",
				Default:     nil,
			},
		},
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *namespaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *namespaceResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	historyArchivalState := archivalStateValue(resp.Diagnostics, data.HistoryArchivalState.ValueString())
	visibilityArchivalState := archivalStateValue(resp.Diagnostics, data.VisibilityArchivalState.ValueString())
	if resp.Diagnostics.HasError() {
		return
	}

	// Create new namespace
	ttl, _ := parseDuration(data.RetentionTTL.ValueString())
	var _, err = r.client.WorkflowService().RegisterNamespace(ctx, &workflowservice.RegisterNamespaceRequest{
		Namespace:   data.Name.ValueString(),
		Description: data.Description.ValueString(),
		OwnerEmail:  data.OwnerEmail.ValueString(),
		WorkflowExecutionRetentionPeriod: &durationpb.Duration{
			Seconds: int64(ttl.Seconds()),
		},
		IsGlobalNamespace:       data.IsGlobal.ValueBool(),
		HistoryArchivalState:    historyArchivalState,
		HistoryArchivalUri:      data.HistoryArchivalURI.ValueString(),
		VisibilityArchivalState: visibilityArchivalState,
		VisibilityArchivalUri:   data.VisibilityArchivalURI.ValueString(),
		Data:                    data.Data, // map[string]string
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating namespace",
			"Could not create namespace, unexpected error: "+err.Error(),
		)
		return
	}

	namespace, err := r.client.WorkflowService().DescribeNamespace(ctx, &workflowservice.DescribeNamespaceRequest{
		Namespace: data.Name.ValueString(),
	})

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to find Temporal namespace after creation ",
			err.Error(),
		)
		return
	}

	if data.HistoryArchivalState.ValueString() == "enabled" && namespace.GetConfig().HistoryArchivalState != enums.ARCHIVAL_STATE_ENABLED {
		resp.Diagnostics.AddError("Unable to enable history archival for the namespace. Is history archival enabled at the cluster level?", "")
	}
	if data.VisibilityArchivalState.ValueString() == "enabled" && namespace.GetConfig().VisibilityArchivalState != enums.ARCHIVAL_STATE_ENABLED {
		resp.Diagnostics.AddError("Unable to enable visibility archival for the namespace. Is visibility archival enabled at the cluster level?", "")
	}

	data = parseNamespaceResource(namespace)

	diags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *namespaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var id string
	diags := req.State.GetAttribute(ctx, path.Root("id"), &id)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	namespace, err := r.client.WorkflowService().DescribeNamespace(ctx, &workflowservice.DescribeNamespaceRequest{Id: id})
	if err != nil {
		if err.Error() == "Namespace "+id+" is not found." {
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error fetching the Namespace "+id, err.Error())
		}
		return
	}

	data := parseNamespaceResource(namespace)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *namespaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *namespaceResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	historyArchivalState := archivalStateValue(resp.Diagnostics, data.HistoryArchivalState.ValueString())
	visibilityArchivalState := archivalStateValue(resp.Diagnostics, data.VisibilityArchivalState.ValueString())
	if resp.Diagnostics.HasError() {
		return
	}

	ttl, _ := parseDuration(data.RetentionTTL.ValueString())

	var ns, err = r.client.WorkflowService().UpdateNamespace(ctx, &workflowservice.UpdateNamespaceRequest{
		Namespace: data.Name.ValueString(),
		UpdateInfo: &tpNamespace.UpdateNamespaceInfo{
			Description: data.Description.ValueString(),
			OwnerEmail:  data.OwnerEmail.ValueString(),
			Data:        data.Data,
		},
		Config: &tpNamespace.NamespaceConfig{
			WorkflowExecutionRetentionTtl: &durationpb.Duration{Seconds: int64(ttl.Seconds())},
			HistoryArchivalState:          historyArchivalState,
			HistoryArchivalUri:            data.HistoryArchivalURI.ValueString(),
			VisibilityArchivalState:       visibilityArchivalState,
			VisibilityArchivalUri:         data.VisibilityArchivalURI.ValueString(),
		},
		PromoteNamespace: data.IsGlobal.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating namespace",
			"Unexpected error: "+err.Error(),
		)
		return
	}
	if data.HistoryArchivalState.ValueString() == "enabled" && ns.GetConfig().HistoryArchivalState != enums.ARCHIVAL_STATE_ENABLED {
		resp.Diagnostics.AddError("Unable to enable history archival for the namespace. Is history archival enabled at the cluster level?", "")
	}
	if data.VisibilityArchivalState.ValueString() == "enabled" && ns.GetConfig().VisibilityArchivalState != enums.ARCHIVAL_STATE_ENABLED {
		resp.Diagnostics.AddError("Unable to enable visibility archival for the namespace. Is visibility archival enabled at the cluster level?", "")
	}

	data = parseNamespaceResource(ns)

	diags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *namespaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data namespaceResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.OperatorService().DeleteNamespace(ctx, &operatorservice.DeleteNamespaceRequest{
		NamespaceId: data.ID.ValueString(),
		Namespace:   data.Name.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error while deleting namespace "+data.Name.ValueString(), err.Error())
		return
	}
}

func (r *namespaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
