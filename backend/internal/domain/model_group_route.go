package domain

// ModelGroupRoute 按客户端精确模型名选择实际调度和计费分组。
type ModelGroupRoute struct {
	Model         string `json:"model"`
	TargetGroupID int64  `json:"target_group_id"`
	Mode          string `json:"mode"`
	Enabled       bool   `json:"enabled"`
}

const (
	ModelGroupRouteTargetOnly  = "target_only"
	ModelGroupRoutePreferLocal = "prefer_local"
)
