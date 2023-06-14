package CreateOneChatMessage

import (
	"custom-go/generated"
	"custom-go/pkg/base"
	"custom-go/pkg/plugins"
)

type (
	accountCostDurationI    = generated.Account__CostAccountDurationInput
	accountCostDurationRD   = generated.Account__CostAccountDurationResponseData
	durationHistoryCreateI  = generated.Duration__CreateCostDurationHistoryInternalInput
	durationHistoryCreateRD = generated.Duration__CreateCostDurationHistoryResponseData
)

func PostResolve(hook *base.HookRequest, body generated.Chat__Message__CreateOneChatMessageBody) (res generated.Chat__Message__CreateOneChatMessageBody, err error) {
	accountRes, err := GetAccountByChatId(hook.InternalClient, body.Input.ChatId)
	if err != nil {
		return
	}

	respData := body.Response.Data.Data
	accountId := accountRes.Data.Id
	cost := respData.CostDuration
	durationHistoryCreateInput := durationHistoryCreateI{
		MessageId: respData.Id,
		AccountId: accountId,
		Value:     cost,
	}
	_, err = plugins.ExecuteInternalRequestMutations[durationHistoryCreateI, durationHistoryCreateRD](hook.InternalClient, generated.Duration__CreateCostDurationHistory, durationHistoryCreateInput)
	if err != nil {
		return
	}

	cost = cost - respData.OutTimeDuration
	if cost == 0 {
		return body, nil
	}
	accountUpdateInput := accountCostDurationI{Duration: cost, Id: accountId}
	_, err = plugins.ExecuteInternalRequestMutations[accountCostDurationI, accountCostDurationRD](hook.InternalClient, generated.Account__CostAccountDuration, accountUpdateInput)
	if err != nil {
		return
	}
	return body, nil
}
