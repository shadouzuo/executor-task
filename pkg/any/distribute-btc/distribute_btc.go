package distribute_btc

import (
	"context"
	"strings"
	"time"

	"github.com/btcsuite/btcd/chaincfg"

	go_coin_btc "github.com/pefish/go-coin-btc"
	go_crypto "github.com/pefish/go-crypto"
	go_decimal "github.com/pefish/go-decimal"
	go_format "github.com/pefish/go-format"
	i_logger "github.com/pefish/go-interface/i-logger"
	t_mysql "github.com/pefish/go-interface/t-mysql"
	go_random "github.com/pefish/go-random"
	"github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/global"
	"github.com/shadouzuo/executor-task/pkg/util"
)

type DistributeBtcType struct {
	logger    i_logger.ILogger
	config    *Config
	btcWallet *go_coin_btc.Wallet
}

type Config struct {
	Wif              string   `json:"wif"`
	Amount           string   `json:"amount"`
	SelectAddressSql []string `json:"select_address_sql"`
	BtcNodeUrl       string   `json:"btc_node_url"`
}

type ActionTypeData struct {
	Task *constant.Task
}

func New(logger i_logger.ILogger) *DistributeBtcType {
	t := &DistributeBtcType{
		logger: logger,
	}
	return t
}

func (p *DistributeBtcType) Start(ctx context.Context, task *constant.Task) (any, error) {
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

func (p *DistributeBtcType) init(task *constant.Task) error {
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

func (p *DistributeBtcType) do(task *constant.Task) error {
	wif, err := go_crypto.AesCbcDecrypt(global.GlobalConfig.Pass, p.config.Wif)
	if err != nil {
		return err
	}
	keyInfo, err := p.btcWallet.KeyInfoFromWif(wif)
	if err != nil {
		return err
	}
	fromAddress, err := p.btcWallet.AddressFromPubKey(keyInfo.PubKey, go_coin_btc.ADDRESS_TYPE_P2TR)
	if err != nil {
		return err
	}

	addressDbs := make([]*constant.BtcAddress, 0)
	err = global.MysqlInstance.RawSelect(
		&addressDbs,
		p.config.SelectAddressSql[0],
		p.config.SelectAddressSql[1],
	)
	if err != nil {
		return err
	}

	for _, addressDb := range addressDbs {
		err := util.CheckUnConfirmedCountAndWait(p.logger, task)
		if err != nil {
			return err
		}

		var btcAddressDb constant.BtcAddress
		notFound, err := global.MysqlInstance.SelectById(
			&btcAddressDb,
			&t_mysql.SelectByIdParams{
				TableName: "btc_address",
				Select:    "*",
				Id:        addressDb.Id,
			},
		)
		if err != nil {
			return err
		}
		if notFound {
			continue
		}

		arr := strings.Split(p.config.Amount, "-")
		var targetAmount float64
		if len(arr) == 1 {
			targetAmount = go_format.MustToFloat64(arr[0])
		} else {
			startAmount := go_format.MustToFloat64(arr[0])
			endAmount := go_format.MustToFloat64(arr[1])
			targetAmount = go_decimal.Decimal.MustStart(go_random.MustRandomFloat64(startAmount, endAmount)).Round(8).MustEndForFloat64()
		}

		p.logger.InfoF("Prepare send <%s> <%f> BTC.", btcAddressDb.Address, targetAmount)
		txId, msgTx, spentUtxos, newUtxos, err := util.SendBtc(
			p.logger,
			p.btcWallet,
			keyInfo.PrivKey,
			fromAddress,
			btcAddressDb.Address,
			targetAmount,
		)
		if err != nil {
			return err
		}

		// 保存交易记录
		p.logger.InfoF("Save tx record...")
		txHex, _ := p.btcWallet.MsgTxToHex(msgTx)
		_, err = global.MysqlInstance.Insert(
			"btc_tx",
			constant.BtcTx{
				TaskId:  task.Id,
				TxId:    txId,
				Confirm: 0,
				TxHex:   txHex,
			},
		)
		if err != nil {
			return err
		}

		// 更新 utxo
		p.logger.InfoF("Update from address utxo...")
		fromAddrNewUtxos := make([]constant.UTXO, 0)
		for address, outPoints := range newUtxos {
			if strings.EqualFold(address, fromAddress) {
				for _, outPoint := range outPoints {
					fromAddrNewUtxos = append(fromAddrNewUtxos, constant.UTXO{
						TxId:  outPoint.Hash,
						Index: uint64(outPoint.Index),
					})
				}
			}
		}
		err = util.UpdateUtxo(
			fromAddress,
			spentUtxos,
			fromAddrNewUtxos,
		)
		if err != nil {
			return err
		}

		p.logger.InfoF("Update to address utxo...")
		toAddrNewUtxos := make([]constant.UTXO, 0)
		for address, outPoints := range newUtxos {
			if strings.EqualFold(address, fromAddress) {
				for _, outPoint := range outPoints {
					toAddrNewUtxos = append(toAddrNewUtxos, constant.UTXO{
						TxId:  outPoint.Hash,
						Index: uint64(outPoint.Index),
					})
				}
			}
		}
		err = util.UpdateUtxo(
			btcAddressDb.Address,
			nil,
			toAddrNewUtxos,
		)
		if err != nil {
			return err
		}
		p.logger.InfoF("Distribute done. address id: %d, amount: %f", addressDb.Id, targetAmount)
	}

	return nil
}
