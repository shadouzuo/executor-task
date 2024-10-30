package any_

import (
	"context"

	"github.com/shadouzuo/executor-task/pkg/constant"
)

type IExecutor interface {
	Start(ctx context.Context, task *constant.Task) (any, error)
}
