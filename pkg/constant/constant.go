package constant

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

type UTXO struct {
	TxId  string  `json:"tx_id"`
	Index uint64  `json:"index"`
	Value float64 `json:"value"`
}
