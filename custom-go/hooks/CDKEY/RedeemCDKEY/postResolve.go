package RedeemCDKEY

import (
	"custom-go/generated"
	"custom-go/pkg/base"
	"custom-go/pkg/plugins"
)

type (
	durationHistoryCreateI = generated.Duration__CreateCDKEYDurationHistoryInternalInput
	accountDurationFetchI  = generated.Account__FetchAccountDurationInput
)

func PostResolve(hook *base.HookRequest, body generated.CDKEY__RedeemCDKEYBody) (res generated.CDKEY__RedeemCDKEYBody, err error) {
	if body.Response == nil || hook.User == nil {
		return
	}

	data := body.Response.Data.Data
	durationHistoryCreateInput := durationHistoryCreateI{
		CKDKEYId:  data.Id,
		AccountId: hook.User.UserId,
	}
	if membership := data.Membership; membership.Id != "" {
		durationHistoryCreateInput.Value = membership.PresentDuration
	}
	if durationPackage := data.DurationPackage; durationPackage.Id != "" {
		durationHistoryCreateInput.Value = durationPackage.Value
	}
	_, err = plugins.ExecuteInternalRequestMutations[durationHistoryCreateI, any](hook.InternalClient, generated.Duration__CreateCDKEYDurationHistory, durationHistoryCreateInput)
	if err != nil {
		return
	}

	// 更新剩余时长
	accountDurationFetchInput := accountDurationFetchI{Duration: durationHistoryCreateInput.Value, Id: durationHistoryCreateInput.AccountId}
	_, err = plugins.ExecuteInternalRequestMutations[accountDurationFetchI, any](hook.InternalClient, generated.Account__FetchAccountDuration, accountDurationFetchInput)
	return body, nil
}
