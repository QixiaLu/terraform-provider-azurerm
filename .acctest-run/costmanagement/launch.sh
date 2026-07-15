#!/usr/bin/env bash
cd /workspace/azurerm
export TF_ACC=1
exec go test -v -timeout 0 \
  -run '^(TestAccBillingAccountCostManagementExport_basic|TestAccCostManagementScheduledAction_basic|TestAccResourceAnomalyAlert_basic|TestAccResourceGroupCostManagementExport_basic|TestAccResourceGroupCostManagementView_basic|TestAccSubscriptionCostManagementExport_basic|TestAccSubscriptionCostManagementView_basic)$' \
  -json \
  -ldflags="-X=github.com/hashicorp/terraform-provider-azurerm/version.ProviderVersion=acc" \
  ./internal/services/costmanagement/...
