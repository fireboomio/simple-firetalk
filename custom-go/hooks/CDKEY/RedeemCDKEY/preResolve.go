package RedeemCDKEY

import (
	"custom-go/generated"
	"custom-go/pkg/base"
	"custom-go/pkg/plugins"
	"errors"
)

type (
	CDKEYQueryI  = generated.CDKEY__GetCDKKEYByCodeInput
	CDKEYQueryRD = generated.CDKEY__GetCDKKEYByCodeResponseData
)

func PreResolve(hook *base.HookRequest, body generated.CDKEY__RedeemCDKEYBody) (res generated.CDKEY__RedeemCDKEYBody, err error) {
	CDKEYQueryInput := CDKEYQueryI{Code: body.Input.Code}
	CDKEYQueryResp, err := plugins.ExecuteInternalRequestQueries[CDKEYQueryI, CDKEYQueryRD](hook.InternalClient, generated.CDKEY__GetCDKKEYByCode, CDKEYQueryInput)
	if err != nil {
		return
	}

	if CDKEYQueryResp.Data.Id == "" {
		err = errors.New("cdkey无效")
		return
	}

	if CDKEYQueryResp.Data.Redeemed {
		err = errors.New("cdkey已被兑换")
		return
	}

	return body, nil
}
