package customize

import (
	"context"
	"custom-go/generated"
	"custom-go/pkg/base"
	"custom-go/pkg/plugins"
	"custom-go/pkg/types"
	"custom-go/pkg/utils"
	"fmt"
	"github.com/graphql-go/graphql"
	"net/http"
	"time"
)

type (
	payAction struct {
		buildClient               func(context.Context) (any, error)
		prepayForApp              func(any, *base.GraphqlRequestContext, int64, string, string, string, string) (any, error)
		PayNotify                 func(*http.Request, string) (PaymentUpdateI, error)
		statusQuery               func(any, context.Context, string) (any, error)
		ModifyRequestForPayNotify func(body *plugins.HttpTransportBody) (*base.ClientRequest, error)
	}
	PayType      string
	payActionMap map[PayType]*payAction
)

type (
	unifiedOrderInput struct {
		AccountId string `json:"accountId"`
		Product   string `json:"product"`
		ProductId string `json:"productId"`
		PayType   string `json:"payType"`
	}
	unifiedOrderCalculate struct {
		prepare              func(*base.GraphqlRequestContext, *unifiedOrderInput) (float64, float64, error)
		name                 string
		createPayment        func(*base.GraphqlRequestContext, *unifiedOrderInput, float64, string, string, string) (string, error)
		FetchPaymentDuration func(PaymentUpdateRD) float64
	}
	UnifiedOrderProduct      string
	unifiedOrderCalculateMap map[UnifiedOrderProduct]*unifiedOrderCalculate
)

type (
	statusQueryInput struct {
		PayType    string `json:"payType"`
		OutTradeNo string `json:"outTradeNo"`
	}
)

type (
	membershipI                    = generated.Membership__GetOneMembershipInput
	membershipRD                   = generated.Membership__GetOneMembershipResponseData
	durationPackageI               = generated.Duration__GetOneDurationPackageInput
	durationPackageRD              = generated.Duration__GetOneDurationPackageResponseData
	durationHistoryCreateI         = generated.Duration__CreatePaymentDurationHistoryInternalInput
	durationHistoryCreateRD        = generated.Duration__CreatePaymentDurationHistoryResponseData
	paymentDurationPackageCreateI  = generated.Payment__CreatePaymentDurationPackageInternalInput
	paymentDurationPackageCreateRD = generated.Payment__CreatePaymentDurationPackageResponseData
	paymentMembershipCreateI       = generated.Payment__CreatePaymentMembershipInternalInput
	paymentMembershipCreateRD      = generated.Payment__CreatePaymentMembershipResponseData
	PaymentUpdateI                 = generated.Payment__UpdateOnePaymentInput
	PaymentUpdateRD                = generated.Payment__UpdateOnePaymentResponseData
	accountDurationFetchI          = generated.Account__FetchAccountDurationInput
	accountDurationFetchRD         = generated.Account__FetchAccountDurationResponseData
	getOneAccountI                 = generated.Account__GetOneAccountInternalInput
	getOneAccountRD                = generated.Account__GetOneAccountResponseData
)

var (
	membershipQueryPath              = generated.Membership__GetOneMembership
	durationPackageQueryPath         = generated.Duration__GetOneDurationPackage
	durationHistoryCreatePath        = generated.Duration__CreatePaymentDurationHistory
	paymentDurationPackageCreatePath = generated.Payment__CreatePaymentDurationPackage
	paymentMembershipCreatePath      = generated.Payment__CreatePaymentMembership
	accountDurationFetchPath         = generated.Account__FetchAccountDuration
	getOneAccountPath                = generated.Account__GetOneAccount
	paymentUpdatePath                = generated.Payment__UpdateOnePayment
)

var (
	publicNodeUrl = utils.GetConfigurationVal(types.WdgGraphConfig.Api.NodeOptions.PublicNodeUrl)
	payNotifyURI  = "/proxy/payNotify/"
)

func RecordAndUpdateDuration(c *base.InternalClient, increaseDuration float64, accountId, paymentId string) (err error) {
	durationHistoryCreateInput := durationHistoryCreateI{
		AccountId: accountId,
		PaymentId: paymentId,
		Value:     increaseDuration,
	}
	// 添加时长获取记录
	_, err = plugins.ExecuteInternalRequestMutations[durationHistoryCreateI, durationHistoryCreateRD](c, durationHistoryCreatePath, durationHistoryCreateInput)
	if err != nil {
		return
	}

	// 更新剩余时长
	accountDurationFetchInput := accountDurationFetchI{Duration: increaseDuration, Id: accountId}
	_, err = plugins.ExecuteInternalRequestMutations[accountDurationFetchI, accountDurationFetchRD](c, accountDurationFetchPath, accountDurationFetchInput)
	return
}

var Payment_schema, _ = graphql.NewSchema(graphql.SchemaConfig{
	Query: graphql.NewObject(graphql.ObjectConfig{Name: "query", Fields: graphql.Fields{
		"statusQuery": {
			Type: graphql.String,
			Args: graphql.FieldConfigArgument{
				"payType": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"outTradeNo": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
			},
			Resolve: func(params graphql.ResolveParams) (result interface{}, err error) {
				grc, args, err := plugins.ResolveArgs[statusQueryInput](params)
				if err != nil {
					return
				}

				pay, ok := PayMap[PayType(args.PayType)]
				if !ok {
					err = fmt.Errorf("not support statusQuery payType [%s]", args.PayType)
					return
				}

				payClient, err := pay.buildClient(grc.Context)
				if err != nil {
					return
				}

				result, err = pay.statusQuery(payClient, grc.Context, args.OutTradeNo)
				return
			},
		},
	}}),
	Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "mutation", Fields: graphql.Fields{
		"unifiedOrder": {
			Type: graphql.String,
			Args: graphql.FieldConfigArgument{
				"accountId": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"product": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"productId": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
				"payType": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
			},
			Resolve: func(params graphql.ResolveParams) (result interface{}, err error) {
				grc, args, err := plugins.ResolveArgs[unifiedOrderInput](params)
				if err != nil {
					return
				}

				pay, ok := PayMap[PayType(args.PayType)]
				if !ok {
					err = fmt.Errorf("not support unifiedOrder payType [%s]", args.PayType)
					return
				}
				payClient, err := pay.buildClient(grc.Context)
				if err != nil {
					return
				}

				orderCalculate, ok := OrderCalculateMap[UnifiedOrderProduct(args.Product)]
				if !ok {
					err = fmt.Errorf("not support unifiedOrder product [%s]", args.Product)
					return
				}

				confRes, err := plugins.ExecuteInternalRequestQueries[paymentConfI, paymentConfRD](grc.InternalClient, paymentConfQueryPath, paymentConfI{})
				if err != nil {
					return
				}

				// 获取时常包或会员的价格
				price, attach, err := orderCalculate.prepare(grc, args)
				if err != nil {
					return
				}
				orderAmount := price * 100
				outTradeNo := utils.GenOrderNumber()
				totalFee := int64(orderAmount)
				expireAt := time.Now().Add(time.Duration(confRes.Data.ExpireMin) * time.Minute).Format(utils.ISO8601Layout)

				notifyURI := fmt.Sprintf("%s%s%s", publicNodeUrl, payNotifyURI, args.PayType)
				result, err = pay.prepayForApp(payClient, grc, totalFee, notifyURI, outTradeNo, orderCalculate.name, expireAt)

				// 创建订单标记为待支付
				paymentId, err := orderCalculate.createPayment(grc, args, orderAmount, outTradeNo, result.(string), expireAt)
				if err != nil {
					return nil, err
				}

				if totalFee == 0 {
					err = RecordAndUpdateDuration(grc.InternalClient, attach, args.AccountId, paymentId)
					return nil, err
				}
				return
			},
		},
	}}),
})

var (
	PayMap            payActionMap
	OrderCalculateMap unifiedOrderCalculateMap
)

const (
	durationPackage UnifiedOrderProduct = "DurationPackage"
	membership      UnifiedOrderProduct = "Membership"
)

func init() {
	PayMap = make(payActionMap, 0)

	calculateDiscountFunc := func(price float64, expireAt string, discount float64) float64 {
		if utils.CurrentDateTime() > expireAt {
			return price
		}

		return price - discount
	}

	calculateMembershipPrice := func(grc *base.GraphqlRequestContext, price float64, accountRes getOneAccountRD, membershipRes membershipRD) float64 {
		// 如果会员已经过期，返回 要买的会员价格
		if utils.CurrentDateTime() > accountRes.Data.MembershipEndTime {
			grc.Logger.Info("【新办会员】返回原价: ", price)
			return price
		}
		// 如果会员未过期
		// 1.如果要买的会员时长 > 当前会员时长，返回 要买的会员价格-当前会员价格
		if accountRes.Data.Membership.Lifespan < membershipRes.Data.Lifespan {
			price = price - accountRes.Data.Membership.Price
			grc.Logger.Info("【升级会员】返回差价: ", price)
			return price
		}
		// 2.如果要买的会员时长 <= 当前会员时长，返回 要买的会员价格
		if accountRes.Data.Membership.Lifespan <= membershipRes.Data.Lifespan {
			grc.Logger.Info("【续费会员】返回原价: ", price)
			return price
		}
		return price
	}
	OrderCalculateMap = make(unifiedOrderCalculateMap, 0)
	OrderCalculateMap[durationPackage] = &unifiedOrderCalculate{
		name: "时长包",
		prepare: func(grc *base.GraphqlRequestContext, args *unifiedOrderInput) (price, duration float64, err error) {
			durationPackageInput := durationPackageI{Id: args.ProductId}
			packageRes, err := plugins.ExecuteInternalRequestQueries[durationPackageI, durationPackageRD](grc.InternalClient, durationPackageQueryPath, durationPackageInput)
			if err != nil {
				return
			}

			data := packageRes.Data
			price = calculateDiscountFunc(data.Price, data.Discount.ExpireAt, data.Discount.Value)
			duration = data.Value
			return
		},
		createPayment: func(grc *base.GraphqlRequestContext, args *unifiedOrderInput, totalFee float64, outTradeNo string, sn string, expireAt string) (paymentId string, err error) {
			paymentDurationPackageCreateInput := paymentDurationPackageCreateI{
				AccountId:   args.AccountId,
				OrderAmount: totalFee,
				OrderNumber: outTradeNo,
				PackageId:   args.ProductId,
				PayType:     args.PayType,
				Sn:          sn,
				ExpireAt:    expireAt,
			}
			resp, err := plugins.ExecuteInternalRequestMutations[paymentDurationPackageCreateI, paymentDurationPackageCreateRD](grc.InternalClient, paymentDurationPackageCreatePath, paymentDurationPackageCreateInput)
			if err != nil {
				return
			}

			paymentId = resp.Data.Id
			return
		},
		FetchPaymentDuration: func(payment PaymentUpdateRD) float64 {
			return payment.Data.DurationValue
		},
	}
	OrderCalculateMap[membership] = &unifiedOrderCalculate{
		name: "会员",
		prepare: func(grc *base.GraphqlRequestContext, args *unifiedOrderInput) (price, duration float64, err error) {
			membershipInput := membershipI{Id: args.ProductId}
			membershipRes, err := plugins.ExecuteInternalRequestQueries[membershipI, membershipRD](grc.InternalClient, membershipQueryPath, membershipInput)
			if err != nil {
				return
			}

			data := membershipRes.Data

			// 会员购买折扣计算
			price = calculateDiscountFunc(data.Price, data.Discount.ExpireAt, data.Discount.Value)
			grc.Logger.Infof("【折扣价】%d: ", price)

			accountInput := getOneAccountI{UserId: args.AccountId}
			accountRes, err := plugins.ExecuteInternalRequestQueries[getOneAccountI, getOneAccountRD](grc.InternalClient, getOneAccountPath, accountInput)

			// 会员购买差价计算
			price = calculateMembershipPrice(grc, price, accountRes, membershipRes)
			grc.Logger.Infof("【计算差价】%d: ", price)

			duration = data.PresentDuration
			return
		},
		createPayment: func(grc *base.GraphqlRequestContext, args *unifiedOrderInput, totalFee float64, outTradeNo string, sn string, expireAt string) (paymentId string, err error) {
			paymentMembershipCreateInput := paymentMembershipCreateI{
				AccountId:    args.AccountId,
				OrderAmount:  totalFee,
				OrderNumber:  outTradeNo,
				MembershipId: args.ProductId,
				PayType:      args.PayType,
				Sn:           sn,
				ExpireAt:     expireAt,
			}
			resp, err := plugins.ExecuteInternalRequestMutations[paymentMembershipCreateI, paymentMembershipCreateRD](grc.InternalClient, paymentMembershipCreatePath, paymentMembershipCreateInput)
			if err != nil {
				return
			}

			paymentId = resp.Data.Id
			return
		},
		FetchPaymentDuration: func(payment PaymentUpdateRD) float64 {
			return payment.Data.MembershipPresent
		},
	}
}
