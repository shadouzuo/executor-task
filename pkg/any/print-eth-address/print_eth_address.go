package print_eth_address

import (
	"context"
	"fmt"
	"strings"
	"time"

	go_coin_eth "github.com/pefish/go-coin-eth"
	go_crypto "github.com/pefish/go-crypto"
	go_format "github.com/pefish/go-format"
	i_logger "github.com/pefish/go-interface/i-logger"
	"github.com/shadouzuo/executor-task/pkg/constant"
)

type PrintEthAddressType struct {
	logger i_logger.ILogger
}

type ActionTypeData struct {
	Task *constant.Task
}

type PrintEthAddressConfig struct {
	Mnemonic string `json:"mnemonic"`
	Pass     string `json:"pass"`
	Path     string `json:"path"`
}

func New(logger i_logger.ILogger) *PrintEthAddressType {
	t := &PrintEthAddressType{
		logger: logger,
	}
	return t
}

func (p *PrintEthAddressType) Start(ctx context.Context, task *constant.Task) (any, error) {
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

func (p *PrintEthAddressType) do(task *constant.Task) (interface{}, error) {
	var config PrintEthAddressConfig
	err := go_format.MapToStruct(&config, task.Data)
	if err != nil {
		return "", err
	}

	wallet := go_coin_eth.NewWallet(p.logger)
	seed := wallet.SeedHexByMnemonic(config.Mnemonic, config.Pass)

	results := make([]map[string]interface{}, 0)

	arr := strings.Split(config.Path, "-")
	if len(arr) <= 1 {
		result, err := wallet.DeriveFromPath(seed, config.Path)
		if err != nil {
			return "", err
		}
		cryptedPriv, err := go_crypto.AesCbcEncrypt(config.Pass, result.PrivateKey)
		if err != nil {
			return "", err
		}
		results = append(results, map[string]interface{}{
			"address":     result.Address,
			"priv":        result.PrivateKey,
			"cryptedPriv": cryptedPriv,
		})
	} else {
		lastPos := strings.LastIndex(arr[0], "/")
		startIndex := go_format.MustToInt(arr[0][lastPos+1:])
		endIndex := go_format.MustToInt(arr[1])

		pathPrefix := arr[0][:lastPos+1]

		for i := startIndex; i < endIndex; i++ {
			path := fmt.Sprintf("%s%d", pathPrefix, i)
			result, err := wallet.DeriveFromPath(seed, path)
			if err != nil {
				return "", err
			}
			cryptedPriv, err := go_crypto.AesCbcEncrypt(config.Pass, result.PrivateKey)
			if err != nil {
				return "", err
			}
			results = append(results, map[string]interface{}{
				"address":     result.Address,
				"priv":        result.PrivateKey,
				"cryptedPriv": cryptedPriv,
			})
		}

	}

	return results, nil
}
