package provider

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	matlas "go.mongodb.org/atlas/mongodbatlas"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"

	"github.com/mongodb/terraform-provider-mongodbatlas/mongodbatlas/framework/utils"
)

const (
	errorProjectCreate  = "error creating atlas project"
	errorProjectRead    = "error getting atlas project(%s)"
	errorProjectDelete  = "error deleting atlas project (%s)"
	errorProjectSetting = "error setting `%s` for atlas project (%s)"
)

var _ resource.Resource = &ProjectResource{}
var _ resource.ResourceWithImportState = &ProjectResource{}

func NewProjectResource() resource.Resource {
	return &ProjectResource{}
}

type ProjectResource struct {
	client *MongoDBClient
}

type projectResourceModel struct {
	ID                                          types.String `tfsdk:"id"`
	Name                                        types.String `tfsdk:"name"`
	OrgID                                       types.String `tfsdk:"org_id"`
	ClusterCount                                types.Int64  `tfsdk:"cluster_count"`
	Created                                     types.String `tfsdk:"created"`
	ProjectOwnerID                              types.String `tfsdk:"project_owner_id"`
	WithDefaultAlertsSettings                   types.Bool   `tfsdk:"with_default_alerts_settings"`
	IsCollectDatabaseSpecificsStatisticsEnabled types.Bool   `tfsdk:"is_collect_database_specifics_statistics_enabled"`
	IsDataExplorerEnabled                       types.Bool   `tfsdk:"is_data_explorer_enabled"`
	IsExtendedStorageSizesEnabled               types.Bool   `tfsdk:"is_extended_storage_sizes_enabled"`
	IsPerformanceAdvisorEnabled                 types.Bool   `tfsdk:"is_performance_advisor_enabled"`
	IsRealtimePerformancePanelEnabled           types.Bool   `tfsdk:"is_realtime_performance_panel_enabled"`
	IsSchemaAdvisorEnabled                      types.Bool   `tfsdk:"is_schema_advisor_enabled"`
	RegionUsageRestrictions                     types.String `tfsdk:"region_usage_restrictions"`
	Teams                                       []team       `tfsdk:"teams"`
	ApiKeys                                     []apiKey     `tfsdk:"api_keys"`
}

type team struct {
	TeamID    types.String `tfsdk:"team_id"`
	RoleNames types.Set    `tfsdk:"role_names"`
}

type apiKey struct {
	ApiKeyID  types.String `tfsdk:"api_key_id"`
	RoleNames types.Set    `tfsdk:"role_names"`
}

// Resources that need to be cleaned up before a project can be deleted
type AtlastProjectDependents struct {
	AdvancedClusters *matlas.AdvancedClustersResponse
}

func (r *ProjectResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *ProjectResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"org_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cluster_count": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"created": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_owner_id": schema.StringAttribute{
				Optional: true,
			},

			"with_default_alerts_settings": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			// Since api_keys is a Computed attribute it will not be added as a Block:
			// https://developer.hashicorp.com/terraform/plugin/framework/migrating/attributes-blocks/blocks-computed
			"api_keys": schema.SetNestedAttribute{
				Optional:           true,
				Computed:           true,
				DeprecationMessage: fmt.Sprintf(DeprecationMessageParameterToResource, "v1.12.0", "mongodbatlas_project_api_key"),
				NestedObject: schema.NestedAttributeObject{

					Attributes: map[string]schema.Attribute{
						"api_key_id": schema.StringAttribute{
							Required: true,
						},
						"role_names": schema.SetAttribute{
							Required:    true,
							ElementType: types.StringType,
						},
					},
				},
				// https://discuss.hashicorp.com/t/computed-attributes-and-plan-modifiers/45830/12
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.UseStateForUnknown(),
				},
			},
			"is_collect_database_specifics_statistics_enabled": schema.BoolAttribute{
				Computed: true,
				Optional: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"is_data_explorer_enabled": schema.BoolAttribute{
				Computed: true,
				Optional: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"is_extended_storage_sizes_enabled": schema.BoolAttribute{
				Computed: true,
				Optional: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"is_performance_advisor_enabled": schema.BoolAttribute{
				Computed: true,
				Optional: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"is_realtime_performance_panel_enabled": schema.BoolAttribute{
				Computed: true,
				Optional: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"is_schema_advisor_enabled": schema.BoolAttribute{
				Computed: true,
				Optional: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"region_usage_restrictions": schema.StringAttribute{
				Optional: true,
			},
		},
		Blocks: map[string]schema.Block{
			"teams": schema.SetNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"team_id": schema.StringAttribute{
							Required: true,
						},
						"role_names": schema.SetAttribute{
							Required:    true,
							ElementType: types.StringType,
						},
					},
				},
			},
		},
	}
}

func (r *ProjectResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*MongoDBClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *MongoDBClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *ProjectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var projectPlan projectResourceModel
	conn := r.client.Atlas

	diags := req.Plan.Get(ctx, &projectPlan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectReq := &matlas.Project{
		OrgID:                     projectPlan.OrgID.ValueString(),
		Name:                      projectPlan.Name.ValueString(),
		WithDefaultAlertsSettings: projectPlan.WithDefaultAlertsSettings.ValueBoolPointer(),
		RegionUsageRestrictions:   projectPlan.RegionUsageRestrictions.ValueString(),
	}

	var createProjectOptions *matlas.CreateProjectOptions

	createProjectOptions = &matlas.CreateProjectOptions{
		ProjectOwnerID: projectPlan.ProjectOwnerID.ValueString(),
	}

	if !projectPlan.ProjectOwnerID.IsNull() {
		createProjectOptions = &matlas.CreateProjectOptions{
			ProjectOwnerID: projectPlan.ProjectOwnerID.ValueString(),
		}
	}

	project, _, err := conn.Projects.Create(ctx, projectReq, createProjectOptions)
	if err != nil {
		resp.Diagnostics.AddError(errorProjectCreate, err.Error())
		return
	}

	// Check if teams were set, if so we need to add the teams into the project
	if len(projectPlan.Teams) > 0 {
		// adding the teams into the project
		_, _, err := conn.Projects.AddTeamsToProject(ctx, project.ID, expandTeamsSet(ctx, projectPlan.Teams))
		if err != nil {
			errd := deleteProject(ctx, conn, project.ID)
			if errd != nil {
				resp.Diagnostics.AddError(fmt.Sprintf(errorProjectDelete, project.ID), err.Error())
				return
			}
			resp.Diagnostics.AddError("error adding teams into the project", err.Error())
			return
		}
	}

	// Check if api keys were set, if so we need to add keys into the project
	if len(projectPlan.ApiKeys) > 0 {
		// assign api keys to the project
		for _, apiKey := range projectPlan.ApiKeys {
			_, err := conn.ProjectAPIKeys.Assign(ctx, project.ID, apiKey.ApiKeyID.ValueString(), &matlas.AssignAPIKey{
				Roles: utils.TypesSetToString(ctx, apiKey.RoleNames),
			})
			if err != nil {
				errd := deleteProject(ctx, conn, project.ID)
				if errd != nil {
					resp.Diagnostics.AddError(fmt.Sprintf(errorProjectDelete, project.ID), err.Error())
					return
				}
				resp.Diagnostics.AddError("error assigning api keys to the project", err.Error())
				return
			}
		}
	}

	projectSettings, _, err := conn.Projects.GetProjectSettings(ctx, project.ID)
	if err != nil {
		errd := deleteProject(ctx, conn, project.ID)
		if errd != nil {
			resp.Diagnostics.AddError(fmt.Sprintf(errorProjectDelete, project.ID), err.Error())
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("error getting project's settings assigned (%s):", project.ID), err.Error())
		return
	}
	projectSettings.IsCollectDatabaseSpecificsStatisticsEnabled = projectPlan.IsCollectDatabaseSpecificsStatisticsEnabled.ValueBoolPointer()
	projectSettings.IsDataExplorerEnabled = projectPlan.IsDataExplorerEnabled.ValueBoolPointer()
	projectSettings.IsExtendedStorageSizesEnabled = projectPlan.IsExtendedStorageSizesEnabled.ValueBoolPointer()
	projectSettings.IsPerformanceAdvisorEnabled = projectPlan.IsPerformanceAdvisorEnabled.ValueBoolPointer()
	projectSettings.IsRealtimePerformancePanelEnabled = projectPlan.IsRealtimePerformancePanelEnabled.ValueBoolPointer()
	projectSettings.IsSchemaAdvisorEnabled = projectPlan.IsSchemaAdvisorEnabled.ValueBoolPointer()

	_, _, err = conn.Projects.UpdateProjectSettings(ctx, project.ID, projectSettings)
	if err != nil {
		errd := deleteProject(ctx, conn, project.ID)
		if errd != nil {
			resp.Diagnostics.AddError(fmt.Sprintf(errorProjectDelete, project.ID), err.Error())
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("error updating project's settings assigned (%s):", project.ID), err.Error())
		return
	}

	// do a Read GET request
	projectID := project.ID
	projectRes, atlasResp, err := conn.Projects.GetOneProject(ctx, projectID)
	if err != nil {
		if resp != nil && atlasResp.StatusCode == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf(errorProjectRead, projectID), err.Error())
		return
	}

	projectPlanPtr, err := getModelWithPropsFromAtlas(ctx, conn, projectRes)
	projectPlan = *projectPlanPtr

	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf(errorProjectRead, projectID), err.Error())
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, projectPlan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func expandTeamsSet(ctx context.Context, teams []team) []*matlas.ProjectTeam {
	res := make([]*matlas.ProjectTeam, len(teams))

	for i, team := range teams {
		res[i] = &matlas.ProjectTeam{
			TeamID:    team.TeamID.ValueString(),
			RoleNames: utils.TypesSetToString(ctx, team.RoleNames),
		}
	}
	return res
}

func (r *ProjectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var projectPlan projectResourceModel
	conn := r.client.Atlas

	// Get current state
	diags := req.State.Get(ctx, &projectPlan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := projectPlan.ID.ValueString()

	projectRes, atlasResp, err := conn.Projects.GetOneProject(ctx, projectID)
	if err != nil {
		if resp != nil && atlasResp.StatusCode == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf(errorProjectRead, projectID), err.Error())
		return
	}

	projectPlanUpdatedPtr, err := getModelWithPropsFromAtlas(ctx, conn, projectRes)
	projectPlanUpdated := *projectPlanUpdatedPtr

	// we need to reset defaults from what was previously in the state:
	// https://discuss.hashicorp.com/t/boolean-optional-default-value-migration-to-framework/55932
	var withDefaultAlertsSettings types.Bool
	req.State.GetAttribute(ctx, path.Root("with_default_alerts_settings"), &withDefaultAlertsSettings)
	projectPlanUpdated.WithDefaultAlertsSettings = withDefaultAlertsSettings

	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf(errorProjectRead, projectID), err.Error())
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &projectPlanUpdated)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func getModelWithPropsFromAtlas(ctx context.Context, conn *matlas.Client, projectRes *matlas.Project) (*projectResourceModel, error) {
	projectID := projectRes.ID
	teams, _, err := conn.Projects.GetProjectTeamsAssigned(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("error getting project's teams assigned (%s): %v", projectID, err.Error())
	}

	apiKeys, err := getProjectAPIKeys(ctx, conn, projectID)
	if err != nil {
		return nil, fmt.Errorf("error getting project's teams assigned (%s): %v", projectID, err.Error())
	}

	projectSettings, _, err := conn.Projects.GetProjectSettings(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("error getting project's settings assigned (%s): %v", projectID, err.Error())
	}

	return toProjectResourceModel(ctx, projectID, projectRes, teams, apiKeys, projectSettings), nil
}

func getProjectAPIKeys(ctx context.Context, conn *matlas.Client, projectID string) ([]matlas.APIKey, error) {
	var filteredKeys []matlas.APIKey
	apiKeys, _, err := conn.ProjectAPIKeys.List(ctx, projectID, &matlas.ListOptions{})

	if err != nil {
		var target *matlas.ErrorResponse
		if errors.As(err, &target) && target.ErrorCode != "USER_UNAUTHORIZED" {
			return nil, fmt.Errorf("error getting project's api keys (%s): %v", projectID, err.Error())
		}
		tflog.Info(ctx, "[WARN] `api_keys` will be empty because the user has no permissions to read the api keys endpoint")
		return filteredKeys, nil
	}

	for _, key := range apiKeys {
		id := key.ID

		var roles []matlas.AtlasRole
		for _, role := range key.Roles {
			// ProjectAPIKeys.List returns all API keys of the Project, including the org and project roles
			// For more details: https://docs.atlas.mongodb.com/reference/api/projectApiKeys/get-all-apiKeys-in-one-project/
			if !strings.HasPrefix(role.RoleName, "ORG_") && role.GroupID == projectID {
				roles = append(roles, role)
			}
		}
		filteredKeys = append(filteredKeys, matlas.APIKey{
			ID:    id,
			Roles: roles,
		})
	}
	return filteredKeys, nil
}

func toProjectResourceModel(ctx context.Context, projectID string, projectRes *matlas.Project, teams *matlas.TeamsAssigned, apiKeys []matlas.APIKey, projectSettings *matlas.ProjectSettings) *projectResourceModel {
	projectPlan := projectResourceModel{
		ID:           types.StringValue(projectID),
		Name:         types.StringValue(projectRes.Name),
		OrgID:        types.StringValue(projectRes.OrgID),
		ClusterCount: types.Int64Value(int64(projectRes.ClusterCount)),
		Created:      types.StringValue(projectRes.Created),
		IsCollectDatabaseSpecificsStatisticsEnabled: types.BoolValue(*projectSettings.IsCollectDatabaseSpecificsStatisticsEnabled),
		IsDataExplorerEnabled:                       types.BoolValue(*projectSettings.IsDataExplorerEnabled),
		IsExtendedStorageSizesEnabled:               types.BoolValue(*projectSettings.IsExtendedStorageSizesEnabled),
		IsPerformanceAdvisorEnabled:                 types.BoolValue(*projectSettings.IsPerformanceAdvisorEnabled),
		IsRealtimePerformancePanelEnabled:           types.BoolValue(*projectSettings.IsRealtimePerformancePanelEnabled),
		IsSchemaAdvisorEnabled:                      types.BoolValue(*projectSettings.IsSchemaAdvisorEnabled),
		Teams:                                       toTeamsResourceModel(ctx, teams),
		ApiKeys:                                     toApiKeysResourceModel(ctx, apiKeys),
	}
	// projectPlan.Name = types.StringValue(projectRes.Name)
	// projectPlan.OrgID = types.StringValue(projectRes.OrgID)
	// projectPlan.ClusterCount = types.Int64Value(int64(projectRes.ClusterCount))
	// projectPlan.Created = types.StringValue(projectRes.Created)
	// projectPlan.IsCollectDatabaseSpecificsStatisticsEnabled = types.BoolValue(*projectSettings.IsCollectDatabaseSpecificsStatisticsEnabled)
	// projectPlan.IsDataExplorerEnabled = types.BoolValue(*projectSettings.IsDataExplorerEnabled)
	// projectPlan.IsExtendedStorageSizesEnabled = types.BoolValue(*projectSettings.IsExtendedStorageSizesEnabled)
	// projectPlan.IsPerformanceAdvisorEnabled = types.BoolValue(*projectSettings.IsPerformanceAdvisorEnabled)
	// projectPlan.IsRealtimePerformancePanelEnabled = types.BoolValue(*projectSettings.IsRealtimePerformancePanelEnabled)
	// projectPlan.IsSchemaAdvisorEnabled = types.BoolValue(*projectSettings.IsSchemaAdvisorEnabled)
	// projectPlan.Teams = convertTeamsToModel(ctx, teams)
	// projectPlan.ApiKeys = convertApiKeysToModel(ctx, apiKeys, projectID)

	return &projectPlan
}

func toApiKeysResourceModel(ctx context.Context, atlasApiKeys []matlas.APIKey) []apiKey {
	res := []apiKey{}

	for _, atlasKey := range atlasApiKeys {
		id := atlasKey.ID

		var atlasRoles []attr.Value
		for _, role := range atlasKey.Roles {
			atlasRoles = append(atlasRoles, types.StringValue(role.RoleName))
		}

		res = append(res, apiKey{
			ApiKeyID:  types.StringValue(id),
			RoleNames: utils.ArrToSetValue(atlasRoles),
		})
	}
	return res
}

func toTeamsResourceModel(ctx context.Context, atlasTeams *matlas.TeamsAssigned) []team {
	if atlasTeams.TotalCount == 0 {
		return nil
	}
	teams := make([]team, atlasTeams.TotalCount)

	for i, atlasTeam := range atlasTeams.Results {
		roleNames, _ := types.SetValueFrom(ctx, types.StringType, atlasTeam.RoleNames)

		teams[i] = team{
			TeamID:    types.StringValue(atlasTeam.TeamID),
			RoleNames: roleNames,
		}
	}
	return teams
}

func (r *ProjectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var projectPlan *projectResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &projectPlan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state.
	resp.Diagnostics.Append(resp.State.Set(ctx, &projectPlan)...)
}

func (r *ProjectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var project *projectResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &project)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := project.ID.ValueString()
	err := deleteProject(ctx, r.client.Atlas, projectID)

	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf(errorProjectDelete, projectID), err.Error())
		return
	}
}

func (r *ProjectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func deleteProject(ctx context.Context, conn *matlas.Client, projectID string) error {
	stateConf := &retry.StateChangeConf{
		Pending:    []string{"DELETING", "RETRY"},
		Target:     []string{"IDLE"},
		Refresh:    resourceProjectDependentsDeletingRefreshFunc(ctx, projectID, conn),
		Timeout:    30 * time.Minute,
		MinTimeout: 30 * time.Second,
		Delay:      0,
	}

	_, err := stateConf.WaitForStateContext(ctx)

	if err != nil {
		tflog.Info(ctx, fmt.Sprintf("[ERROR] could not determine MongoDB project %s dependents status: %s", projectID, err.Error()))
	}

	_, err = conn.Projects.Delete(ctx, projectID)

	return err
}

/*
This assumes the project CRUD outcome will be the same for any non-zero number of dependents

If all dependents are deleting, wait to try and delete
Else consider the aggregate dependents idle.

If we get a defined error response, return that right away
Else retry
*/
func resourceProjectDependentsDeletingRefreshFunc(ctx context.Context, projectID string, client *matlas.Client) retry.StateRefreshFunc {
	return func() (interface{}, string, error) {
		var target *matlas.ErrorResponse
		clusters, _, err := client.AdvancedClusters.List(ctx, projectID, nil)
		dependents := AtlastProjectDependents{AdvancedClusters: clusters}

		if errors.As(err, &target) {
			return nil, "", err
		} else if err != nil {
			return nil, "RETRY", nil
		}

		if dependents.AdvancedClusters.TotalCount == 0 {
			return dependents, "IDLE", nil
		}

		for _, v := range dependents.AdvancedClusters.Results {
			if v.StateName != "DELETING" {
				return dependents, "IDLE", nil
			}
		}

		log.Printf("[DEBUG] status for MongoDB project %s dependents: %s", projectID, "DELETING")

		return dependents, "DELETING", nil
	}
}
