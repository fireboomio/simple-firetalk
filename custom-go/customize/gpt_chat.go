package customize

import (
	"bytes"
	"custom-go/generated"
	"custom-go/hooks/Chat/Message/CreateOneChatMessage"
	"custom-go/pkg/base"
	"custom-go/pkg/plugins"
	"custom-go/pkg/utils"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/tidwall/gjson"
)

type (
	GptChatArgs struct {
		ChatId       string `json:"chatId,omitempty"`
		ChatCtxLimit int64  `json:"chatCtxLimit,omitempty"`
		Message      string `json:"message,omitempty"`
		Usage        string `json:"usage"`
		Helper       string `json:"helper,omitempty"`
		HelperArgs   string `json:"helperArgs,omitempty"`
		UserId       string `json:"userId,omitempty"`
	}
	gptChatMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	gptChatBody struct {
		Model    string           `json:"model"`
		Messages []gptChatMessage `json:"messages"`
		Stream   bool             `json:"stream"`
	}
	gptAction struct {
		prepare func(*base.GraphqlRequestContext, *GptChatArgs) ([]gptChatMessage, error)
		done    func(*base.GraphqlRequestContext, *GptChatArgs, string) (string, error)
	}
	gptActionMap      map[gptActionUsage]*gptAction
	gptActionUsage    string
	gptContentPath    string
	handleMessageFunc func(string) ([]gptChatMessage, error)
)

const (
	streamContentPath gptContentPath = "choices.0.delta.content"
	normalContentPath gptContentPath = "choices.0.message.content"
)

type (
	gptModelSetting struct {
		gptURl   string
		gptModel string
		headers  map[string]string
	}
	gptModelType      string
	gptModelActionMap map[gptModelType]*gptModelSetting
)

const (
	promptTeacherRole       = "teacher_role"
	promptStudentRole       = "student_role"
	promptHello             = "chat_hello"
	promptHelperPlaceholder = "${helper}"
)

type (
	chatI           = generated.Chat__GetOneChatInput
	chatRD          = generated.Chat__GetOneChatResponseData
	chatMsgI        = generated.Chat__Message__GetChatMessageListInput
	chatMsgRD       = generated.Chat__Message__GetChatMessageListResponseData
	chatMsgCreateI  = generated.Chat__Message__CreateOneChatMessageInternalInput
	chatMsgCreateRD = generated.Chat__Message__CreateOneChatMessageResponseData
	teacherI        = generated.Teacher__GetOneTeacherInput
	teacherRD       = generated.Teacher__GetOneTeacherResponseData
	sceneI          = generated.Scene__GetOneSceneInput
	sceneRD         = generated.Scene__GetOneSceneResponseData
	userI           = generated.UserSetting__GetOneSettingInternalInput
	userRD          = generated.UserSetting__GetOneSettingResponseData
	promptI         = generated.PromptText__GetOnePromptTextInput
	promptRD        = generated.PromptText__GetOnePromptTextResponseData
)

var (
	chatQueryPath       = generated.Chat__GetOneChat
	chatMsgQueryPath    = generated.Chat__Message__GetChatMessageList
	chatMsgMutationPath = generated.Chat__Message__CreateOneChatMessage
	teacherQueryPath    = generated.Teacher__GetOneTeacher
	sceneQueryPath      = generated.Scene__GetOneScene
	userQueryPath       = generated.UserSetting__GetOneSetting
	promptQueryPath     = generated.PromptText__GetOnePromptText
)

const (
	userRole      = "user"
	systemRole    = "system"
	assistantRole = "assistant"
)

const (
	gptModelTypeCode = "gpt_model_name"
)

func prepareRecentChatMessage(grc *base.GraphqlRequestContext, args *GptChatArgs, messageFunc handleMessageFunc) (result []gptChatMessage, err error) {
	if args.ChatId == "" {
		err = errors.New("chatId must not empty")
		return
	}
	if args.UserId == "" {
		err = errors.New("userId must not empty")
		return
	}

	account, err := CreateOneChatMessage.GetAccountByChatId(grc.InternalClient, args.ChatId)
	if err != nil {
		return
	}

	_, err = CreateOneChatMessage.BeforeChat(grc.InternalClient, args.ChatId, account)
	if err != nil {
		return
	}

	// 查询chat的信息
	chatRes, err := plugins.ExecuteInternalRequestQueries[chatI, chatRD](grc.InternalClient, chatQueryPath, chatI{Id: args.ChatId})
	grc.Logger.Infof("Execute [%s] with error [%v]", chatQueryPath, err)
	if err != nil {
		return
	}
	if chatRes.Data.Id == "" {
		err = fmt.Errorf("not found chat with id [%s]", args.ChatId)
		return
	}

	chatInfo := chatRes.Data
	// 查询teacher的信息
	teacherRes, err := plugins.ExecuteInternalRequestQueries[teacherI, teacherRD](grc.InternalClient, teacherQueryPath, teacherI{Id: chatInfo.TeacherId})
	grc.Logger.Infof("Execute [%s] with error [%v]", teacherQueryPath, err)
	if err != nil {
		return
	}
	if teacherRes.Data.Id == "" {
		err = fmt.Errorf("not found teacher with id [%s]", chatInfo.TeacherId)
		return
	}
	teacherBytes, err := json.Marshal(teacherRes.Data)
	if err != nil {
		return
	}
	teacherJson := string(teacherBytes)

	// 查询scene的信息
	sceneRes, err := plugins.ExecuteInternalRequestQueries[sceneI, sceneRD](grc.InternalClient, sceneQueryPath, sceneI{Id: chatInfo.CurrentSceneId})
	grc.Logger.Infof("Execute [%s] with error [%v]", sceneQueryPath, err)
	if err != nil {
		return
	}
	if sceneRes.Data.Id == "" {
		err = fmt.Errorf("not found scene with id [%s]", chatInfo.CurrentSceneId)
		return
	}
	sceneBytes, err := json.Marshal(sceneRes.Data)
	if err != nil {
		return
	}
	sceneJson := string(sceneBytes)

	userJson, err := getUserJsonStr(grc, args)
	if err != nil {
		return
	}

	argsMessage, err := messageFunc(userJson)
	if err != nil {
		return
	}

	chatMsgInput := chatMsgI{ChatId: args.ChatId, Take: args.ChatCtxLimit}
	chatMsgRes, err := plugins.ExecuteInternalRequestQueries[chatMsgI, chatMsgRD](grc.InternalClient, chatMsgQueryPath, chatMsgInput)
	grc.Logger.Infof("Execute [%s] with error [%v]", chatMsgQueryPath, err)
	if err != nil {
		return
	}

	presetFunc := func(promptType, replaceJson string) {
		promptRes, _ := getPromptContent(grc, promptI{UsageId: promptType})
		if promptRes == "" {
			return
		}

		if replaceJson != "" {
			promptRes = utils.ReplacePlaceholder(replaceJson, promptRes)
		}
		result = append(result, gptChatMessage{Role: systemRole, Content: promptRes})
	}

	// 之前具体场景名称关联prompt scene.prompts
	// 现在场景分类关联prompt，并且具体名称作为占位符替换 scene.sceneCategory.prompts
	// category 演讲 我现在将扮演评委老师，将在${sceneName}中
	// category 用户自定义/(演讲/日常/学习/辩论)/(雅思/托福)
	for _, prompt := range sceneRes.Data.Prompts {
		scenePrompt := utils.ReplacePlaceholder(sceneJson, prompt.Content)
		result = append(result, gptChatMessage{Role: systemRole, Content: scenePrompt})
	}

	presetFunc(promptTeacherRole, teacherJson)
	for _, prompt := range teacherRes.Data.Prompts {
		teacherPrompt := utils.ReplacePlaceholder(teacherJson, prompt.Content)
		result = append(result, gptChatMessage{Role: systemRole, Content: teacherPrompt})
	}
	presetFunc(promptStudentRole, userJson)

	msgSize := len(chatMsgRes.Data)
	if msgSize == 0 {
		presetFunc(promptHello, userJson)
		return
	}

	for i := msgSize - 1; i >= 0; i-- {
		itemData := chatMsgRes.Data[i]
		if itemData.Content == "" {
			continue
		}

		result = append(result, gptChatMessage{Role: itemData.Role, Content: itemData.Content})
	}
	if len(argsMessage) > 0 {
		result = append(result, argsMessage...)
	}
	return
}

func getUserJsonStr(grc *base.GraphqlRequestContext, args *GptChatArgs) (result string, err error) {
	if args.UserId == "" {
		return "{}", nil
	}

	userInput := userI{UserId: args.UserId}
	userRes, err := plugins.ExecuteInternalRequestQueries[userI, userRD](grc.InternalClient, userQueryPath, userInput)
	grc.Logger.Infof("Execute [%s] with error [%v], res [%v]", userQueryPath, err, userRes)
	if err != nil {
		return
	}
	if userRes.Data.Id == "" {
		err = fmt.Errorf("not found user with id [%s]", args.UserId)
		return
	}
	userBytes, err := json.Marshal(userRes.Data)
	if err != nil {
		return
	}
	result = string(userBytes)
	return
}

func preparePromptHelper(grc *base.GraphqlRequestContext, args *GptChatArgs, userJson string) (result []gptChatMessage, err error) {
	if args.Helper == "" {
		err = errors.New("please supply promptText type")
		return
	}

	promptContent, err := getPromptContent(grc, promptI{UsageId: args.Helper})
	if err != nil {
		return
	}

	promptContent = strings.ReplaceAll(promptContent, promptHelperPlaceholder, args.Message)
	if args.HelperArgs != "" {
		if helperArgs := fmt.Sprintf("{%s}", args.HelperArgs); json.Valid([]byte(helperArgs)) {
			promptContent = utils.ReplacePlaceholder(helperArgs, promptContent)
		}
	}

	if userJson == "" {
		userJson, err = getUserJsonStr(grc, args)
		if err != nil {
			return
		}
	}

	promptContent = utils.ReplacePlaceholder(userJson, promptContent)
	result = append(result, gptChatMessage{Role: systemRole, Content: promptContent})
	return
}

func getPromptContent(grc *base.GraphqlRequestContext, promptInput promptI) (content string, err error) {
	promptRes, resErr := plugins.ExecuteInternalRequestQueries[promptI, promptRD](grc.InternalClient, promptQueryPath, promptInput)
	grc.Logger.Infof("Execute [%s] with error [%v]", promptQueryPath, err)
	if resErr != nil {
		err = resErr
		return
	}

	if promptRes.Data.Content == "" {
		err = fmt.Errorf("prompt [%s] not found", promptInput.UsageId)
		return
	}

	content = promptRes.Data.Content
	return
}

func extractGptContent(data []byte, path gptContentPath) string {
	return gjson.Get(string(data), string(path)).String()
}

func GptChatResolve(grc *base.GraphqlRequestContext, args *GptChatArgs, useStream bool, streamKey string) (result string, err error) {
	action, ok := chatActionMap[gptActionUsage(args.Usage)]
	if !ok {
		err = fmt.Errorf("not support chat usage [%s]", args.Usage)
		return
	}

	startPrepare := time.Now()
	msgs, err := action.prepare(grc, args)
	if err != nil {
		return
	}

	donePrepare := time.Since(startPrepare)
	gptSetting := modelActionMap[defaultGptModelType]
	body := gptChatBody{
		Model:    gptSetting.gptModel,
		Messages: msgs,
		Stream:   useStream,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return
	}

	grc.Logger.Infof("GPT ChatId [%s] prepare cost [%s] with payload [%s]", args.ChatId, donePrepare, string(payload))

	// 发送 SSE 请求
	req, err := http.NewRequest(http.MethodPost, gptSetting.gptURl, bytes.NewReader(payload))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")

	for k, v := range gptSetting.headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		err = errors.New(string(bodyBytes))
		grc.Logger.Infof("GPT ChatId [%s] resp non-200 [%d] with [%s]", args.ChatId, resp.StatusCode, string(bodyBytes))
		return
	}

	if !useStream {
		bodyBytes, bodyErr := io.ReadAll(resp.Body)
		if bodyErr != nil {
			err = bodyErr
			return
		}

		normalContent := extractGptContent(bodyBytes, normalContentPath)
		if action.done != nil {
			doneData, doneErr := action.done(grc, args, normalContent)
			if doneErr != nil {
				err = doneErr
				return
			}
			if doneData != "" {
				normalContent = utils.JoinString("[DONE]", normalContent, doneData)
			}
		}

		doneResp := time.Since(startPrepare)
		grc.Logger.Infof("GPT ChatId [%s] doneResp cost [%s] with normalContent [%s]", args.ChatId, doneResp, normalContent)
		result = normalContent
		return
	}

	var gptReply []string
	plugins.HandleSSEReader(resp.Body, grc, func(data []byte) ([]byte, bool) {
		if string(data) == "[DONE]" {
			if action.done != nil {
				doneData, doneErr := action.done(grc, args, strings.Join(gptReply, ""))
				if doneErr != nil {
					grc.Result.Error <- []byte(doneErr.Error())
					return nil, true
				}
				if doneData != "" {
					doneDataBytes, _ := json.Marshal(map[string]string{
						streamKey: string(data) + doneData,
					})
					grc.Result.Data <- doneDataBytes
				}
			}

			doneResp := time.Since(startPrepare)
			grc.Logger.Infof("GPT ChatId [%s] doneResp cost [%s] with gptReply [%s]", args.ChatId, doneResp, strings.Join(gptReply, ""))
			grc.Result.Done <- data
			return nil, true
		}

		streamContent := extractGptContent(data, streamContentPath)
		if len(streamContent) == 0 {
			return nil, false
		}

		resultBytes, resultErr := json.Marshal(map[string]string{
			streamKey: streamContent,
		})
		if resultErr != nil {
			grc.Result.Error <- []byte(resultErr.Error())
			return nil, true
		}

		gptReply = append(gptReply, streamContent)
		return resultBytes, false
	})
	return
}

func buildGptChatFields(keySuffix string, useStream bool) graphql.Fields {
	return graphql.Fields{
		keySuffix: &graphql.Field{
			Type: graphql.String,
			Args: graphql.FieldConfigArgument{
				"chatId": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"chatCtxLimit": &graphql.ArgumentConfig{
					Type:         graphql.Int,
					DefaultValue: 10,
				},
				"message": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"usage": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"helper": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"userId": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"helperArgs": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
			},
			Resolve: func(params graphql.ResolveParams) (result interface{}, err error) {
				// 准备 OpenAI 请求体数据
				grc, args, err := plugins.ResolveArgs[GptChatArgs](params)
				if err != nil {
					return
				}
				result, err = GptChatResolve(grc, args, useStream, params.Info.Path.Key.(string))
				return
			},
		},
	}
}

var Gpt_chat_schema, _ = graphql.NewSchema(graphql.SchemaConfig{
	Query:        graphql.NewObject(graphql.ObjectConfig{Name: "query", Fields: buildGptChatFields("query", false)}),
	Subscription: graphql.NewObject(graphql.ObjectConfig{Name: "subscription", Fields: buildGptChatFields("subscription", true)}),
})

var (
	defaultGptModelType = openai4
	chatActionMap       gptActionMap
	modelActionMap      gptModelActionMap
)

const (
	chatOnce         gptActionUsage = "chat_once"          // 不需要上下文，仅一次发问
	chatSimple       gptActionUsage = "chat_simple"        // 简单的聊天
	chatAnswerHelper gptActionUsage = "chat_answer_helper" // 聊天中的回答提示
	chatPromptHelper gptActionUsage = "chat_prompt_helper" // 预设的prompt，可以使用message替换${helper}
)

func init() {
	base.AddRegisteredHook(func(logger echo.Logger) {
		if modelType, _ := CreateOneChatMessage.GetDictISetting(plugins.DefaultInternalClient, gptModelTypeCode); modelType != "" {
			gptType := gptModelType(modelType)
			if _, ok := modelActionMap[gptType]; ok {
				defaultGptModelType = gptType
			} else {
				logger.Warnf("not support gptModelType [%s], and use default [%s]", gptType, defaultGptModelType)
			}
		}
	})

	chatActionMap = make(gptActionMap, 0)

	chatActionMap[chatSimple] = &gptAction{
		prepare: func(grc *base.GraphqlRequestContext, args *GptChatArgs) ([]gptChatMessage, error) {
			return prepareRecentChatMessage(grc, args, func(string) ([]gptChatMessage, error) {
				return nil, nil
			})
		},
		done: func(grc *base.GraphqlRequestContext, args *GptChatArgs, gptReply string) (result string, err error) {
			chatMsgCreateInput := chatMsgCreateI{Role: assistantRole, Content: gptReply, ChatId: args.ChatId, UpdatedAt: utils.CurrentDateTime(), CreateWith: "Content"}
			chatMsgCreateResp, err := plugins.ExecuteInternalRequestMutations[chatMsgCreateI, chatMsgCreateRD](grc.InternalClient, chatMsgMutationPath, chatMsgCreateInput)
			if err != nil {
				return
			}

			result = chatMsgCreateResp.Data.Id
			return
		},
	}
	chatActionMap[chatAnswerHelper] = &gptAction{
		prepare: func(grc *base.GraphqlRequestContext, args *GptChatArgs) ([]gptChatMessage, error) {
			return prepareRecentChatMessage(grc, args, func(userJson string) (result []gptChatMessage, err error) {
				return preparePromptHelper(grc, args, userJson)
			})
		},
	}
	chatActionMap[chatOnce] = &gptAction{
		prepare: func(grc *base.GraphqlRequestContext, args *GptChatArgs) (result []gptChatMessage, err error) {
			if args.Message == "" {
				err = errors.New("message must not empty")
				return
			}
			result = append(result, gptChatMessage{Role: userRole, Content: args.Message})
			return
		},
	}
	chatActionMap[chatPromptHelper] = &gptAction{
		prepare: func(grc *base.GraphqlRequestContext, args *GptChatArgs) ([]gptChatMessage, error) {
			return preparePromptHelper(grc, args, "")
		},
	}

	modelActionMap = make(gptModelActionMap, 0)
}
