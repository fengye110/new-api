package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

type QuotaSetting struct {
	EnableFreeModelPreConsume        bool `json:"enable_free_model_pre_consume"`          // 是否对免费模型启用预消耗
	AutoSubscribeFreePlansForNewUser bool `json:"auto_subscribe_free_plans_for_new_user"` // 是否为新用户自动订阅可见的零价套餐
}

// 默认配置
var quotaSetting = QuotaSetting{
	EnableFreeModelPreConsume:        true,
	AutoSubscribeFreePlansForNewUser: false,
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("quota_setting", &quotaSetting)
}

func GetQuotaSetting() *QuotaSetting {
	return &quotaSetting
}
