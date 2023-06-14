package CreateOneChatMessage

import (
	"custom-go/generated"
	"custom-go/pkg/base"
	"custom-go/pkg/plugins"
	"errors"
	"fmt"
	"strconv"
)

type (
	accountByChatIdI  = generated.Account__GetAccountByChatIdInternalInput
	accountByChatIdRD = generated.Account__GetAccountByChatIdResponseData
	dictI             = generated.Dict__GetDictDefaultValueInput
	dictRD            = generated.Dict__GetDictDefaultValueResponseData
	chatTOI           = generated.Chat__Message__GetChatOutTimeDurationInput
	chatTORD          = generated.Chat__Message__GetChatOutTimeDurationResponseData
)

const (
	contentCostCode  = "calculate_content_cost"
	chatOutTimeLimit = "chat_out_time_limit"
)

const (
	audio         createWith = "Audio"
	nativeAudio   createWith = "NativeAudio"
	content       createWith = "Content"
	nativeContent createWith = "NativeContent"
)

type (
	calculateDuration    func(hook *base.HookRequest, body generated.Chat__Message__CreateOneChatMessageBody) (float64, error)
	createWith           string
	calculateDurationMap map[createWith]calculateDuration
)

var (
	calculateMap            calculateDurationMap
	chatTimeOutDurationPath = generated.Chat__Message__GetChatOutTimeDuration
)

func init() {
	calculateMap = make(calculateDurationMap, 0)
	calculateMap[audio] = calculateAudioCost
	calculateMap[nativeAudio] = calculateAudioCost
	calculateMap[content] = func(hook *base.HookRequest, body generated.Chat__Message__CreateOneChatMessageBody) (float64, error) {
		return calculateContentCost(hook, body.Input.Content)
	}
	calculateMap[nativeContent] = func(hook *base.HookRequest, body generated.Chat__Message__CreateOneChatMessageBody) (float64, error) {
		return calculateContentCost(hook, body.Input.NativeContent)
	}
}

func GetAccountByChatId(internalClient *base.InternalClient, chatId string) (result accountByChatIdRD, err error) {
	accountInput := accountByChatIdI{ChatId: chatId}
	return plugins.ExecuteInternalRequestQueries[accountByChatIdI, accountByChatIdRD](internalClient, generated.Account__GetAccountByChatId, accountInput)
}

func MutatingPreResolve(hook *base.HookRequest, body generated.Chat__Message__CreateOneChatMessageBody) (res generated.Chat__Message__CreateOneChatMessageBody, err error) {

	accountRes, err := GetAccountByChatId(hook.InternalClient, body.Input.ChatId)
	if err != nil {
		return
	}

	isSuper, err := BeforeChat(hook.InternalClient, body.Input.ChatId, accountRes)
	if err != nil {
		return
	}

	accountData := accountRes.Data
	if accountData.Id == "" {
		err = fmt.Errorf("account not found for chatId [%s]", body.Input.ChatId)
		return
	}

	calculate, ok := calculateMap[createWith(body.Input.CreateWith)]
	if !ok {
		return nil, fmt.Errorf("not support createWith [%s]", body.Input.CreateWith)
	}

	cost, err := calculate(hook, body)
	if err != nil {
		return
	}

	body.Input.IsSuper = isSuper
	//时长超额了 需要记录超额时长
	left := accountData.LeftDuration
	if !isSuper && cost > left {
		body.Input.OutTimeDuration = cost - left
	}

	body.Input.CostDuration = cost
	return body, nil
}

func calculateAudioCost(hook *base.HookRequest, body generated.Chat__Message__CreateOneChatMessageBody) (cost float64, err error) {
	if body.Input.AudioDuration == 0 {
		err = fmt.Errorf("audioDuration must not empty by createWith [%s]", body.Input.CreateWith)
		return
	}

	cost = body.Input.AudioDuration
	return
}

func calculateContentCost(hook *base.HookRequest, content string) (cost float64, err error) {
	dictInput := dictI{Code: contentCostCode}
	dictRes, err := plugins.ExecuteInternalRequestQueries[dictI, dictRD](hook.InternalClient, generated.Dict__GetDictDefaultValue, dictInput)
	if err != nil {
		return
	}

	if dictRes.Data.Id == "" {
		err = fmt.Errorf("dict [%s] not found", contentCostCode)
		return
	}

	costDict, err := strconv.ParseFloat(dictRes.Data.Value, 64)
	if err != nil {
		return
	}

	cost = float64(len(content)) * costDict
	return
}

// BeforeChat 处理不同类型用户聊天流程
func BeforeChat(internalClient *base.InternalClient, chatId string, account accountByChatIdRD) (isSuper bool, err error) {
	//判断是否是超级会员
	presentDuration := account.Data.Membership.PresentDuration
	if presentDuration == -1 {
		isSuper = true
		return
	}

	//不是超级会员
	//如果无剩余时长，允许超时一定额度
	if account.Data.LeftDuration != 0 {
		return
	}

	//查询本次会话超额时长总和，上限默认1000，从字典表取
	chatOutTimeRes, err := plugins.ExecuteInternalRequestQueries[chatTOI, chatTORD](internalClient, chatTimeOutDurationPath, chatTOI{ChatId: chatId})
	if err != nil {
		return
	}

	limit, _ := GetDictISetting(internalClient, chatOutTimeLimit)
	limitFloatVal, err := strconv.ParseFloat(limit, 64)
	if err != nil {
		limitFloatVal = 1000
		err = nil
	}
	//会话中消息的超时总和 > 限制额度  则返回，不再创建消息
	if chatOutTimeRes.Data.OutTimeDuration > limitFloatVal {
		err = errors.New("剩余时长为0[ERROR]10001")
		return
	}
	return
}

func GetDictISetting(internalClient *base.InternalClient, setting string) (string, error) {
	dictInput := dictI{Code: setting}
	dictRes, err := plugins.ExecuteInternalRequestQueries[dictI, dictRD](internalClient, generated.Dict__GetDictDefaultValue, dictInput)
	if err != nil {
		return "", err
	}

	if dictRes.Data.Id == "" {
		return "", fmt.Errorf("dict [%s] not found", setting)
	}
	return dictRes.Data.Value, nil
}
