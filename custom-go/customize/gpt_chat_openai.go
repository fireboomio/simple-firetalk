package customize

import (
	"custom-go/hooks/Chat/Message/CreateOneChatMessage"
	"custom-go/pkg/base"
	"custom-go/pkg/plugins"
	"github.com/labstack/echo/v4"
)

const (
	openaiUrl           = "https://api.openai.com/v1/chat/completions"
	openaiAccessKeyName = "Authorization"
	openaiAccessKeyCode = "openai_accessKey"

	openai35      gptModelType = "openai3.5"
	openai35Model              = "gpt-3.5-turbo"
	openai4       gptModelType = "openai4"
	openai4Model               = "gpt-4"
)

var openaiHeaders = map[string]string{
	openaiAccessKeyName: "Bearer ${openaiAccessKey}",
}

func init() {
	base.AddRegisteredHook(func(_ echo.Logger) {
		if accessKey, _ := CreateOneChatMessage.GetDictISetting(plugins.DefaultInternalClient, openaiAccessKeyCode); accessKey != "" {
			openaiHeaders[openaiAccessKeyName] = "Bearer " + accessKey
		}
	})

	modelActionMap[openai35] = &gptModelSetting{
		gptModel: openai35Model,
		gptURl:   openaiUrl,
		headers:  openaiHeaders,
	}
	modelActionMap[openai4] = &gptModelSetting{
		gptModel: openai4Model,
		gptURl:   openaiUrl,
		headers:  openaiHeaders,
	}
}
