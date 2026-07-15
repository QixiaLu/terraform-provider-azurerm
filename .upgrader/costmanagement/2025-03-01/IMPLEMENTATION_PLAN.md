# Upgrade plan: costmanagement 2023-08-01 -> 2025-03-01

Shared task list across iterations. Keep current; future iterations start fresh.

- [x] Confirm target API version is published; bump go-azure-sdk if needed.
- [x] Update imports/clients/models/enums to target version.
- [x] go mod tidy && go mod vendor.
- [x] Assess and Address breaking change as instructed in `contributing/topics/guide_breaking_change.md`
- [x] Fix compile errors until `go build ./...` is green.

## Notes

### iter-02 findings (2025-03-01 upgrade)

**API version 2025-03-01 is present in the already-pinned go-azure-sdk v0.20260608.1152044 — no go.mod bump needed.**

#### Structural change in 2025-03-01
In 2023-08-01, scoped operations (views and scheduled-actions by scope) lived alongside the non-scoped operations in the same `views` and `scheduledactions` packages.
In 2025-03-01, the SDK splits them into dedicated packages:
- `viewoperationgroup` — contains `ScopedViewId` and `ViewsCreateOrUpdateByScope`, `ViewsGetByScope`, `ViewsDeleteByScope`
- `scheduledactionoperationgroup` — contains `ScopedScheduledActionId` and `ScheduledActionsCreateOrUpdateByScope`, `ScheduledActionsGetByScope`, `ScheduledActionsDeleteByScope`
- `exports` — unchanged structure, only version bump

#### Breaking-change assessment
- Method renames: `GetByScope` → `ScheduledActionsGetByScope` / `ViewsGetByScope`, etc. — **compile-time only, no behavioral change**
- `CheckNameAvailability` + `CheckNameAvailabilityByScope` removed from `scheduledactionoperationgroup` — these were never used in the provider
- `View` model gains a `SystemData` read-only field — no schema/behavior impact
- `exports` models gain `ExportRunRequest`, `ExportSuspensionContext`, `FilterItems` — not used; no impact
- No default value, required-field, or enum changes; no schema changes required; no upgrade guide needed

#### Files changed
- `client/client.go` — `ScheduledActionsClient` type changed to `*scheduledactionoperationgroup.ScheduledActionOperationGroupClient`, `ViewsClient` to `*viewoperationgroup.ViewOperationGroupClient`
- `view_resource_base.go`, `resource_group_cost_management_view_resource.go`, `subscription_cost_management_view_resource.go` — `views` → `viewoperationgroup`
- `cost_management_scheduled_action_resource.go`, `cost_anomaly_alert_resource.go` — `scheduledactions` → `scheduledactionoperationgroup`, `views` → `viewoperationgroup`
- `export_resource_base.go`, `billing_account_cost_management_export_resource.go`, `resource_group_cost_management_export_resource.go`, `subscription_cost_management_export_resource.go` — `2023-08-01/exports` → `2025-03-01/exports`
- All `_test.go` files — matching import and method-call updates

#### Build results
- `go build ./internal/services/costmanagement/...` ✅ exit 0
- `go build ./...` ✅ exit 0
- `vendor/` updated: old `2023-08-01/{exports,scheduledactions,views}` replaced by `2025-03-01/{exports,scheduledactionoperationgroup,viewoperationgroup}`