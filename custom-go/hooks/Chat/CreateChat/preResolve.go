package CreateChat

import (
	"custom-go/generated"
	"custom-go/pkg/base"
	"custom-go/pkg/plugins"
	"errors"
)

type (
	accountI  = generated.Account__GetOneAccountInternalInput
	accountRD = generated.Account__GetOneAccountResponseData
)

func PreResolve(hook *base.HookRequest, body generated.Chat__CreateChatBody) (res generated.Chat__CreateChatBody, err error) {
	accountInput := accountI{UserId: body.Input.UserId}
	accountRes, err := plugins.ExecuteInternalRequestQueries[accountI, accountRD](hook.InternalClient, generated.Account__GetOneAccount, accountInput)
	if err != nil {
		return
	}

	if accountRes.Data.LeftDuration == 0 {
		return nil, errors.New("剩余时长为0[ERROR]10001")
	}
	return body, nil
}
