package mongodbatlas

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/id"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	matlas "go.mongodb.org/atlas/mongodbatlas"
)

func dataSourceMongoDBAtlasCustomDBRoles() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceMongoDBAtlasCustomDBRolesRead,
		Schema: map[string]*schema.Schema{
			"project_id": {
				Type:     schema.TypeString,
				Required: true,
			},
			"results": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"role_name": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"actions": {
							Type:     schema.TypeList,
							Computed: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"action": {
										Type:     schema.TypeString,
										Computed: true,
									},
									"resources": {
										Type:     schema.TypeSet,
										Computed: true,
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"collection_name": {
													Type:     schema.TypeString,
													Computed: true,
												},
												"database_name": {
													Type:     schema.TypeString,
													Computed: true,
												},
												"cluster": {
													Type:     schema.TypeBool,
													Computed: true,
												},
											},
										},
									},
								},
							},
						},
						"inherited_roles": {
							Type:     schema.TypeList,
							Computed: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"database_name": {
										Type:     schema.TypeString,
										Computed: true,
									},
									"role_name": {
										Type:     schema.TypeString,
										Computed: true,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func dataSourceMongoDBAtlasCustomDBRolesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*MongoDBClient).Atlas
	projectID := d.Get("project_id").(string)

	customDBRoles, _, err := conn.CustomDBRoles.List(ctx, projectID, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error getting custom db roles information: %s", err))
	}

	if err := d.Set("results", flattenCustomDBRoles(*customDBRoles)); err != nil {
		return diag.FromErr(fmt.Errorf("error setting `results for custom db roles: %s", err))
	}

	d.SetId(id.UniqueId())

	return nil
}

func flattenCustomDBRoles(customDBRoles []matlas.CustomDBRole) []map[string]interface{} {
	var customDBRolesMap []map[string]interface{}

	if len(customDBRoles) > 0 {
		customDBRolesMap = make([]map[string]interface{}, len(customDBRoles))

		for k, customDBRole := range customDBRoles {
			customDBRolesMap[k] = map[string]interface{}{
				"role_name":       customDBRole.RoleName,
				"actions":         flattenActions(customDBRole.Actions),
				"inherited_roles": flattenInheritedRoles(customDBRole.InheritedRoles),
			}
		}
	}

	return customDBRolesMap
}
