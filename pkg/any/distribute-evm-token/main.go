package distribute_evm_token

import (
	"context"
	"math/big"
	"strings"
	"time"

	"github.com/pkg/errors"

	go_coin_eth "github.com/pefish/go-coin-eth"
	go_crypto "github.com/pefish/go-crypto"
	go_decimal "github.com/pefish/go-decimal"
	go_format "github.com/pefish/go-format"
	i_logger "github.com/pefish/go-interface/i-logger"
	"github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/global"
)

type DistributeEthType struct {
	logger    i_logger.ILogger
	config    *Config
	ethWallet *go_coin_eth.Wallet
}

type Config struct {
	Priv             string   `json:"priv"`
	Amount           string   `json:"amount"`
	TokenAddress     string   `json:"token_address"`
	SelectAddressSql []string `json:"select_address_sql"`
	EthNodeUrl       string   `json:"eth_node_url"`
}

type ActionTypeData struct {
	Task *constant.Task
}

func New(logger i_logger.ILogger) *DistributeEthType {
	t := &DistributeEthType{
		logger: logger,
	}
	return t
}

func (p *DistributeEthType) Start(ctx context.Context, task *constant.Task) (any, error) {
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

func (p *DistributeEthType) init(task *constant.Task) error {
	var config Config
	err := go_format.MapToStruct(&config, task.Data)
	if err != nil {
		return err
	}
	p.config = &config

	p.ethWallet = go_coin_eth.NewWallet(p.logger)
	p.ethWallet.InitRemote(&go_coin_eth.UrlParam{
		RpcUrl: p.config.EthNodeUrl,
	})
	return nil
}

func (p *DistributeEthType) do(task *constant.Task) error {
	priv, err := go_crypto.AesCbcDecrypt(global.GlobalConfigInDb.Pass, p.config.Priv)
	if err != nil {
		return err
	}

	var taskObj constant.Task
	notFound, err := global.MysqlInstance.RawSelectFirst(
		&taskObj,
		p.config.SelectAddressSql[0],
		p.config.SelectAddressSql[1],
	)
	if err != nil {
		return err
	}
	if notFound {
		return errors.New("Not found")
	}
	addresses := strings.Split(taskObj.Mark, ",")

	tokenDecimals, err := getTokenDecimals(p.ethWallet, p.config.TokenAddress)
	if err != nil {
		return err
	}

	for _, address := range addresses {
		transferAmountWithDecimals := go_decimal.Decimal.MustStart(p.config.Amount).MustShiftedBy(tokenDecimals).RoundDown(0).MustEndForBigInt()

		var hash string
		var shouldTransfer *big.Int
		if p.config.TokenAddress == "" {
			balWithDecimals, err := p.ethWallet.Balance(address)
			if err != nil {
				return err
			}
			if go_decimal.Decimal.MustStart(balWithDecimals).MustGte(transferAmountWithDecimals) {
				p.logger.InfoF("<%s> ignore.", address)
				continue
			}
			shouldTransfer = go_decimal.Decimal.MustStart(transferAmountWithDecimals).
				MustSub(balWithDecimals).
				MustUnShiftedBy(18).
				RoundUp(8).
				MustShiftedBy(18).
				MustEndForBigInt()

			p.logger.InfoF("transfer to <%s> <%s> ETH.", address, shouldTransfer.String())
			tx, err := p.ethWallet.BuildTransferTx(
				priv,
				address,
				&go_coin_eth.BuildTransferTxOpts{
					CallMethodOpts: go_coin_eth.CallMethodOpts{
						Value:    shouldTransfer,
						GasLimit: 30000,
					},
					Payload:  nil,
					IsLegacy: true,
				},
			)
			if err != nil {
				return err
			}
			hash_, err := p.ethWallet.SendRawTransaction(tx.TxHex)
			if err != nil {
				return err
			}
			hash = hash_
		} else {
			balWithDecimals, err := p.ethWallet.TokenBalance(p.config.TokenAddress, address)
			if err != nil {
				return err
			}
			if go_decimal.Decimal.MustStart(balWithDecimals).MustGte(transferAmountWithDecimals) {
				p.logger.InfoF("<%s> ignore.", address)
				continue
			}
			shouldTransfer = go_decimal.Decimal.MustStart(transferAmountWithDecimals).MustSub(balWithDecimals).RoundUp(0).MustEndForBigInt()

			p.logger.InfoF("transfer to <%s> <%s>.", address, shouldTransfer.String())
			hash_, err := p.ethWallet.SendToken(
				priv,
				p.config.TokenAddress,
				address,
				shouldTransfer,
				nil,
			)
			if err != nil {
				return err
			}
			hash = hash_
		}
		p.logger.InfoF("hash: %s", hash)
		p.ethWallet.WaitConfirm(context.Background(), hash, time.Second)
		p.logger.InfoF("transfer to <%s> <%s> finished.", address, shouldTransfer.String())
	}

	return nil
}

func getTokenDecimals(wallet *go_coin_eth.Wallet, tokenAddress string) (uint64, error) {
	if tokenAddress == "" {
		return 18, nil
	}
	decimals, err := wallet.TokenDecimals(tokenAddress)
	if err != nil {
		return 0, err
	}
	return decimals, nil
}
