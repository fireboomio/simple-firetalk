package CreateOneUserScene

import (
	"custom-go/generated"
	"custom-go/hooks/Chat/Message/CreateOneChatMessage"
	"custom-go/pkg/base"
	"custom-go/pkg/plugins"
	"errors"
	"strconv"
	"unicode/utf8"
)

const (
	userSceneLengthLimit  = "user_scene_len_limit"
	FindSceneCategoryPath = generated.Scene__FindFirstSceneCategory
)

type (
	sceneCategoryIn  = generated.Scene__FindFirstSceneCategoryInternalInput
	sceneCategoryOut = generated.Scene__FindFirstSceneCategoryResponseData
)

func MutatingPreResolve(hook *base.HookRequest, body generated.Scene__CreateOneUserSceneBody) (res generated.Scene__CreateOneUserSceneBody, err error) {
	//判断用户自定义场景名是否超长
	lengthLimit, err := CreateOneChatMessage.GetDictISetting(hook.InternalClient, userSceneLengthLimit)
	if err != nil {
		lengthLimit = "20"
		err = nil
	}

	lengthLimitIntVal, _ := strconv.Atoi(lengthLimit)
	if utf8.RuneCountInString(body.Input.Name) > lengthLimitIntVal {
		return res, errors.New("自定义场景名称超长")
	}

	//设置场景类型为用户自定义
	categoryId, err := plugins.ExecuteInternalRequestQueries[sceneCategoryIn, sceneCategoryOut](hook.InternalClient, FindSceneCategoryPath, sceneCategoryIn{})
	if err != nil {
		hook.Logger().Errorf("Execute query user scene category id failed: %v", err)
		return
	}

	body.Input.CategoryId = categoryId.Data.Id
	res = body
	return
}
