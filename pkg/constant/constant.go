package constant

import go_best_type "github.com/pefish/go-best-type"

type TaskStatusType uint64

const (
	TaskStatusType_WaitExec      TaskStatusType = 1
	TaskStatusType_Executing     TaskStatusType = 2
	TaskStatusType_WaitExit      TaskStatusType = 5
	TaskStatusType_Exited        TaskStatusType = 3
	TaskStatusType_ExitedWithErr TaskStatusType = 4
)

type TaskTypeType uint64

const (
	TaskTypeType_Test TaskTypeType = 0
)

const (
	ActionType_Start go_best_type.ActionType = "start"
	ActionType_Stop  go_best_type.ActionType = "stop"
)
