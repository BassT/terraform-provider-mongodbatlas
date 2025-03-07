---
layout: "mongodbatlas"
page_title: "MongoDB Atlas: mongodbatlas_cloud_provider_access"
sidebar_current: "docs-mongodbatlas-datasource-cloud-provider-access"
description: |-
    Allows you to get the list of cloud provider access roles
---

**WARNING:** The data source `mongodbatlas_cloud_provider_access` is deprecated and will be removed in version v1.14.0, use the data source `mongodbatlas_cloud_provider_access_setup` instead.

# Data Source: mongodbatlas_cloud_provider_access

`mongodbatlas_cloud_provider_access` allows you to get the list of cloud provider access roles, currently only AWS and Azure is supported.

-> **NOTE:** Groups and projects are synonymous terms. You may find `groupId` in the official documentation.

## Example Usage
```terraform
resource "mongodbatlas_cloud_provider_access" "test_role" {
   project_id = "64259ee860c43338194b0f8e"
   provider_name = "AWS"
}

data "mongodbatlas_cloud_provider_access" "all" {
   project_id = mongodbatlas_cloud_provider_access.test_role.project_id
}
```

## Argument Reference

* `project_id` - (Required) The unique ID for the project to get all Cloud Provider Access 

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - Autogenerated Unique ID for this data source.
* `aws_iam_roles` - A list where each represents a Cloud Provider Access Role.

### Each element in the aws_iam_roles array consists in an object with the following elements

* `atlas_assumed_role_external_id` - Unique external ID Atlas uses when assuming the IAM role in your AWS account.
* `atlas_aws_account_arn`          - ARN associated with the Atlas AWS account used to assume IAM roles in your AWS account.
* `authorized_date`                - Date on which this role was authorized.
* `created_date`                   - Date on which this role was created.
* `feature_usages`                 - Atlas features this AWS IAM role is linked to.
* `iam_assumed_role_arn`           - ARN of the IAM Role that Atlas assumes when accessing resources in your AWS account.
* `provider_name`                  - Name of the cloud provider. Currently limited to AWS.
* `role_id`                        - Unique ID of this role.


See [MongoDB Atlas API](https://docs.atlas.mongodb.com/reference/api/cloud-provider-access-get-roles/) Documentation for more information.
