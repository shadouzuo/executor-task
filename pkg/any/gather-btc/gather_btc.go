package gather_btc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/pkg/errors"

	go_coin_btc "github.com/pefish/go-coin-btc"
	go_crypto "github.com/pefish/go-crypto"
	go_decimal "github.com/pefish/go-decimal"
	go_format "github.com/pefish/go-format"
	go_format_slice "github.com/pefish/go-format/slice"
	go_format_type "github.com/pefish/go-format/type"
	i_logger "github.com/pefish/go-interface/i-logger"
	t_mysql "github.com/pefish/go-interface/t-mysql"
	"github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/global"
	"github.com/shadouzuo/executor-task/pkg/util"
)

type GatherBtcType struct {
	logger    i_logger.ILogger
	config    *Config
	btcWallet *go_coin_btc.Wallet
}

type Config struct {
	Mnemonic         string   `json:"mnemonic"`
	Pass             string   `json:"pass"`
	SelectAddressSql []string `json:"select_address_sql"`
	TargetAddressId  uint64   `json:"target_address_id"`
	BtcNodeUrl       string   `json:"btc_node_url"`
	FeeRate          int64    `json:"fee_rate"`
	Batch            uint64   `json:"batch"`
}

type ActionTypeData struct {
	Task *constant.Task
}

func New(logger i_logger.ILogger) *GatherBtcType {
	t := &GatherBtcType{
		logger: logger,
	}
	return t
}

func (p *GatherBtcType) Start(ctx context.Context, task *constant.Task) (any, error) {
	err := p.init(task)
	if err != nil {
		return nil, err
	}

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

func (p *GatherBtcType) init(task *constant.Task) error {
	var config Config
	err := go_format.MapToStruct(&config, task.Data)
	if err != nil {
		return err
	}
	p.config = &config

	p.btcWallet = go_coin_btc.NewWallet(&chaincfg.MainNetParams, p.logger)
	p.btcWallet.InitRpcClient(&go_coin_btc.RpcServerConfig{
		Url: config.BtcNodeUrl,
	}, 10*time.Second)
	return nil
}

func (p *GatherBtcType) do(task *constant.Task) error {

	addresses := make([]*constant.BtcAddress, 0)
	err := global.MysqlInstance.RawSelect(
		&addresses,
		p.config.SelectAddressSql[0],
		p.config.SelectAddressSql[1],
	)
	if err != nil {
		return err
	}

	var targetAddrDb constant.BtcAddress
	notFound, err := global.MysqlInstance.SelectById(
		&targetAddrDb,
		&t_mysql.SelectByIdParams{
			TableName: "btc_address",
			Select:    "*",
			Id:        p.config.TargetAddressId,
		},
	)
	if err != nil {
		return err
	}
	if notFound {
		return errors.New("Target address not found.")
	}

	slippedAddresses := go_format_slice.Group(addresses, &go_format_type.GroupOpts{
		GroupCount: int(p.config.Batch),
	})

	for _, addresses := range slippedAddresses {
		err := util.CheckUnConfirmedCountAndWait(p.logger, task)
		if err != nil {
			return err
		}

		err = p.gatherBtc(task, addresses, &targetAddrDb)
		if err != nil {
			return err
		}
		indexes := make([]string, 0)
		for _, addressDb := range addresses {
			indexes = append(indexes, go_format.ToString(addressDb.Index))
		}
		p.logger.InfoF("Address indexes <%s> gather done.", strings.Join(indexes, ","))
	}

	return nil
}

func (p *GatherBtcType) gatherBtc(
	task *constant.Task,
	addressDbs []*constant.BtcAddress,
	toAddrDb *constant.BtcAddress,
) error {
	feeRate := p.config.FeeRate
	if feeRate == 0 {
		feeRate_, err := p.btcWallet.RpcClient.EstimateSmartFee()
		if err != nil {
			return err
		}
		feeRate = go_decimal.Decimal.MustStart(feeRate_).RoundDown(0).MustEndForInt64()
	}

	seedPass, err := go_crypto.AesCbcDecrypt(global.GlobalConfig.Pass, p.config.Pass)
	if err != nil {
		return err
	}
	seedHex := p.btcWallet.SeedHexByMnemonic(p.config.Mnemonic, seedPass)

	outPoints := make([]*go_coin_btc.OutPoint, 0)
	for _, addressDb := range addressDbs {
		fromAddressUtxos := make([]constant.UTXO, 0)
		if addressDb.Utxos == nil {
			p.logger.ErrorF("index <%d> no utxos.", addressDb.Index)
			continue
		}
		err := json.Unmarshal([]byte(*addressDb.Utxos), &fromAddressUtxos)
		if err != nil {
			return err
		}
		if len(fromAddressUtxos) == 0 {
			p.logger.ErrorF("index <%d> no utxos.", addressDb.Index)
			continue
		}

		keyInfo, err := p.btcWallet.DeriveBySeedPath(seedHex, fmt.Sprintf("m/86'/0'/0'/0/%d", addressDb.Index))
		if err != nil {
			return err
		}
		err = p.btcWallet.AddAccountByPrivKey(keyInfo.PrivKey)
		if err != nil {
			return err
		}

		for _, utxo := range fromAddressUtxos {
			outPoints = append(outPoints, &go_coin_btc.OutPoint{
				Hash:  utxo.TxId,
				Index: int(utxo.Index),
			})
		}
	}

	if len(outPoints) == 0 {
		p.logger.InfoF("Balance not enough. no utxo")
		return nil
	}

	p.logger.InfoF("Build tx...")
	msgTx, newUtxos, realFee, err := p.btcWallet.BuildTx(
		outPoints,
		"",
		toAddrDb.Address,
		0,
		feeRate,
	)
	if err != nil {
		return err
	}
	txHex, err := p.btcWallet.MsgTxToHex(msgTx)
	if err != nil {
		return err
	}
	p.logger.InfoF("feeRate: %f, realFee: %f, hex: %s", feeRate, realFee, txHex)

	// 发送交易
	p.logger.InfoF("Send tx...")
	txId, err := p.btcWallet.RpcClient.SendMsgTx(msgTx)
	if err != nil {
		return err
	}

	// 保存交易记录
	p.logger.InfoF("Save tx record...")
	_, err = global.MysqlInstance.Insert(
		"btc_tx",
		constant.BtcTx{
			TaskId:  task.Id,
			TxId:    txId,
			TxHex:   txHex,
			Confirm: 0,
		},
	)
	if err != nil {
		return err
	}

	// 更新 utxo
	addresses := make([]string, 0)
	for _, addressDb := range addressDbs {
		addresses = append(addresses, addressDb.Address)
	}
	p.logger.InfoF("Update utxo...")
	_, err = global.MysqlInstance.Update(
		&t_mysql.UpdateParams{
			TableName: "btc_address",
			Update: map[string]interface{}{
				"utxos": "[]",
			},
			Where: map[string]interface{}{
				"address": addresses,
			},
		},
	)
	if err != nil {
		return err
	}

	toAddressUtxos := make([]constant.UTXO, 0)
	if toAddrDb.Utxos != nil {
		err := json.Unmarshal([]byte(*toAddrDb.Utxos), &toAddressUtxos)
		if err != nil {
			return err
		}
	}
	for address, utxos := range newUtxos {
		if strings.EqualFold(address, toAddrDb.Address) {
			for _, utxo := range utxos {
				toAddressUtxos = append(toAddressUtxos, constant.UTXO{
					TxId:  utxo.Hash,
					Index: uint64(utxo.Index),
				})
			}
		}
	}

	b, _ := json.Marshal(toAddressUtxos)
	_, err = global.MysqlInstance.Update(
		&t_mysql.UpdateParams{
			TableName: "btc_address",
			Update: map[string]interface{}{
				"utxos": string(b),
			},
			Where: map[string]interface{}{
				"address": toAddrDb.Address,
			},
		},
	)
	if err != nil {
		return err
	}

	return nil
}
