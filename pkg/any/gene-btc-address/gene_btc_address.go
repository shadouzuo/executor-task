package gene_btc_address

import (
	"context"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/chaincfg"

	go_coin_btc "github.com/pefish/go-coin-btc"
	go_crypto "github.com/pefish/go-crypto"
	go_format "github.com/pefish/go-format"
	i_logger "github.com/pefish/go-interface/i-logger"
	"github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/global"
)

type GeneBtcAddressType struct {
	logger i_logger.ILogger
}

type ActionTypeData struct {
	Task *constant.Task
}

type GeneBtcAddressConfig struct {
	Mnemonic   string `json:"mnemonic"`
	Pass       string `json:"pass"`
	StartIndex uint64 `json:"start_index"`
	EndIndex   uint64 `json:"end_index"`
	TaskId     uint64 `json:"task_id"`
}

func New(logger i_logger.ILogger) *GeneBtcAddressType {
	t := &GeneBtcAddressType{
		logger: logger,
	}
	return t
}

func (p *GeneBtcAddressType) Start(ctx context.Context, task *constant.Task) (any, error) {
	timer := time.NewTimer(0)
	for {
		select {
		case <-timer.C:
			err := p.do(task)
			if err != nil {
				return nil, err
			}
			if task.Interval != 0 {
				timer.Reset(time.Duration(task.Interval) * time.Second)
				continue
			}
			return nil, nil
		case <-ctx.Done():
			return nil, nil
		}
	}
}

func (p *GeneBtcAddressType) do(task *constant.Task) error {
	var config GeneBtcAddressConfig
	err := go_format.MapToStruct(&config, task.Data)
	if err != nil {
		return err
	}

	addresses := make([]constant.BtcAddress, 0)

	wallet := go_coin_btc.NewWallet(&chaincfg.MainNetParams, p.logger)
	seedPass, err := go_crypto.AesCbcDecrypt(global.GlobalConfig.Pass, config.Pass)
	if err != nil {
		return err
	}
	seedHex := wallet.SeedHexByMnemonic(config.Mnemonic, seedPass)
	for i := config.StartIndex; i < config.EndIndex; i++ {
		result, err := wallet.DeriveBySeedPath(seedHex, fmt.Sprintf("m/86'/0'/0'/0/%d", i))
		if err != nil {
			return err
		}
		taprootAddr, err := wallet.AddressFromPubKey(result.PubKey, go_coin_btc.ADDRESS_TYPE_P2TR)
		if err != nil {
			return err
		}
		addresses = append(addresses, constant.BtcAddress{
			Address: taprootAddr,
			Index:   i,
		})
	}

	_, err = global.MysqlInstance.Insert("btc_address", addresses)
	if err != nil {
		return err
	}

	return nil
}
