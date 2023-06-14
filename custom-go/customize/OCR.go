package customize

import (
	"custom-go/pkg/plugins"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/graphql-go/graphql"
	"github.com/tidwall/gjson"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	OCRApiKey      = "${OCRApiKey}"
	OCRSecretKey   = "${OCRSecretKey}"
	OCRTokenUrl    = "https://aip.baidubce.com/oauth/2.0/token?grant_type=client_credentials&client_id=%s&client_secret=%s"
	OCRIdentifyUrl = "https://aip.baidubce.com/rest/2.0/ocr/v1/accurate_basic?access_token=%s"
)

type (
	OCRIdentifyConfig struct {
		Index int    `json:"index"`
		Text  string `json:"text"`
	}
	OCRIdentifyBody struct {
		Url string `json:"url"`
	}
	OCRIdentifyResult struct {
		LogId          uint64                  `json:"log_id"`
		Direction      int32                   `json:"direction"`
		WordsResultNum uint32                  `json:"words_result_num"`
		WordsResult    []OCRIdentifyResultWord `json:"words_result"`
		ErrorMsg       string                  `json:"error_msg,omitempty"`
	}
	OCRIdentifyResultWord struct {
		Words string `json:"words"`
	}
)

var OCR_schema, _ = graphql.NewSchema(graphql.SchemaConfig{
	Query: graphql.NewObject(graphql.ObjectConfig{Name: "query", Fields: graphql.Fields{
		"identify": {
			Type: graphql.NewList(graphql.String),
			Args: graphql.FieldConfigArgument{
				"url": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
			},
			Resolve: func(params graphql.ResolveParams) (result interface{}, err error) {
				_, args, err := plugins.ResolveArgs[OCRIdentifyBody](params)
				if err != nil {
					return
				}

				tokenBytes, err := commonRequest(fmt.Sprintf(OCRTokenUrl, OCRApiKey, OCRSecretKey), "", "application/json")
				if err != nil {
					return
				}

				identifyPayload := fmt.Sprintf("url=%s&language_type=auto_detect&detect_direction=true", url.QueryEscape(args.Url))
				accessToken := gjson.Get(string(tokenBytes), "access_token").String()
				identifyBytes, err := commonRequest(fmt.Sprintf(OCRIdentifyUrl, accessToken), identifyPayload, "application/x-www-form-urlencoded")
				if err != nil {
					return
				}

				var ORCResult OCRIdentifyResult
				err = json.Unmarshal(identifyBytes, &ORCResult)
				if err != nil {
					return
				}

				if errorMsg := ORCResult.ErrorMsg; errorMsg != "" {
					err = errors.New(errorMsg)
					return
				}

				var words []string
				for _, item := range ORCResult.WordsResult {
					words = append(words, item.Words)
				}

				result = words
				return
			},
		},
	}}),
})

func commonRequest(url string, payload string, contentType string) (result []byte, err error) {
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(payload))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		err = errors.New(string(bodyBytes))
		return
	}

	result, err = io.ReadAll(resp.Body)
	return
}
