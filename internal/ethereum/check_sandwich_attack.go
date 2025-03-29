package ethereum

import (
	"context"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	transferEventHash = common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")
)

type txContext struct {
	txHash   common.Hash
	sender   *common.Address
	receiver *common.Address
	amount   *big.Int
	txIndex  int
}

func CheckSandwichAttack(ctx context.Context, rpcUrl string, txHash string, userAddress string, tokenAddress string, maxCheckTxCount int) error {
	userAddressParsed := common.HexToAddress(userAddress)
	ethClient, err := ethclient.Dial(rpcUrl)
	if err != nil {
		return err
	}

	userTxReceipt, err := ethClient.TransactionReceipt(ctx, common.HexToHash(txHash))
	if err != nil {
		return err
	}

	var userAmount *big.Int
	var userTokenSender *common.Address
	for i := len(userTxReceipt.Logs) - 1; i >= 0; i-- {
		amount, sender, receipt, err := parseTransferAmount(userTxReceipt.Logs[i], common.HexToAddress(tokenAddress))
		if err != nil {
			continue
		}
		if receipt.Cmp(userAddressParsed) == 0 {
			userTokenSender = sender
			userAmount = amount
			break
		}
	}
	if userAmount == nil {
		return fmt.Errorf("user amount not found")
	}

	block, err := ethClient.BlockByNumber(ctx, userTxReceipt.BlockNumber)
	if err != nil {
		return err
	}

	var relatedTxs []txContext
	txsInBlock := block.Transactions()

	for i := int(userTxReceipt.TransactionIndex - 1); i >= 0; i-- {
		tx := txsInBlock[i]
		log.Printf("Checking buying tx: %d/%d %s", i, len(txsInBlock), tx.Hash().Hex())
		ret, err := ethClient.TransactionReceipt(ctx, tx.Hash())
		if err != nil {
			return err
		}
		if ret.Status == types.ReceiptStatusFailed {
			continue
		}
		for j := len(ret.Logs) - 1; j >= 0; j-- {
			amount, sender, receiver, err := parseTransferAmount(ret.Logs[j], common.HexToAddress(tokenAddress))
			if err != nil {
				continue
			}
			relatedTxs = append(relatedTxs, txContext{txHash: tx.Hash(), sender: sender, receiver: receiver, amount: amount, txIndex: i})
		}
		if int(userTxReceipt.TransactionIndex)-i > maxCheckTxCount {
			break
		}
	}

	relatedTxs = append(relatedTxs, txContext{txHash: userTxReceipt.TxHash, sender: userTokenSender, receiver: &userAddressParsed, amount: userAmount, txIndex: int(userTxReceipt.TransactionIndex)})

OUTER:
	for i := int(userTxReceipt.TransactionIndex + 1); i < len(txsInBlock); i++ {
		tx := txsInBlock[i]
		log.Printf("Checking selling tx: %d/%d %s", i, len(txsInBlock), tx.Hash().Hex())
		ret, err := ethClient.TransactionReceipt(ctx, tx.Hash())
		if err != nil {
			return err
		}
		if ret.Status == types.ReceiptStatusFailed {
			continue
		}
		for j := len(ret.Logs) - 1; j >= 0; j-- {
			amount, sender, receiver, err := parseTransferAmount(ret.Logs[j], common.HexToAddress(tokenAddress))
			if err != nil {
				continue
			}
			for k := 0; k < len(relatedTxs); k++ {
				if equalBnInPercent(amount, relatedTxs[k].amount, 100) && relatedTxs[k].receiver.Cmp(*sender) == 0 {
					relatedTxs = append(relatedTxs, txContext{txHash: tx.Hash(), sender: sender, receiver: receiver, amount: amount, txIndex: int(ret.TransactionIndex)})
					break OUTER
				}
			}
		}
		if i-int(userTxReceipt.TransactionIndex) > maxCheckTxCount {
			break
		}
	}

	log.Println(">>>>>>>>> Related transactions <<<<<<<<<")
	for _, relatedTx := range relatedTxs {
		var desc string
		var addr common.Address
		if relatedTx.txIndex == int(userTxReceipt.TransactionIndex) {
			desc = "(user)"
			addr = *relatedTx.receiver
		} else if relatedTx.txIndex > int(userTxReceipt.TransactionIndex) {
			addr = *relatedTx.sender
		} else {
			addr = *relatedTx.receiver
		}
		log.Printf("idx: %d%s, amount: %v, related-address: %s, tx: %s", relatedTx.txIndex, desc, relatedTx.amount, addr, relatedTx.txHash)
	}

	return nil
}

func equalBnInPercent(bn1 *big.Int, bn2 *big.Int, percent int) bool {
	percentBn := big.NewInt(int64(percent))
	percent1 := new(big.Int).Div(bn1, percentBn)
	percent2 := new(big.Int).Div(bn2, percentBn)
	percentMin := percent1
	if percent1.Cmp(percent2) < 0 {
		percentMin = percent2
	}
	return new(big.Int).Abs(new(big.Int).Sub(bn1, bn2)).Cmp(percentMin) <= 0
}

func parseTransferAmount(log *types.Log, token common.Address) (*big.Int, *common.Address, *common.Address, error) {
	if len(log.Topics) < 3 {
		return nil, nil, nil, fmt.Errorf("invalid log")
	}
	if log.Address != token {
		return nil, nil, nil, fmt.Errorf("invalid token")
	}
	if log.Topics[0] != transferEventHash {
		return nil, nil, nil, fmt.Errorf("invalid event")
	}
	amount := new(big.Int).SetBytes(log.Data)
	senderAddress := common.HexToAddress(log.Topics[1].Hex())
	receiverAddress := common.HexToAddress(log.Topics[2].Hex())
	return amount, &senderAddress, &receiverAddress, nil
}
