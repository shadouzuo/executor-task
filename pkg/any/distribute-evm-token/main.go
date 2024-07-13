package distribute_evm_token

import (
	"context"
	"math/big"
	"strings"
	"time"

	"github.com/pkg/errors"

	go_best_type "github.com/pefish/go-best-type"
	go_coin_eth "github.com/pefish/go-coin-eth"
	go_crypto "github.com/pefish/go-crypto"
	go_decimal "github.com/pefish/go-decimal"
	go_format "github.com/pefish/go-format"
	go_mysql "github.com/pefish/go-mysql"
	"github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/global"
)

type DistributeEthType struct {
	go_best_type.BaseBestType
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

func New(name string) *DistributeEthType {
	t := &DistributeEthType{}
	t.BaseBestType = *go_best_type.NewBaseBestType(t, name)
	return t
}

func (p *DistributeEthType) Start(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
	task := ask.Data.(ActionTypeData).Task

	err := p.init(task)
	if err != nil {
		ask.AnswerChan <- constant.TaskResult{
			BestType: p,
			Task:     task,
			Data:     "",
			Err:      err,
		}
		return nil
	}

	timer := time.NewTimer(0)
	for {
		select {
		case <-timer.C:
			err := p.do(task)
			if err != nil {
				ask.AnswerChan <- constant.TaskResult{
					BestType: p,
					Task:     task,
					Data:     "",
					Err:      err,
				}
				p.BestTypeManager().ExitSelf(p.Name())
				return nil
			}
			if task.Interval != 0 {
				timer.Reset(time.Duration(task.Interval) * time.Second)
				continue
			}
			ask.AnswerChan <- constant.TaskResult{
				BestType: p,
				Task:     task,
				Data:     "result",
				Err:      nil,
			}
			p.BestTypeManager().ExitSelf(p.Name())
			return nil
		case exitType := <-exitChan:
			switch exitType {
			case go_best_type.ExitType_System:
				ask.AnswerChan <- constant.TaskResult{
					BestType: p,
					Task:     task,
					Data:     "",
					Err:      errors.New("Exited by system."),
				}
				return nil
			case go_best_type.ExitType_User:
				ask.AnswerChan <- constant.TaskResult{
					BestType: p,
					Task:     task,
					Data:     "",
					Err:      errors.New("Exited by user."),
				}
				return nil
			}
		}
	}
}

func (p *DistributeEthType) ProcessOtherAsk(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
	return nil
}

func (p *DistributeEthType) init(task *constant.Task) error {
	var config Config
	err := go_format.FormatInstance.MapToStruct(&config, task.Data)
	if err != nil {
		return err
	}
	p.config = &config

	p.ethWallet = go_coin_eth.NewWallet()
	p.ethWallet.InitRemote(&go_coin_eth.UrlParam{
		RpcUrl: p.config.EthNodeUrl,
	})
	return nil
}

func (p *DistributeEthType) do(task *constant.Task) error {
	priv, err := go_crypto.CryptoInstance.AesCbcDecrypt(global.GlobalConfig.Pass, p.config.Priv)
	if err != nil {
		return err
	}

	var taskObj constant.Task
	notFound, err := go_mysql.MysqlInstance.RawSelectFirst(
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
				p.Logger().InfoF("<%s> ignore.", address)
				continue
			}
			shouldTransfer = go_decimal.Decimal.MustStart(transferAmountWithDecimals).
				MustSub(balWithDecimals).
				MustUnShiftedBy(18).
				RoundUp(8).
				MustShiftedBy(18).
				MustEndForBigInt()

			p.Logger().InfoF("transfer to <%s> <%s> ETH.", address, shouldTransfer.String())
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
				p.Logger().InfoF("<%s> ignore.", address)
				continue
			}
			shouldTransfer = go_decimal.Decimal.MustStart(transferAmountWithDecimals).MustSub(balWithDecimals).RoundUp(0).MustEndForBigInt()

			p.Logger().InfoF("transfer to <%s> <%s>.", address, shouldTransfer.String())
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
		p.Logger().InfoF("hash: %s", hash)
		p.ethWallet.WaitConfirm(context.Background(), hash, time.Second)
		p.Logger().InfoF("transfer to <%s> <%s> finished.", address, shouldTransfer.String())
	}

	return nil
}

func getTokenDecimals(wallet *go_coin_eth.Wallet, tokenAddress string) (uint64, error) {
	if tokenAddress == "" {
		return 18, nil
	}
	decimals, err := wallet.GetTokenDecimals(tokenAddress)
	if err != nil {
		return 0, err
	}
	return decimals, nil
}
