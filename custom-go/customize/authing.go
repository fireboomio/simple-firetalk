package customize

import (
	"custom-go/generated"
	"custom-go/pkg/base"
	"custom-go/pkg/consts"
	"custom-go/pkg/plugins"
	"custom-go/pkg/utils"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"os"
	"strings"

	"github.com/Authing/authing-go-sdk/lib/authentication"
	"github.com/Authing/authing-go-sdk/lib/model"
	authV3 "github.com/Authing/authing-golang-sdk/v3/authentication"
	"github.com/Authing/authing-golang-sdk/v3/dto"
	"github.com/graphql-go/graphql"
	"github.com/joho/godotenv"
)

type (
	loginParams struct {
		Phone              string `json:"phone"`
		Code               string `json:"code"`
		LearningLanguageId string `json:"learningLanguageId"`
		LanguageDifficulty string `json:"languageDifficulty"`
		Nickname           string `json:"nickname"`
		// Age                string `json:"age"`
		// Profession         string `json:"profession"`
		// LearningPurpose    string `json:"learningPurpose"`
	}

	stdJwtBody struct {
		Sub string `json:"sub"`
		// 其它的忽略
	}
)

var (
	authV2Client *authentication.Client
	authV3Client *authV3.AuthenticationClient

	authingRootQuery = graphql.ObjectConfig{Name: "RootQuery", Fields: graphql.Fields{
		"_empty": &graphql.Field{
			Type: graphql.String,
			Resolve: func(params graphql.ResolveParams) (interface{}, error) {
				return nil, nil
			},
		},
	}}

	authingLoginOrRegister = graphql.ObjectConfig{Name: "RootMutation", Fields: graphql.Fields{
		"loginOrRegister": &graphql.Field{
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "LoginResponse",
				Fields: graphql.Fields{
					"access_token": &graphql.Field{
						Type: graphql.String,
					},
					"id_token": &graphql.Field{
						Type: graphql.String,
					},
					"refresh_token": &graphql.Field{
						Type: graphql.String,
					},
					"token_type": &graphql.Field{
						Type: graphql.String,
					},
					"expire_in": &graphql.Field{
						Type: graphql.String,
					},
				},
			}),
			Args: graphql.FieldConfigArgument{
				"phone": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"code": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"learningLanguageId": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"languageDifficulty": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"nickname": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"age": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"profession": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"learningPurpose": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
			},
			Resolve: func(params graphql.ResolveParams) (interface{}, error) {
				grc, args, err := plugins.ResolveArgs[loginParams](params)
				if err != nil {
					return nil, err
				}

				randomPwd := utils.RandStr(12)
				defaultGender := "male"
				existed, err := authV2Client.IsUserExists(&model.IsUserExistsRequest{
					Phone: &args.Phone,
				})
				if err != nil {
					return nil, err
				}
				signInOptions := dto.SignInOptionsDto{
					Scope: "profile openid username phone offline_access",
				}
				var loginResp *dto.LoginTokenRespDto
				if !*existed {
					_, err := authV2Client.RegisterByPhoneCode(&model.RegisterByPhoneCodeInput{
						Phone:    args.Phone,
						Code:     args.Code,
						Password: &randomPwd,
						Profile: &model.RegisterProfile{
							Username:          &args.Phone,
							Nickname:          &args.Phone,
							PreferredUsername: &args.Phone,
							Gender:            &defaultGender,
						},
					})
					if err != nil {
						return nil, err
					}
					loginResp = authV3Client.SignInByPhonePassword(args.Phone, randomPwd, signInOptions)
				} else {
					loginResp = authV3Client.SignInByPhonePassCode(args.Phone, args.Code, "+86", signInOptions)
				}
				if loginResp.StatusCode < 300 && loginResp.StatusCode >= 200 {
					// 同步
					err = syncUser(grc.InternalClient, args, loginResp.Data.IdToken)
					if err != nil {
						grc.Logger.Errorf("用户同步失败: %s", err.Error())
						return nil, errors.New("用户同步失败")
					}
					grc.Logger.Infof("用户%s已同步", args.Phone)
					return loginResp.Data, nil
				}
				return nil, errors.New(loginResp.Message)
			},
		},
	}}

	Authing_schema, _ = graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(authingRootQuery), Mutation: graphql.NewObject(authingLoginOrRegister)})
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	authingClientId := os.Getenv(consts.ENV_AUTHING_CLIENT_ID)
	authingClientSecret := os.Getenv(consts.ENV_AUTHING_CLIENT_SECRET)
	authV2Client = authentication.NewClient(authingClientId, authingClientSecret)
	authV2Client.UserPoolId = os.Getenv(consts.ENV_AUTHING_USER_POOL_ID)
	authV3Client, err = authV3.NewAuthenticationClient(&authV3.AuthenticationClientOptions{
		AppId:       authingClientId,
		AppSecret:   authingClientSecret,
		AppHost:     os.Getenv(consts.ENV_AUTHING_APP_HOST),
		RedirectUri: os.Getenv(consts.ENV_AUTHING_REDIRECT_URI),
	})
	if err != nil {
		// log.Panic(err)
	}
}

func createUser(internalClient *base.InternalClient, user *loginParams, jwtToken string) (string, error) {
	bytes, err := base64.StdEncoding.DecodeString(strings.Split(jwtToken, ".")[1])
	if err != nil {
		return "", err
	}

	var jwtBody stdJwtBody
	err = json.Unmarshal(bytes, &jwtBody)
	if err != nil {
		return "", err
	}

	userId := jwtBody.Sub
	// 查询默认的难度
	difficultyResp, err := plugins.ExecuteInternalRequestQueries[any, generated.Dict__GetDictDefaultValueResponseData](internalClient, generated.Dict__GetDictDefaultValue, generated.Dict__GetDictDefaultValueInput{
		Code: "lng_difficulty",
	})
	if err != nil {
		return "", err
	}

	// 查询默认的语速
	speedResp, err := plugins.ExecuteInternalRequestQueries[any, generated.Dict__GetDictDefaultValueResponseData](internalClient, generated.Dict__GetDictDefaultValue, generated.Dict__GetDictDefaultValueInput{
		Code: "speech_speed",
	})
	if err != nil {
		return "", err
	}

	// 获取默认学习语言
	languages, err := plugins.ExecuteInternalRequestQueries[any, generated.Language__GetLearningLanguagesResponseData](internalClient, generated.Language__GetLearningLanguages, nil)
	if err != nil {
		return "", err
	}

	// 同步用户
	_, err = plugins.ExecuteInternalRequestMutations[generated.User__CreateOneUserInput, any](internalClient, generated.User__CreateOneUser, generated.User__CreateOneUserInput{
		Id:     userId,
		Avatar: "https://freetalk-common.oss-cn-shanghai.aliyuncs.com/avatar/freetalk_default_avatar.png",
		// Nickname:           utils.GetStringValueWithDefault(user.Nickname, user.Phone),
		// 默认用户名 Lily
		Nickname:           utils.GetStringValueWithDefault(user.Nickname, "Lily"),
		Phone:              user.Phone,
		LearningLanguageId: utils.GetStringValueWithDefault(user.LearningLanguageId, languages.Data[0].Id),
		Difficulty:         utils.GetStringValueWithDefault(user.LanguageDifficulty, difficultyResp.Data.Value),
		SpeedOfSpeech:      speedResp.Data.Value,
		// Age:    user.Age,
		// LearningPurpose:    user.LearningPurpose,
		// Profession:         user.Profession,
	})
	return userId, err
}

func syncUser(internalClient *base.InternalClient, user *loginParams, jwtToken string) error {
	// 判断用户是否已同步
	existedUser, err := plugins.ExecuteInternalRequestQueries[generated.User__IsUserExistedInput, generated.User__IsUserExistedResponseData](internalClient, generated.User__IsUserExisted, generated.User__IsUserExistedInput{
		Phone: user.Phone,
	})
	if err != nil {
		return err
	}

	userId := existedUser.Data.Id
	if userId == "" {
		userId, err = createUser(internalClient, user, jwtToken)
		if err != nil {
			return err
		}
	}

	// 判断账户是否已同步
	if existedUser.Data.AccountId == "" {
		// 同步账户
		_, err = plugins.ExecuteInternalRequestMutations[generated.Account__CreateOneAccountInput, any](internalClient, generated.Account__CreateOneAccount, generated.Account__CreateOneAccountInput{
			UserId: userId,
		})
		if err != nil {
			// 删除用户
			_, _ = plugins.ExecuteInternalRequestMutations[generated.User__DeleteOneUserInput, any](internalClient, generated.User__DeleteOneUser, generated.User__DeleteOneUserInput{
				Id: userId,
			})
			return err
		}
	}
	return nil
}
