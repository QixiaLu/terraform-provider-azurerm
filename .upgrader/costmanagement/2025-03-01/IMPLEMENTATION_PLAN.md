# Upgrade plan: costmanagement 2023-08-01 -> 2025-03-01

Shared task list across iterations. Keep current; future iterations start fresh.

- [x] Confirm target API version is published; bump go-azure-sdk if needed.
- [x] Update imports/clients/models/enums to target version.
- [x] go mod tidy && go mod vendor.
- [x] Assess and Address breaking change as instructed in `contributing/topics/guide_breaking_change.md`
- [x] Fix compile errors until `go build ./...` is green.

## Notes

### Iteration 1 - API Import and Breaking Changes Fixed

**Completed:**
- Target API 2025-03-01 is available in pinned go-azure-sdk (no version bump needed)
- Updated all imports in client/client.go and resource files (11 files total)
- Ran go mod tidy && go mod vendor successfully
- Vendored packages: exports, scheduledactions, views, **scheduledactionoperationgroup**, **viewoperationgroup** at 2025-03-01
- Fixed all compile errors - costmanagement service builds cleanly

**Breaking Changes Identified and Addressed:**

1. **scheduledactions / views package reorganization (API redesign)**
   - **OLD (2023-08-01)**: Single packages with `*ByScope` methods for scoped operations
     - `scheduledactions.ScopedScheduledActionId` + methods `GetByScope`, `CreateOrUpdateByScope`, `DeleteByScope`
     - `views.ScopedViewID` + methods `GetByScope`, `CreateOrUpdateByScope`, `DeleteByScope`
   - **NEW (2025-03-01)**: Separate packages for tenant-level vs scoped operations
     - `scheduledactions` / `views`: tenant-level operations (non-scoped)
     - **`scheduledactionoperationgroup`** / **`viewoperationgroup`**: scoped operations with prefixed method names
     - Method naming changed: `GetByScope` → `ScheduledActionsGetByScope` / `ViewsGetByScope`
     - ID types preserved: `ScopedScheduledActionId` and `ScopedViewID` moved to operation group packages

**Impact:** 
   - No user-facing breaking changes - scoped operations still supported, just moved to different packages
   - All affected resources updated:
     - `cost_anomaly_alert_resource.go`: uses scheduledactionoperationgroup + viewoperationgroup
     - `cost_management_scheduled_action_resource.go`: uses scheduledactionoperationgroup + viewoperationgroup  
     - `resource_group_cost_management_view_resource.go`: uses viewoperationgroup
     - `subscription_cost_management_view_resource.go`: uses viewoperationgroup
     - `view_resource_base.go`: uses viewoperationgroup
   - Client struct expanded to include both operation group clients

**Files Modified (13):**
1. `client/client.go` - added ScheduledActionOperationGroupClient and ViewOperationGroupClient
2. `cost_anomaly_alert_resource.go` - updated to scheduledactionoperationgroup/viewoperationgroup
3. `cost_management_scheduled_action_resource.go` - updated to scheduledactionoperationgroup/viewoperationgroup
4. `resource_group_cost_management_view_resource.go` - updated to viewoperationgroup
5. `subscription_cost_management_view_resource.go` - updated to viewoperationgroup
6. `view_resource_base.go` - updated to viewoperationgroup
7-13. Export resources (7 files) - updated imports only (exports API unchanged)

**Vendor Changes:**
- Added: `costmanagement/2025-03-01/scheduledactionoperationgroup/`, `costmanagement/2025-03-01/viewoperationgroup/`
- Updated: `costmanagement/2025-03-01/exports/`, `scheduledactions/`, `views/`
- Removed: `costmanagement/2023-08-01/*` (all old API versions)

**Build Status:**
- ✅ `go build ./internal/services/costmanagement/...` passes
- ✅ `go build ./...` passes (exit code 0)

**Assessment:**
- **No user-facing breaking changes** - all functionality preserved
- API reorganization is internal only - resource schemas/behavior unchanged
- Migration is API-compatible: scoped operations still work, just different package location

### Iteration 2 - Test File Updates

**Completed:**
- Updated 7 test files to use 2025-03-01 API:
  1. `billing_account_cost_management_export_resource_test.go` - exports package updated
  2. `cost_anomaly_alert_resource_test.go` - updated to scheduledactionoperationgroup + new method names
  3. `cost_management_scheduled_action_resource_test.go` - updated to scheduledactionoperationgroup + new method names
  4. `resource_group_cost_management_export_resource_test.go` - exports package updated
  5. `resource_group_cost_management_view_resource_test.go` - updated to viewoperationgroup + new method names
  6. `subscription_cost_management_export_resource_test.go` - exports package updated
  7. `subscription_cost_management_view_resource_test.go` - updated to viewoperationgroup + new method names

**Changes Made:**
- Updated imports from 2023-08-01 to 2025-03-01
- Updated scheduledactions → scheduledactionoperationgroup in test Exists methods
- Updated views → viewoperationgroup in test Exists methods
- Updated method calls: GetByScope → ScheduledActionsGetByScope / ViewsGetByScope
- Updated ID parsers to use operation group packages

**Build Status:**
- ✅ `go build ./internal/services/costmanagement/...` passes
- ✅ `go build ./...` passes (exit code 0)
- ✅ No remaining 2023-08-01 references in costmanagement service

**Final Status:**
- All costmanagement code updated to 2025-03-01
- All imports, clients, and test files using new API
- Full provider builds successfully
- Upgrade complete