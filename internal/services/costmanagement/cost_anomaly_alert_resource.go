// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package costmanagement

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/lang/response"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/commonids"
	"github.com/hashicorp/go-azure-sdk/resource-manager/costmanagement/2025-03-01/scheduledactionoperationgroup"
	"github.com/hashicorp/go-azure-sdk/resource-manager/costmanagement/2025-03-01/viewoperationgroup"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/costmanagement/validate"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/validation"
	"github.com/hashicorp/terraform-provider-azurerm/utils"
)

var _ sdk.Resource = AnomalyAlertResource{}

type AnomalyAlertResource struct{}

func (AnomalyAlertResource) Arguments() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"name": {
			Type:         pluginsdk.TypeString,
			Required:     true,
			ForceNew:     true,
			ValidateFunc: validate.CostAnomalyAlertName,
			// action names can contain only alphanumeric characters and hyphens.
		},

		"display_name": {
			Type:         pluginsdk.TypeString,
			Required:     true,
			ValidateFunc: validation.StringIsNotEmpty,
		},

		"subscription_id": {
			Type:         pluginsdk.TypeString,
			Optional:     true,
			ForceNew:     true,
			Computed:     true,
			ValidateFunc: commonids.ValidateSubscriptionID,
		},

		"notification_email": {
			Type:         pluginsdk.TypeString,
			Optional:     true,
			Computed:     true,
			ValidateFunc: validation.StringIsNotEmpty,
		},

		"email_subject": {
			Type:         pluginsdk.TypeString,
			Required:     true,
			ValidateFunc: validation.StringLenBetween(1, 70),
		},

		"email_addresses": {
			Type:     pluginsdk.TypeSet,
			Required: true,
			MinItems: 1,
			Elem: &pluginsdk.Schema{
				Type:         pluginsdk.TypeString,
				ValidateFunc: validation.StringIsNotEmpty,
			},
		},

		"message": {
			Type:         pluginsdk.TypeString,
			Optional:     true,
			ValidateFunc: validation.StringLenBetween(1, 250),
		},
	}
}

func (AnomalyAlertResource) Attributes() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{}
}

func (AnomalyAlertResource) ModelObject() interface{} {
	return nil
}

func (AnomalyAlertResource) ResourceType() string {
	return "azurerm_cost_anomaly_alert"
}

func (r AnomalyAlertResource) Create() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 30 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.CostManagement.ScheduledActionsClient

			var subscriptionId string
			if v, ok := metadata.ResourceData.GetOk("subscription_id"); ok {
				subscriptionId = v.(string)
			} else {
				subscriptionId = fmt.Sprint("/subscriptions/", metadata.Client.Account.SubscriptionId)
			}
			id := scheduledactionoperationgroup.NewScopedScheduledActionID(subscriptionId, metadata.ResourceData.Get("name").(string))

			if !metadata.Client.Features.SkipImportCheckOnCreateAndAllowOverwritingExistingResources {
				existing, err := client.ScheduledActionsGetByScope(ctx, id)
				if err != nil && !response.WasNotFound(existing.HttpResponse) {
					return fmt.Errorf("checking for presence of existing %s: %+v", id, err)
				}

				if !response.WasNotFound(existing.HttpResponse) {
					return metadata.ResourceRequiresImport(r.ResourceType(), id)
				}
			}

			emailAddressesRaw := metadata.ResourceData.Get("email_addresses").(*pluginsdk.Set).List()
			emailAddresses := utils.ExpandStringSlice(emailAddressesRaw)

			viewId := viewoperationgroup.NewScopedViewID(subscriptionId, "ms:DailyAnomalyByResourceGroup")

			schedule := scheduledactionoperationgroup.ScheduleProperties{
				Frequency: scheduledactionoperationgroup.ScheduleFrequencyDaily,
			}
			schedule.SetEndDateAsTime(time.Now().AddDate(1, 0, 0))
			schedule.SetStartDateAsTime(time.Now())

			notificationEmail := (*emailAddresses)[0]
			if v, ok := metadata.ResourceData.GetOk("notification_email"); ok {
				notificationEmail = v.(string)
			}
			param := scheduledactionoperationgroup.ScheduledAction{
				Kind: pointer.To(scheduledactionoperationgroup.ScheduledActionKindInsightAlert),
				Properties: &scheduledactionoperationgroup.ScheduledActionProperties{
					DisplayName: metadata.ResourceData.Get("display_name").(string),
					Status:      scheduledactionoperationgroup.ScheduledActionStatusEnabled,
					ViewId:      viewId.ID(),
					FileDestination: &scheduledactionoperationgroup.FileDestination{
						FileFormats: &[]scheduledactionoperationgroup.FileFormat{},
					},
					NotificationEmail: &notificationEmail,
					Notification: scheduledactionoperationgroup.NotificationProperties{
						Subject: metadata.ResourceData.Get("email_subject").(string),
						Message: pointer.To(metadata.ResourceData.Get("message").(string)),
						To:      *emailAddresses,
					},
					Schedule: schedule,
				},
			}
			if _, err := client.ScheduledActionsCreateOrUpdateByScope(ctx, id, param, scheduledactionoperationgroup.DefaultScheduledActionsCreateOrUpdateByScopeOperationOptions()); err != nil {
				return fmt.Errorf("creating %s: %+v", id, err)
			}

			metadata.SetID(id)
			return nil
		},
	}
}

func (r AnomalyAlertResource) Update() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 30 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.CostManagement.ScheduledActionsClient

			id, err := scheduledactionoperationgroup.ParseScopedScheduledActionID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			resp, err := client.ScheduledActionsGetByScope(ctx, *id)
			if err != nil {
				return fmt.Errorf("reading %s: %+v", id, err)
			}

			if model := resp.Model; model != nil {
				if model.ETag == nil {
					return fmt.Errorf("add %s: etag was nil", *id)
				}
			}

			emailAddressesRaw := metadata.ResourceData.Get("email_addresses").(*pluginsdk.Set).List()
			emailAddresses := utils.ExpandStringSlice(emailAddressesRaw)

			var subscriptionId string
			if v, ok := metadata.ResourceData.GetOk("subscription_id"); ok {
				subscriptionId = v.(string)
			} else {
				subscriptionId = fmt.Sprint("/subscriptions/", metadata.Client.Account.SubscriptionId)
			}
			viewId := viewoperationgroup.NewScopedViewID(subscriptionId, "ms:DailyAnomalyByResourceGroup")

			schedule := scheduledactionoperationgroup.ScheduleProperties{
				Frequency: scheduledactionoperationgroup.ScheduleFrequencyDaily,
			}
			schedule.SetEndDateAsTime(time.Now().AddDate(1, 0, 0))
			schedule.SetStartDateAsTime(time.Now())

			notificationEmail := (*emailAddresses)[0]
			if v, ok := metadata.ResourceData.GetOk("notification_email"); ok {
				notificationEmail = v.(string)
			}
			param := scheduledactionoperationgroup.ScheduledAction{
				Kind: pointer.To(scheduledactionoperationgroup.ScheduledActionKindInsightAlert),
				ETag: resp.Model.ETag,
				Properties: &scheduledactionoperationgroup.ScheduledActionProperties{
					DisplayName:       metadata.ResourceData.Get("display_name").(string),
					Status:            scheduledactionoperationgroup.ScheduledActionStatusEnabled,
					ViewId:            viewId.ID(),
					NotificationEmail: &notificationEmail,
					Notification: scheduledactionoperationgroup.NotificationProperties{
						Subject: metadata.ResourceData.Get("email_subject").(string),
						Message: pointer.To(metadata.ResourceData.Get("message").(string)),
						To:      *emailAddresses,
					},
					Schedule: schedule,
				},
			}
			if _, err := client.ScheduledActionsCreateOrUpdateByScope(ctx, *id, param, scheduledactionoperationgroup.DefaultScheduledActionsCreateOrUpdateByScopeOperationOptions()); err != nil {
				return fmt.Errorf("creating %s: %+v", id, err)
			}

			metadata.SetID(id)
			return nil
		},
	}
}

func (AnomalyAlertResource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 5 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.CostManagement.ScheduledActionsClient

			id, err := scheduledactionoperationgroup.ParseScopedScheduledActionID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			resp, err := client.ScheduledActionsGetByScope(ctx, *id)
			if err != nil {
				if response.WasNotFound(resp.HttpResponse) {
					return metadata.MarkAsGone(id)
				}

				return fmt.Errorf("retrieving %s: %+v", id, err)
			}

			if model := resp.Model; model != nil {
				metadata.ResourceData.Set("name", model.Name)
				if props := model.Properties; props != nil {
					metadata.ResourceData.Set("display_name", props.DisplayName)
					metadata.ResourceData.Set("subscription_id", fmt.Sprint("/", *props.Scope))
					metadata.ResourceData.Set("email_subject", props.Notification.Subject)
					metadata.ResourceData.Set("notification_email", props.NotificationEmail)
					metadata.ResourceData.Set("email_addresses", props.Notification.To)
					metadata.ResourceData.Set("message", props.Notification.Message)
				}
			}

			return nil
		},
	}
}

func (AnomalyAlertResource) Delete() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 30 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.CostManagement.ScheduledActionsClient

			id, err := scheduledactionoperationgroup.ParseScopedScheduledActionID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			if _, err = client.ScheduledActionsDeleteByScope(ctx, *id); err != nil {
				return fmt.Errorf("deleting %s: %+v", *id, err)
			}

			return nil
		},
	}
}

func (AnomalyAlertResource) IDValidationFunc() pluginsdk.SchemaValidateFunc {
	return scheduledactionoperationgroup.ValidateScopedScheduledActionID
}
