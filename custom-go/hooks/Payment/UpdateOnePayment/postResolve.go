package UpdateOnePayment

import (
	"custom-go/customize"
	"custom-go/generated"
	"custom-go/pkg/base"
	"custom-go/pkg/plugins"
	"custom-go/pkg/utils"
	"fmt"
	"time"
)

type (
	getOneAccountI  = generated.Account__GetOneAccountInternalInput
	getOneAccountRD = generated.Account__GetOneAccountResponseData
	setMembershipI  = generated.Account__SetMembershipInternalInput
	setMembershipRD = generated.Account__SetMembershipResponseData
	membershipI     = generated.Membership__GetOneMembershipInput
	membershipRD    = generated.Membership__GetOneMembershipResponseData
)

var (
	getOneAccountPath = generated.Account__GetOneAccount
	setMembershipPath = generated.Account__SetMembership
	membershipPath    = generated.Membership__GetOneMembership
)

// PostResolve UpdateOnePayment后置钩子
// 1.发送 会员 或 时长包 购买成功通知
// 2.修改Account的Membership关联，重置membershipEndTime
func PostResolve(hook *base.HookRequest, body generated.Payment__UpdateOnePaymentBody) (res generated.Payment__UpdateOnePaymentBody, err error) {
	// 1.发送 会员 或 时长包 购买成功通知
	err = customize.CreateAnnouncementByType(hook.InternalClient, body.Response.Data.Data.AccountId, generated.Freetalk_AnnoType(body.Response.Data.Data.Usage), hook.Logger())
	if err != nil {
		err = fmt.Errorf("创建通知失败: %s", err)
		return
	}
	// 2.修改Account的Membership关联，重置membershipEndTime
	if body.Response.Data.Data.Usage == generated.Freetalk_PaymentUsage_Membership {
		err = setAccountMembership(hook, body)
		if err != nil {
			err = fmt.Errorf("修改Account的Membership关联失败: %s", err)
		}
	}
	return
}

func setAccountMembership(hook *base.HookRequest, body generated.Payment__UpdateOnePaymentBody) (err error) {
	accountInput := getOneAccountI{UserId: body.Response.Data.Data.AccountId}
	accountRes, err := plugins.ExecuteInternalRequestQueries[getOneAccountI, getOneAccountRD](hook.InternalClient, getOneAccountPath, accountInput)
	if err != nil {
		err = fmt.Errorf("获取Account失败: %s", err)
		return
	}

	membershipEndTime, err := calcMembershipEndTime(hook, body, accountRes)
	if err != nil {
		err = fmt.Errorf("计算会员过期时间失败: %s", err)
		return
	}

	setMembershipInput := setMembershipI{
		Id:                accountRes.Data.Id,
		MembershipId:      body.Response.Data.Data.UsageId,
		MembershipEndTime: membershipEndTime,
	}
	_, err = plugins.ExecuteInternalRequestMutations[setMembershipI, setMembershipRD](hook.InternalClient, setMembershipPath, setMembershipInput)
	if err != nil {
		err = fmt.Errorf("修改Account的Membership关联失败: %s", err)
	}
	return
}

func calcMembershipEndTime(hook *base.HookRequest, body generated.Payment__UpdateOnePaymentBody, accountRes getOneAccountRD) (membershipEndTime string, err error) {
	membershipInput := membershipI{Id: body.Response.Data.Data.UsageId}
	membershipRes, err := plugins.ExecuteInternalRequestQueries[membershipI, membershipRD](hook.InternalClient, membershipPath, membershipInput)

	// 支付时间
	paymentDate, err := time.Parse(utils.ISO8601Layout, body.Input.PaymentDate)
	if err != nil {
		err = fmt.Errorf("解析支付时间失败: %s", err)
		return
	}

	// 1.如果会员已经过期，过期时间 = 支付时间 + 新会员时长
	if utils.CurrentDateTime() > accountRes.Data.MembershipEndTime {
		membershipEndTime = paymentDate.Add(time.Duration(membershipRes.Data.Lifespan) * time.Hour * 24).Format(utils.ISO8601Layout)
		hook.Logger().Infof("【办理会员】，新过期时间: %s", membershipEndTime)
		return
	}

	// 当前会员过期时间
	accountMembershipEndTime, err := time.Parse(utils.ISO8601Layout, accountRes.Data.MembershipEndTime)
	if err != nil {
		err = fmt.Errorf("解析当前会员过期时间失败: %s", err)
		return
	}

	// 2.如果会员没过期
	// 2.1.如果要买的会员时长 > 当前会员时长，过期时间 = 支付时间 + 新会员时长
	if membershipRes.Data.Lifespan > accountRes.Data.Membership.Lifespan {
		membershipEndTime = paymentDate.Add(time.Duration(membershipRes.Data.Lifespan) * time.Hour * 24).Format(utils.ISO8601Layout)
		hook.Logger().Infof("【升级会员】，新过期时间: %s", membershipEndTime)
		return
	}

	// 2.2.如果要买的会员时长 <= 当前会员时长，过期时间 = 当前会员过期时间 + 新会员时长
	if membershipRes.Data.Lifespan <= accountRes.Data.Membership.Lifespan {
		membershipEndTime = accountMembershipEndTime.Add(time.Duration(membershipRes.Data.Lifespan) * time.Hour * 24).Format(utils.ISO8601Layout)
		hook.Logger().Infof("【续费会员】，新过期时间: %s", membershipEndTime)
	}
	return
}
