package GetCostDurationHistoryGroup

import (
	"custom-go/generated"
	"custom-go/pkg/base"
	"custom-go/pkg/utils"
	"time"
)

type costDurationHistoryRDD = generated.Duration__GetCostDurationHistoryResponseData_data

func MutatingPostResolve(hook *base.HookRequest, body generated.Duration__GetCostDurationHistoryBody) (res generated.Duration__GetCostDurationHistoryBody, err error) {
	if body.Response == nil {
		return body, nil
	}

	durationGroup := make(map[string]*costDurationHistoryRDD, 0)
	for _, item := range body.Response.Data.Data {
		day, _ := time.Parse(utils.ISO8601Layout, item.CreatedAt)
		dayStr := day.Format("2006-01-02")
		if existData, ok := durationGroup[dayStr]; ok {
			existData.Value = existData.Value + item.Value
			continue
		}

		durationGroup[dayStr] = &costDurationHistoryRDD{
			Value:     item.Value,
			CreatedAt: dayStr,
		}
	}

	var respData []costDurationHistoryRDD
	for _, value := range durationGroup {
		respData = append(respData, *value)
	}
	body.Response.Data.Data = respData
	return body, nil
}
