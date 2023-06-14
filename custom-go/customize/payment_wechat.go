package customize

/**
暂未调试通过
*/
import (
	"context"
	"crypto/rsa"
	"custom-go/pkg/base"
	"custom-go/pkg/plugins"
	"custom-go/pkg/utils"
	"fmt"
	wxCore "github.com/wechatpay-apiv3/wechatpay-go/core"
	wxVerifiers "github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	wxDecryptors "github.com/wechatpay-apiv3/wechatpay-go/core/cipher/decryptors"
	wxEncryptors "github.com/wechatpay-apiv3/wechatpay-go/core/cipher/encryptors"
	wxDownloader "github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
	wxNotify "github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	wxOption "github.com/wechatpay-apiv3/wechatpay-go/core/option"
	wxPayments "github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	wxApp "github.com/wechatpay-apiv3/wechatpay-go/services/payments/app"
	wxUtils "github.com/wechatpay-apiv3/wechatpay-go/utils"
	"io"
	"net/http"
)

const (
	appName                      string = "free-talk"                                // 应用名称
	wxAppid                      string = "wxd678efh567hg6787"                       // 应用ID
	wxMchID                      string = "190000****"                               // 商户号
	wxMchCertificateSerialNumber string = "3775B6A45ACD588826D15E583A95F5DD********" // 商户证书序列号
	wxMchAPIv3Key                string = "2ab9****************************"         // 商户APIv3密钥
	wxApiclientKeyFilepath       string = "doc/apiclient_key.pem"                    // 商户私钥存放路径
)

var (
	wxMchPrivateKey *rsa.PrivateKey
	wxNotifyHandler *wxNotify.Handler
)

const (
	WxPay PayType = "wxPay"
)

func init() {
	initWxConfig()

	PayMap[WxPay] = &payAction{
		buildClient: func(ctx context.Context) (client any, err error) {
			// 使用商户私钥等初始化 client，并使它具有自动定时获取微信支付平台证书的能力
			opts := []wxCore.ClientOption{
				wxOption.WithWechatPayAutoAuthCipher(wxMchID, wxMchCertificateSerialNumber, wxMchPrivateKey, wxMchAPIv3Key),
				wxOption.WithWechatPayCipher(
					wxEncryptors.NewWechatPayEncryptor(wxDownloader.MgrInstance().GetCertificateVisitor(wxMchID)),
					wxDecryptors.NewWechatPayDecryptor(wxMchPrivateKey),
				),
			}
			client, err = wxCore.NewClient(ctx, opts...)
			return
		},
		prepayForApp: func(client any, grc *base.GraphqlRequestContext, totalFee int64, notifyURI, outTradeNo, productName string, expireAt string) (resp any, err error) {
			wxClient := client.(*wxCore.Client)
			svc := wxApp.AppApiService{Client: wxClient}
			// 得到prepay_id，以及调起支付所需的参数和签名
			resp, result, err := svc.PrepayWithRequestPayment(grc.Context,
				wxApp.PrepayRequest{
					Mchid:       wxCore.String(wxMchID),
					OutTradeNo:  wxCore.String(outTradeNo),
					Appid:       wxCore.String(wxAppid),
					Description: wxCore.String(utils.JoinString("-", appName, productName)),
					NotifyUrl:   wxCore.String(notifyURI),
					Amount: &wxApp.Amount{
						Total: wxCore.Int64(totalFee),
					},
				},
			)
			if err != nil {
				return
			}

			err = handleWxApiResult(result)
			return
		},
		PayNotify: func(request *http.Request, _ string) (paymentUpdateInput PaymentUpdateI, err error) {
			transaction := new(wxPayments.Transaction)
			_, err = wxNotifyHandler.ParseNotifyRequest(context.Background(), request, transaction)
			if err != nil {
				return
			}

			mappedStatus, ok := StatusMapping[*transaction.TradeState]
			if !ok {
				err = fmt.Errorf("【微信】未知的交易状态: %s", *transaction.TradeState)
				return
			}

			paymentUpdateInput = PaymentUpdateI{
				OrderNumber:   *transaction.OutTradeNo,
				PaymentDate:   *transaction.SuccessTime,
				PaymentStatus: mappedStatus,
			}
			return
		},
		statusQuery: func(client any, ctx context.Context, outTradeNo string) (res any, err error) {
			wxClient := client.(*wxCore.Client)
			svc := wxApp.AppApiService{Client: wxClient}

			params := wxApp.QueryOrderByOutTradeNoRequest{
				OutTradeNo: wxCore.String(outTradeNo),
				Mchid:      wxCore.String(wxMchID),
			}

			resp, result, err := svc.QueryOrderByOutTradeNo(ctx, params)
			if err != nil {
				return
			}

			err = handleWxApiResult(result)
			res = *resp
			return
		},
		ModifyRequestForPayNotify: func(body *plugins.HttpTransportBody) (*base.ClientRequest, error) {
			return body.Request, nil
		},
	}
}

func initWxConfig() {
	// 使用 utils 提供的函数从本地文件中加载商户私钥，商户私钥会用来生成请求的签名
	wxMchPrivateKey, _ = wxUtils.LoadPrivateKeyWithPath(wxApiclientKeyFilepath)

	ctx := context.Background()
	// 1. 使用 `RegisterDownloaderWithPrivateKey` 注册下载器
	_ = wxDownloader.MgrInstance().RegisterDownloaderWithPrivateKey(ctx, wxMchPrivateKey, wxMchCertificateSerialNumber, wxMchID, wxMchAPIv3Key)
	// 2. 获取商户号对应的微信支付平台证书访问器
	certificateVisitor := wxDownloader.MgrInstance().GetCertificateVisitor(wxMchID)
	// 3. 使用证书访问器初始化 `notify.Handler`
	wxNotifyHandler, _ = wxNotify.NewRSANotifyHandler(wxMchAPIv3Key, wxVerifiers.NewSHA256WithRSAVerifier(certificateVisitor))
}

func handleWxApiResult(result *wxCore.APIResult) (err error) {
	if result == nil {
		return
	}

	if result.Response.StatusCode != http.StatusOK {
		respBodyBytes, _ := io.ReadAll(result.Response.Body)
		err = fmt.Errorf("[%s] response error cause by [%s]", result.Request.RequestURI, string(respBodyBytes))
		return
	}
	return
}
