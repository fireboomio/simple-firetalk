package payNotify

import (
	"custom-go/customize"
	"custom-go/pkg/base"
	"custom-go/pkg/plugins"
	"fmt"
	"net/http"
)

var (
	payType = customize.AliPay
)

func init() {
	plugins.AddProxyHook(payNotify)
}

func payNotify(hook *base.HttpTransportHookRequest, body *plugins.HttpTransportBody) (*base.ClientResponse, error) {
	if err := verify(body, hook); err != nil {
		return body.Response, fmt.Errorf("【支付宝】【回调】verification failed: %w", err)
	}

	response, err := modifyResponse(body)
	if err != nil {
		return body.Response, fmt.Errorf("【支付宝】【回调】modify response failed: %w", err)
	}
	return response, nil
}

// verify 1.验证签名 2.修改支付状态
func verify(body *plugins.HttpTransportBody, hook *base.HttpTransportHookRequest) error {
	pay, ok := customize.PayMap[payType]
	if !ok {
		return fmt.Errorf("【支付宝】【回调】unable to find payType %v in PayMap", payType)
	}

	updateOnePaymentInput, err := pay.PayNotify(hook.Request(), string(body.Request.OriginBody))
	if err != nil {
		return fmt.Errorf("【支付宝】【回调】PayNotify failed: %w", err)
	}

	if _, err := handlePayment(updateOnePaymentInput, hook.InternalClient); err != nil {
		return fmt.Errorf("【支付宝】【回调】HandlePayment failed: %w", err)
	}

	return nil
}

// modifyResponse 修改返回体
func modifyResponse(body *plugins.HttpTransportBody) (*base.ClientResponse, error) {
	body.Response = &base.ClientResponse{StatusCode: http.StatusOK}
	body.Response.OriginBody = []byte("SUCCESS")
	return body.Response, nil
}

// handlePayment 处理支付状态
func handlePayment(updateOnePaymentInput customize.PaymentUpdateI, c *base.InternalClient) (result interface{}, err error) {
	payment, err := customize.UpdateOnePayment(c, updateOnePaymentInput)
	if err != nil {
		return
	}

	paymentData := payment.Data
	orderCalculate, ok := customize.OrderCalculateMap[customize.UnifiedOrderProduct(paymentData.Usage)]
	if !ok {
		err = fmt.Errorf("【支付宝】【回调】not support updatePayment for product [%s]", paymentData.Usage)
		return
	}

	increaseDuration := orderCalculate.FetchPaymentDuration(payment)
	err = customize.RecordAndUpdateDuration(c, increaseDuration, paymentData.AccountId, paymentData.Id)
	return
}
