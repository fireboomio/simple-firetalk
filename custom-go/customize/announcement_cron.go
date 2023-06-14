package customize

import (
	"custom-go/generated"
	"custom-go/pkg/base"
	"custom-go/pkg/plugins"
	"custom-go/pkg/utils"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/robfig/cron/v3"
	"time"
)

type (
	annoCreateI             = generated.Announcement__Send__CreateOneAnnouncementInput
	annoCreateRD            = generated.Announcement__Send__CreateOneAnnouncementResponseData
	annoReceiveCreateI      = generated.Announcement__Receive__CreateOneAnnouncementRecvInput
	annoReceiveCreateRD     = generated.Announcement__Receive__CreateOneAnnouncementRecvResponseData
	getEffectiveAccountI    = generated.Account__GetEffectiveAccountsInput
	getEffectiveAccountRD   = generated.Account__GetEffectiveAccountsResponseData
	getManyAnnoConfByTypeI  = generated.Announcement__Conf__GetManyAnnoConfByTypeInput
	getManyAnnoConfByTypeRD = generated.Announcement__Conf__GetManyAnnoConfByTypeResponseData
)

var (
	createOneAnnoMP         = generated.Announcement__Send__CreateOneAnnouncement
	createOneAnnoReceiveMP  = generated.Announcement__Receive__CreateOneAnnouncementRecv
	getEffectiveAccountMP   = generated.Account__GetEffectiveAccounts
	getManyAnnoConfByTypeMP = generated.Announcement__Conf__GetManyAnnoConfByType
)

// init 注册钩子
func init() {
	base.AddRegisteredHook(startAnnouncementCron)
}

// startAnnouncementCron 开始定时任务
func startAnnouncementCron(logger echo.Logger) {
	c := cron.New()
	c.AddFunc("0 10 * * *", func() {
		err := getEffectiveAccounts(logger)
		if err != nil {
			logger.Error("Error in GetEffectiveAccounts: ", err)
		}
	})
	c.Start()
}

// getEffectiveAccounts 获取3天后到期用户
func getEffectiveAccounts(logger echo.Logger) (err error) {
	startTime := time.Now().AddDate(0, 0, 2)
	endTime := time.Now().AddDate(0, 0, 3)

	getEffectiveAccountInput := getEffectiveAccountI{
		StartTime: startTime.Format(utils.ISO8601Layout),
		EndTime:   endTime.Format(utils.ISO8601Layout),
	}
	client := plugins.DefaultInternalClient
	result, err := plugins.ExecuteInternalRequestQueries[getEffectiveAccountI, getEffectiveAccountRD](client, getEffectiveAccountMP, getEffectiveAccountInput)
	if err != nil {
		logger.Error("Error in GetEffectiveAccount: ", err)
		return
	}

	for _, data := range result.Data {
		logger.Info(fmt.Sprintf("发送会员3天到期提醒：用户ID[%s]", data.UserId))
		err = CreateAnnouncementByType(client, data.UserId, generated.Freetalk_AnnoType_MemberReminder, logger)
		if err != nil {
			logger.Error("Error in CreateNewUserAnnouncement: ", err)
		}
	}
	return
}

// CreateAnnouncementByType 创建消息
func CreateAnnouncementByType(c *base.InternalClient, receiverId string, annoType generated.Freetalk_AnnoType, logger echo.Logger) (err error) {
	getManyAnnoConfByTypeInput := getManyAnnoConfByTypeI{
		AnnoType: annoType,
	}
	result, err := plugins.ExecuteInternalRequestQueries[getManyAnnoConfByTypeI, getManyAnnoConfByTypeRD](c, getManyAnnoConfByTypeMP, getManyAnnoConfByTypeInput)
	if err != nil {
		logger.Error("Error in GetManyAnnoConfByType: ", err)
		return
	}
	for _, data := range result.Data {
		annoCreateInput := annoCreateI{
			Abstract: data.Abstract,
			AnnoType: generated.Freetalk_AnnoType_MemberReminder,
			Content:  data.Content,
			IsSend:   true,
			Title:    data.Title,
			UserId:   data.UserId,
		}
		var resultCreate annoCreateRD
		resultCreate, err = plugins.ExecuteInternalRequestMutations[annoCreateI, annoCreateRD](c, createOneAnnoMP, annoCreateInput)
		if err != nil {
			logger.Error("Error in CreateOneAnno: ", err)
			return
		}

		annoReceiveCreateInput := annoReceiveCreateI{
			AnnoId: resultCreate.Data.Id,
			UserId: receiverId,
		}
		_, err = plugins.ExecuteInternalRequestMutations[annoReceiveCreateI, annoReceiveCreateRD](c, createOneAnnoReceiveMP, annoReceiveCreateInput)
		if err != nil {
			logger.Error("Error in CreateOneAnnoReceive: ", err)
			return
		}
	}
	return
}
