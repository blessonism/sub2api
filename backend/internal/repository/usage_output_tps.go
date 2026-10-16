package repository

import (
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// usageOutputTPSValidSQL 统一运维与渠道卡片的文本输出速度样本，固定 SQL 别名为 ul。
var usageOutputTPSValidSQL = fmt.Sprintf(`
	AND ul.output_tokens > 0 AND ul.duration_ms > 0
	AND ul.image_count = 0 AND ul.image_output_tokens = 0
	AND COALESCE(ul.billing_mode, '') <> 'image'
	AND ul.request_type IN (%d, %d, %d, %d)`,
	service.RequestTypeSync, service.RequestTypeStream, service.RequestTypeWSV2, service.RequestTypeCyberBlocked)
