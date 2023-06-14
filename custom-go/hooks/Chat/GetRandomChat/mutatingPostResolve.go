package GetRandomChat

import (
	"custom-go/generated"
	"custom-go/pkg/base"
	"custom-go/pkg/plugins"
	"custom-go/pkg/utils"
	"errors"
)

type (
	createChatI  = generated.Chat__CreateChatInternalInput
	createChatRD = generated.Chat__CreateChatResponseData
)

func MutatingPostResolve(hook *base.HookRequest, body generated.Chat__GetRandomChatBody) (res generated.Chat__GetRandomChatBody, err error) {
	if hook.User == nil {
		err = errors.New("用户操作前必须登录")
		return
	}

	if body.Response == nil || body.Response.Data.Data.Id != "" {
		return
	}

	data := body.Response.Data
	createChatInput := createChatI{
		SceneId:   data.SceneId,
		TeacherId: data.TeacherId,
		UpdatedAt: utils.CurrentDateTime(),
		UserId:    hook.User.UserId,
	}
	chatRes, err := plugins.ExecuteInternalRequestMutations[createChatI, createChatRD](hook.InternalClient, generated.Chat__CreateChat, createChatInput)
	if err != nil {
		return
	}

	body.Response.Data.Data.Id = chatRes.Data.Id
	return body, nil
}
