package print_wifs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/btcsuite/btcd/chaincfg"

	go_coin_btc "github.com/pefish/go-coin-btc"
	go_crypto "github.com/pefish/go-crypto"
	go_format "github.com/pefish/go-format"
	i_logger "github.com/pefish/go-interface/i-logger"
	"github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/global"
)

type PrintWifsType struct {
	logger    i_logger.ILogger
	config    *Config
	btcWallet *go_coin_btc.Wallet
}

type Config struct {
	Mnemonic         string   `json:"mnemonic"`
	Pass             string   `json:"pass"`
	SelectAddressSql []string `json:"select_address_sql"`
}

type ActionTypeData struct {
	Task *constant.Task
}

func New(logger i_logger.ILogger) *PrintWifsType {
	t := &PrintWifsType{
		logger: logger,
	}
	return t
}

func (p *PrintWifsType) Start(ctx context.Context, task *constant.Task) (any, error) {
	err := p.init(task)
	if err != nil {
		return nil, err
	}

	timer := time.NewTimer(0)
	for {
		select {
		case <-timer.C:
			result, err := p.do(task)
			if err != nil {
				return nil, err
			}
			if task.Interval != 0 {
				timer.Reset(time.Duration(task.Interval) * time.Second)
				continue
			}
			return result, nil
		case <-ctx.Done():
			return nil, nil
		}
	}
}

func (p *PrintWifsType) init(task *constant.Task) error {
	var config Config
	err := go_format.MapToStruct(&config, task.Data)
	if err != nil {
		return err
	}
	p.config = &config

	p.btcWallet = go_coin_btc.NewWallet(&chaincfg.MainNetParams, p.logger)
	return nil
}

func (p *PrintWifsType) do(task *constant.Task) (interface{}, error) {

	addresses := make([]*constant.BtcAddress, 0)
	err := global.MysqlInstance.RawSelect(
		&addresses,
		p.config.SelectAddressSql[0],
		p.config.SelectAddressSql[1],
	)
	if err != nil {
		return "", err
	}
	seedPass, err := go_crypto.AesCbcDecrypt(global.GlobalConfigInDb.Pass, p.config.Pass)
	if err != nil {
		return "", err
	}
	seedHex := p.btcWallet.SeedHexByMnemonic(p.config.Mnemonic, seedPass)
	wifs := make([]string, 0)
	for _, addrDb := range addresses {
		keyInfo, err := p.btcWallet.DeriveBySeedPath(seedHex, fmt.Sprintf("m/86'/0'/0'/0/%d", addrDb.Index))
		if err != nil {
			return "", err
		}
		wifs = append(wifs, keyInfo.Wif)
	}

	return strings.Join(wifs, "\n"), nil
}
