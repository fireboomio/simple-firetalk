package customize

import (
	"context"
	"custom-go/generated"
	"custom-go/pkg/base"
	"custom-go/pkg/plugins"
	"custom-go/pkg/utils"
	"encoding/json"
	"github.com/labstack/echo/v4"
	"github.com/smartwalle/alipay/v3"
	"time"
)

type (
	getPendingPaymentsI  = any
	getPendingPaymentsRD = generated.Payment__GetPendingPaymentsResponseData
	cancelOnePaymentI    = generated.Payment__CancelOnePaymentInternalInput
	cancelOnePaymentRD   = generated.Payment__CancelOnePaymentResponseData
	paymentConfI         = generated.Payment__GetPaymentConfInternalInput
	paymentConfRD        = generated.Payment__GetPaymentConfResponseData
)

var (
	getPendingPaymentsQueryPath  = generated.Payment__GetPendingPayments
	cancelOnePaymentMutationPath = generated.Payment__CancelOnePayment
	paymentConfQueryPath         = generated.Payment__GetPaymentConf
)

var (
	StatusMapping = map[string]generated.Freetalk_PaymentStatus{
		"TRADE_FINISHED": generated.Freetalk_PaymentStatus_PAID,
		"TRADE_SUCCESS":  generated.Freetalk_PaymentStatus_PAID,
		"TRADE_CLOSED":   generated.Freetalk_PaymentStatus_CANCELLED,
		"WAIT_BUYER_PAY": generated.Freetalk_PaymentStatus_PENDING,
	}
)

func init() {
	base.AddRegisteredHook(startPaymentCron)
}

func startPaymentCron(logger echo.Logger) {
	internalClient := plugins.DefaultInternalClient

	confRes, err := getPaymentConf(internalClient)
	if err != nil {
		logger.Errorf("获取支付配置失败：%v", err)
		return
	}
	for range time.Tick(time.Duration(confRes.Data.CronIntervalSec) * time.Second) {
		paymentsRD, err := getPendingPayments(internalClient)
		if err != nil {
			logger.Errorf("获取待支付订单失败：%v", err)
			continue
		}

		for _, data := range paymentsRD.Data {
			createAt, err := time.Parse(utils.ISO8601Layout, data.CreatedAt)
			if err != nil {
				logger.Errorf("解析订单'%s'的创建时间'%s'失败：%v", data.OrderNumber, data.CreatedAt, err)
				continue
			}
			expireAt, err := time.Parse(utils.ISO8601Layout, data.ExpireAt)
			if err != nil {
				logger.Errorf("解析订单'%s'的过期时间'%s'失败：%v", data.OrderNumber, data.ExpireAt, err)
				continue
			}

			// 创建时间距离现在不足2分钟，暂不查询
			if time.Since(createAt) < time.Duration(confRes.Data.StartQueryMin)*time.Minute {
				logger.Infof("订单未到查询时间: %s", data.OrderNumber)
				continue
			}

			// 订单过期，取消订单
			if time.Since(expireAt) > 0 {
				_, err := cancelOnePayment(internalClient, data.Id)
				if err != nil {
					logger.Errorf("取消订单'%s'失败：%v", data.OrderNumber, err)
				}
				continue
			}

			logger.Infof("开始查询订单: %s", data.OrderNumber)
			// 查询订单状态
			pay, ok := PayMap[AliPay]
			if !ok {
				logger.Errorf("不支持的支付类型：%s", AliPay)
				continue
			}

			payClient, err := pay.buildClient(context.Background())
			if err != nil {
				logger.Errorf("创建支付客户端失败，订单'%s'：%v", data.OrderNumber, err)
				continue
			}

			result, err := pay.statusQuery(payClient, context.Background(), data.OrderNumber)
			if err != nil {
				logger.Errorf("查询订单'%s'的状态失败：%v", data.OrderNumber, err)
				continue
			}

			resp := result.(*alipay.TradeQueryRsp)
			logger.Infof("【支付宝】查询到订单'%s'的状态：%v", data.OrderNumber, resp.Content.TradeStatus)

			// 更新订单状态
			orderStatus, ok := StatusMapping[string(resp.Content.TradeStatus)]
			if !ok {
				continue
			}

			contentBytes, err := json.Marshal(resp.Content)
			if err != nil {
				logger.Errorf("JSON编码失败：%s", err)
				continue
			}
			paymentResp := string(contentBytes)

			paymentUpdateInput := PaymentUpdateI{
				OrderNumber:   data.OrderNumber,
				PaymentDate:   utils.CurrentDateTime(),
				PaymentStatus: orderStatus,
				PaymentResp:   paymentResp,
			}
			updateResp, err := UpdateOnePayment(internalClient, paymentUpdateInput)
			if err != nil {
				logger.Errorf("更新订单'%s'的状态失败：%v", data.OrderNumber, err)
				continue
			}

			orderCalculate, ok := OrderCalculateMap[UnifiedOrderProduct(data.Usage)]
			if !ok {
				logger.Errorf("不支持的产品：%s", data.Usage)
				continue
			}

			duration := orderCalculate.FetchPaymentDuration(updateResp)
			err = RecordAndUpdateDuration(internalClient, duration, data.AccountId, data.Id)
			if err != nil {
				logger.Errorf("记录和更新订单'%s'的时长失败：%v", data.OrderNumber, err)
			}
		}
	}
}

// getPendingPayments 获取状态为PENDING的订单
func getPendingPayments(internalClient *base.InternalClient) (data getPendingPaymentsRD, err error) {
	data, err = plugins.ExecuteInternalRequestQueries[getPendingPaymentsI, getPendingPaymentsRD](internalClient, getPendingPaymentsQueryPath, nil)
	return
}

// getPaymentConf 获取支付配置
func getPaymentConf(internalClient *base.InternalClient) (confRes paymentConfRD, err error) {
	confRes, err = plugins.ExecuteInternalRequestQueries[paymentConfI, paymentConfRD](internalClient, paymentConfQueryPath, paymentConfI{})
	return
}

// UpdateOnePayment 完成订单
func UpdateOnePayment(internalClient *base.InternalClient, paymentUpdateInput PaymentUpdateI) (PaymentUpdateRD, error) {
	if paymentUpdateInput.PaymentStatus == "PAID" {
		return plugins.ExecuteInternalRequestMutations[PaymentUpdateI, PaymentUpdateRD](internalClient, paymentUpdatePath, paymentUpdateInput)
	}
	return PaymentUpdateRD{}, nil
}

// cancelOnePayment 取消订单
func cancelOnePayment(internalClient *base.InternalClient, id string) (cancelOnePaymentRD, error) {
	cancelOnePaymentInput := cancelOnePaymentI{Id: id}
	return plugins.ExecuteInternalRequestMutations[cancelOnePaymentI, cancelOnePaymentRD](internalClient, cancelOnePaymentMutationPath, cancelOnePaymentInput)
}
