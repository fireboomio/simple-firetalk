## 初始化数据库
1. 在store/list/FbDataSource中修改数据源freetalk连接或在本地启动postgres
2. 使用prisma.txt在飞布控制台-数据建模界面迁移模型
3. 在postgres数据库中执行[create extension "pgcrypto"]，添加gen_random_uuid()插件

## 替换authing身份验证配置
1. 在custom-go/.env中修改[AUTHING]开头的变量
2. 取消注释custom-go/customize/authing.go中174行(防止因配置错误导致服务无法启动)

## 替换OCR配置
1. 修改custom-go/customize/OCR.go中[OCRApiKey]和[OCRSecretKey]

## 替换alipay支持配置
1. 将custom-go/doc/alipay下的文件替换为正式的证书

## 启动项目
1. 执行update.sh脚本下载fireboom二进制文件
2. 执行命令./fireboom dev
3. 下载golang钩子服务并启动go run main.go


## custom-go代码说明
### customize
1. [announcement_cron.go](custom-go%2Fcustomize%2Fannouncement_cron.go) 通知定时任务
2. [authing.go](custom-go%2Fcustomize%2Fauthing.go) authing短信登录
3. [gpt_chat.go](custom-go%2Fcustomize%2Fgpt_chat.go) ai聊天主体逻辑
4. [gpt_chat_azure.go](custom-go%2Fcustomize%2Fgpt_chat_azure.go) 微软gpt配置
5. [gpt_chat_openai.go](custom-go%2Fcustomize%2Fgpt_chat_openai.go) openai gpt配置
6. [OCR.go](custom-go%2Fcustomize%2FOCR.go) 百度图片识别文字
7. [payment.go](custom-go%2Fcustomize%2Fpayment.go) 支付主体逻辑
8. [payment_ali.go](custom-go%2Fcustomize%2Fpayment_ali.go) 支付宝支付(可用)
9. [payment_cron.go](custom-go%2Fcustomize%2Fpayment_cron.go) 支付定时任务，用来取消订单和确认订单支付

### doc/alipay/** 支付宝证书

### hooks
1. [preResolve.go](custom-go%2Fhooks%2FCDKEY%2FRedeemCDKEY%2FpreResolve.go) CDKEY兑换，判断是否可兑换
2. [postResolve.go](custom-go%2Fhooks%2FCDKEY%2FRedeemCDKEY%2FpostResolve.go) CDKEY兑换成功后添加时长记录和增加用户剩余时长
3. [preResolve.go](custom-go%2Fhooks%2FChat%2FCreateChat%2FpreResolve.go) 创建聊天前判断是否有剩余时长
4. [mutatingPostResolve.go](custom-go%2Fhooks%2FChat%2FGetRandomChat%2FmutatingPostResolve.go) 获取随便聊聊的会话，没有就新建后返回
5. [mutatingPreResolve.go](custom-go%2Fhooks%2FChat%2FMessage%2FCreateOneChatMessage%2FmutatingPreResolve.go) 创建消息前计算消耗时长并判断是否时长超限
6. [postResolve.go](custom-go%2Fhooks%2FChat%2FMessage%2FCreateOneChatMessage%2FpostResolve.go) 创建消息成功后添加时长记录和扣减用户剩余时长
7. [mutatingPostResolve.go](custom-go%2Fhooks%2FDuration%2FGetCostDurationHistory%2FmutatingPostResolve.go) 时长消耗记录按天分组后再返回
8. [preResolve.go](custom-go%2Fhooks%2FPayment%2FCreatePaymentMembership%2FpreResolve.go) 购买会员前判断是否超出购买次数限制
9. [postResolve.go](custom-go%2Fhooks%2FPayment%2FUpdateOnePayment%2FpostResolve.go) 购买会员成功后设置用户会员信息和添加时长记录
10. [mutatingPreResolve.go](custom-go%2Fhooks%2FScene%2FCreateOneUserScene%2FmutatingPreResolve.go) 用户自定义场景前关联对应的用户自定义分类
11. [postResolve.go](custom-go%2Fhooks%2FUser%2FCreateOneUser%2FpostResolve.go) 用户新建后发送通知

### proxys
1. [aliPay.go](custom-go%2Fproxys%2FpayNotify%2FaliPay.go) 支付宝支付回调处理