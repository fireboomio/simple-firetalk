package CreateOneUser

import (
	"custom-go/customize"
	"custom-go/generated"
	"custom-go/pkg/base"
	"custom-go/pkg/plugins"
)

type (
	annoReceiveCreateI   = generated.Announcement__Receive__CreateOneAnnouncementRecvInput
	annoReceiveCreateRD  = generated.Announcement__Receive__CreateOneAnnouncementRecvResponseData
	getManyAnnoByScopeI  = generated.Announcement__Send__GetManyAnnouncementByScopeInput
	getManyAnnoByScopeRD = generated.Announcement__Send__GetManyAnnouncementByScopeResponseData
)

var (
	createOneAnnoReceiveMP = generated.Announcement__Receive__CreateOneAnnouncementRecv
	getManyAnnoByScopeMP   = generated.Announcement__Send__GetManyAnnouncementByScope
)

const (
	newUserRegistryAnnoType = "UserRegistration"
	systemAnnoScope         = "ALL"
)

// PostResolve 用户注册后的后置处理
// 1.对用户发送新注册消息推送
// 2.对用户补发系统消息推送
func PostResolve(hook *base.HookRequest, body generated.User__CreateOneUserBody) (res generated.User__CreateOneUserBody, err error) {
	// 1.对用户发送新注册消息推送
	err = customize.CreateAnnouncementByType(hook.InternalClient, body.Input.Id, newUserRegistryAnnoType, hook.Logger())
	if err != nil {
		return
	}

	// 2.对用户补发系统消息推送
	err = reissueSystemAnnouncement(hook.InternalClient, body.Input.Id)
	return
}

// reissueSystemAnnouncement 补发系统消息
func reissueSystemAnnouncement(c *base.InternalClient, userId string) (err error) {
	getManyAnnoByScopeInput := getManyAnnoByScopeI{
		AnnoScope: systemAnnoScope,
	}
	result, err := plugins.ExecuteInternalRequestQueries[getManyAnnoByScopeI, getManyAnnoByScopeRD](c, getManyAnnoByScopeMP, getManyAnnoByScopeInput)
	if err != nil {
		return
	}
	for _, anno := range result.Data {
		err = createAnnouncementReceive(c, anno.Id, userId)
		if err != nil {
			return err
		}
	}
	return
}

// createAnnouncementReceive 创建消息接收记录
func createAnnouncementReceive(c *base.InternalClient, annoId string, userId string) (err error) {
	annoReceiveCreateInput := annoReceiveCreateI{
		AnnoId: annoId,
		UserId: userId,
	}
	_, err = plugins.ExecuteInternalRequestMutations[annoReceiveCreateI, annoReceiveCreateRD](c, createOneAnnoReceiveMP, annoReceiveCreateInput)
	return
}
