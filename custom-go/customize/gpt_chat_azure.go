package customize

import (
	"custom-go/hooks/Chat/Message/CreateOneChatMessage"
	"custom-go/pkg/base"
	"custom-go/pkg/plugins"
	"github.com/labstack/echo/v4"
)

const (
	azure35           gptModelType = "azure3.5"
	azure35Url                     = "${azure35Url}"
	azure35Model                   = "gpt-35-turbo"
	azure35ApiKeyName              = "api-key"
	azure35ApiKeyCode              = "azure35_apikey"
)

var azure35Headers = map[string]string{
	azure35ApiKeyName: "${azure35ApiKey}",
}

func init() {
	base.AddRegisteredHook(func(_ echo.Logger) {
		if apikey, _ := CreateOneChatMessage.GetDictISetting(plugins.DefaultInternalClient, azure35ApiKeyCode); apikey != "" {
			azure35Headers[azure35ApiKeyName] = apikey
		}
	})

	modelActionMap[azure35] = &gptModelSetting{
		gptModel: azure35Model,
		gptURl:   azure35Url,
		headers:  azure35Headers,
	}
}
