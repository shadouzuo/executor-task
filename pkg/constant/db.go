package constant

import (
	"time"
)

type DbTime struct {
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type IdType struct {
	Id uint64 `json:"id,omitempty"`
}

type Task struct {
	IdType
	Name     string                 `json:"name"`
	Desc     string                 `json:"desc"`
	Interval uint64                 `json:"interval"`
	Data     map[string]interface{} `json:"data"`
	Status   TaskStatusType         `json:"status,omitempty"`
	Mark     string                 `json:"mark"`

	DbTime
}

type TaskRecord struct {
	IdType
	Name     string                 `json:"name"`
	Interval uint64                 `json:"interval"`
	Data     map[string]interface{} `json:"data"`
	Mark     string                 `json:"mark"`

	DbTime
}

type BtcAddress struct {
	IdType
	Address string  `json:"address"`
	Index   uint64  `json:"index"`
	Utxos   *string `json:"utxos,omitempty"`

	DbTime
}

type BtcTx struct {
	IdType
	TaskId  uint64 `json:"task_id"`
	TxId    string `json:"tx_id"`
	TxHex   string `json:"tx_hex"`
	Confirm uint64 `json:"confirm"`
	DbTime
}

type BtcUtxo struct {
	IdType
	Address string `json:"address"`
	TxId    string `json:"tx_id"`
	Index   uint64 `json:"index"`
	Value   string `json:"value"`
	Status  uint64 `json:"status"`
	DbTime
}
