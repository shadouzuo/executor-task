package db

import (
	"time"

	"github.com/shadouzuo/executor-task/pkg/constant"
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
	Name     string                  `json:"name"`
	Desc     string                  `json:"desc"`
	Interval uint64                  `json:"interval"`
	Data     map[string]interface{}  `json:"data"`
	Status   constant.TaskStatusType `json:"status,omitempty"`

	DbTime
}
