---
subcategory: "Workspace"
---
# databricks_workspace_conf Resource

-> **Note** This resource has an evolving API, which may change in future versions of the provider.

Manages workspace configuration for expert usage. Currently, more than one instance of resource can exist in Terraform state, though there's no deterministic behavior, when they manage the same property. We strongly recommend to use a single `databricks_workspace_conf` per workspace.

## Example Usage

Allows specification of custom configuration properties for expert usage:

 * `enableIpAccessLists` - enables the use of [databricks_ip_access_list](ip_access_list.md) resources
 * `maxTokenLifetimeDays` - (string) Maximum token lifetime of new tokens in days, as an integer. If zero, new tokens are permitted to have no lifetime limit. Negative numbers are unsupported. **WARNING:** This limit only applies to new tokens, so there may be tokens with lifetimes longer than this value, including unlimited lifetime. Such tokens may have been created before the current maximum token lifetime was set. 
 * `enableTokensConfig` - (boolean) Enable or disable personal access tokens for this workspace.

```hcl
resource "databricks_workspace_conf" "this" {
  custom_config = {
    "enableIpAccessLists" : true
  }
}
```

## Argument Reference

The following arguments are available:

* `custom_config` - (Required) Key-value map of strings, that represent workspace configuration. Upon resource deletion, properties that start with `enable` or `enforce` will be reset to `false` value, regardless of initial default one.

## Import

This resource doesn't support import.
