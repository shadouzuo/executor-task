package util

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	go_logger "github.com/pefish/go-logger"
	go_mysql "github.com/pefish/go-mysql"
	"github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/db"
)

func UpdateUtxo(
	address string,
	spentUtxos []constant.UTXO,
	newUtxos []constant.UTXO,
) error {
	var addrDb db.BtcAddress
	notFound, err := go_mysql.MysqlInstance.SelectFirst(
		&addrDb,
		&go_mysql.SelectParams{
			TableName: "btc_address",
			Select:    "*",
			Where: map[string]interface{}{
				"address": address,
			},
		},
	)
	if err != nil {
		return err
	}
	if notFound {
		return errors.New("Address not found.")
	}
	addressUtxos := make([]constant.UTXO, 0)
	if addrDb.Utxos != nil {
		err = json.Unmarshal([]byte(*addrDb.Utxos), &addressUtxos)
		if err != nil {
			return err
		}
	}

	newAddressUtxos := make([]constant.UTXO, 0)
	// 剔除用掉了的
	for _, addressUtxo := range addressUtxos {
		spent := false
		for _, spentUtxo := range spentUtxos {
			if strings.EqualFold(spentUtxo.TxId, addressUtxo.TxId) && spentUtxo.Index == addressUtxo.Index {
				spent = true
				break
			}
		}
		if !spent {
			newAddressUtxos = append(newAddressUtxos, addressUtxo)
		}
	}

	// 添加新的
	newAddressUtxos = append(newAddressUtxos, newUtxos...)

	// 更新
	b, _ := json.Marshal(newAddressUtxos)
	_, err = go_mysql.MysqlInstance.Update(
		&go_mysql.UpdateParams{
			TableName: "btc_address",
			Update: map[string]interface{}{
				"utxos": string(b),
			},
			Where: map[string]interface{}{
				"address": address,
			},
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func SelectUtxos(
	address string,
	targetAmount float64,
	estimateFee float64,
) ([]constant.UTXO, error) {
	var addrDb db.BtcAddress
	notFound, err := go_mysql.MysqlInstance.SelectFirst(
		&addrDb,
		&go_mysql.SelectParams{
			TableName: "btc_address",
			Select:    "*",
			Where: map[string]interface{}{
				"address": address,
			},
		},
	)
	if err != nil {
		return nil, err
	}
	if notFound {
		return nil, errors.New("Address not found.")
	}
	addressUtxos := make([]constant.UTXO, 0)
	if addrDb.Utxos == nil {
		return nil, errors.New("No utxos.")
	}
	err = json.Unmarshal([]byte(*addrDb.Utxos), &addressUtxos)
	if err != nil {
		return nil, err
	}

	needAmount := targetAmount + estimateFee

	results := make([]constant.UTXO, 0)
	sum := 0.0
	for _, addressUtxo := range addressUtxos {
		sum += addressUtxo.Value
		results = append(results, addressUtxo)
		if sum >= needAmount {
			break
		}
	}

	return results, nil

}

func CheckUnConfirmedCountAndWait(logger go_logger.InterfaceLogger, task *db.Task) error {
	count, err := go_mysql.MysqlInstance.Count(
		&go_mysql.CountParams{
			TableName: "btc_tx",
			Where: map[string]interface{}{
				"task_id": task.Id,
				"confirm": 0,
			},
		},
	)
	if err != nil {
		return err
	}
	if count >= 10 {
		logger.InfoF("UnConfirmed tx count >= 10, just wait.")
		time.Sleep(10 * time.Second)
		err = CheckUnConfirmedCountAndWait(logger, task)
		if err != nil {
			return err
		}
	}
	return nil
}
