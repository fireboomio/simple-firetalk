package CreatePaymentMembership

import (
	"custom-go/generated"
	"custom-go/pkg/base"
	"custom-go/pkg/plugins"
	"errors"
)

type (
	membershipI    = generated.Membership__GetOneMembershipInput
	membershipRD   = generated.Membership__GetOneMembershipResponseData
	paymentCountI  = generated.Payment__GetPaymentCountByMembershipInternalInput
	paymentCountRD = generated.Payment__GetPaymentCountByMembershipResponseData
)

func PreResolve(hook *base.HookRequest, body generated.Payment__CreatePaymentMembershipBody) (res generated.Payment__CreatePaymentMembershipBody, err error) {
	membershipInput := membershipI{Id: body.Input.MembershipId}
	membershipResp, err := plugins.ExecuteInternalRequestQueries[membershipI, membershipRD](hook.InternalClient, generated.Membership__GetOneMembership, membershipInput)
	if err != nil {
		return
	}

	purchaseLimit := membershipResp.Data.PurchaseLimit
	if purchaseLimit == 0 {
		return
	}

	paymentCountInput := paymentCountI{MembershipId: body.Input.MembershipId, AccountId: body.Input.AccountId}
	paymentCountResp, err := plugins.ExecuteInternalRequestQueries[paymentCountI, paymentCountRD](hook.InternalClient, generated.Payment__GetPaymentCountByMembership, paymentCountInput)
	if err != nil {
		return
	}

	purchaseCount := paymentCountResp.Data.Count
	if purchaseCount >= purchaseLimit {
		err = errors.New("会员购买超限制次数")
		return
	}

	return body, nil
}
