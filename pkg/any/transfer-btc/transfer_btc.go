package transfer_btc

import (
	"context"
	"strings"
	"time"

	"github.com/btcsuite/btcd/chaincfg"

	go_coin_btc "github.com/pefish/go-coin-btc"
	go_crypto "github.com/pefish/go-crypto"
	go_format "github.com/pefish/go-format"
	i_logger "github.com/pefish/go-interface/i-logger"
	"github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/global"
	"github.com/shadouzuo/executor-task/pkg/util"
)

type TransferBtcType struct {
	logger    i_logger.ILogger
	config    *Config
	btcWallet *go_coin_btc.Wallet
}

type AddressInfo struct {
	Address string  `json:"address"`
	Amount  float64 `json:"amount"`
}

type Config struct {
	Wif          string        `json:"wif"`
	AddressInfos []AddressInfo `json:"address_infos"`
	BtcNodeUrl   string        `json:"btc_node_url"`
}

type ActionTypeData struct {
	Task *constant.Task
}

func New(logger i_logger.ILogger) *TransferBtcType {
	t := &TransferBtcType{
		logger: logger,
	}
	return t
}

func (p *TransferBtcType) Start(ctx context.Context, task *constant.Task) (any, error) {
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
			return "", nil
		case <-ctx.Done():
			return nil, nil
		}
	}
}

func (p *TransferBtcType) init(task *constant.Task) error {
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

func (p *TransferBtcType) do(task *constant.Task) error {
	wif, err := go_crypto.AesCbcDecrypt(global.GlobalConfigInDb.Pass, p.config.Wif)
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

	for _, addressInfo := range p.config.AddressInfos {
		err := util.CheckUnConfirmedCountAndWait(p.logger, task)
		if err != nil {
			return err
		}

		p.logger.InfoF("Prepare send <%s> <%f> BTC.", addressInfo.Address, addressInfo.Amount)
		_, _, spentUtxos, newUtxos, err := util.SendBtc(
			p.logger,
			p.btcWallet,
			keyInfo.PrivKey,
			fromAddress,
			addressInfo.Address,
			addressInfo.Amount,
		)
		if err != nil {
			return err
		}
		p.logger.InfoF("Update from address utxo...")
		fromAddrNewUtxos := make([]constant.UTXO, 0)
		for address, utxos := range newUtxos {
			if strings.EqualFold(address, fromAddress) {
				for _, utxo := range utxos {
					fromAddrNewUtxos = append(fromAddrNewUtxos, constant.UTXO{
						TxId:  utxo.Hash,
						Index: uint64(utxo.Index),
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
		p.logger.InfoF("Transfer done. address: %s, amount: %f", addressInfo.Address, addressInfo.Amount)
	}

	return nil
}
