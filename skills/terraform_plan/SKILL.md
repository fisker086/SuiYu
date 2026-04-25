---
name: terraform-plan
description: Analyze Terraform .tf files locally: list resources, check config, preview changes
activation_keywords: [terraform, tf, plan, infrastructure, iac, hcl, resource, module]
execution_mode: client
---

# Terraform Plan Skill

Provides read-only local Terraform file analysis:
- Scan .tf files and list all resources to be created
- Show providers and modules used
- Validate HCL syntax
- Summarize infrastructure changes

Use `builtin_terraform_plan` tool with fields:
- `operation`: one of "resources", "providers", "modules", "validate", "summary"
- `path`: path to terraform directory (default: current directory)

Note: Does NOT require terraform CLI installed. Parses .tf files directly.
All operations are read-only.
