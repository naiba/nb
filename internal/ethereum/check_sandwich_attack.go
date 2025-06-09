package ethereum

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	transferEventHash = common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")
)

type txContext struct {
	txHash  common.Hash
	txIndex int
	amounts map[string]*big.Int
}

func CheckSandwichAttack(ctx context.Context, rpcUrl string, txHash string, userAddress string, tokenAddress string, maxCheckTxCount int) error {
	userAddressParsed := common.HexToAddress(userAddress)
	tokenAddressParsed := common.HexToAddress(tokenAddress)

	ethClient, err := ethclient.Dial(rpcUrl)
	if err != nil {
		return err
	}

	tx, _, err := ethClient.TransactionByHash(ctx, common.HexToHash(txHash))
	if err != nil {
		return err
	}
	userTxCtx, userTxReceipt, err := analysisTx(ctx, ethClient, tx, tokenAddressParsed, false)
	if err != nil {
		return err
	}
	if userTxCtx == nil {
		return fmt.Errorf("transaction %s cannot be analyzed", txHash)
	}
	if _, has := userTxCtx.amounts[userAddressParsed.Hex()]; !has {
		return fmt.Errorf("transaction %s does not involve user address %s", txHash, userAddress)
	}

	block, err := ethClient.BlockByNumber(ctx, userTxReceipt.BlockNumber)
	if err != nil {
		return err
	}

	var relatedTxs []*txContext
	txsInBlock := block.Transactions()

	for i := int(userTxReceipt.TransactionIndex - 1); i >= 0; i-- {
		tx := txsInBlock[i]
		log.Printf("Checking buying tx: %d/%d %s", i, len(txsInBlock), tx.Hash().Hex())
		txCtx, _, err := analysisTx(ctx, ethClient, tx, tokenAddressParsed, false)
		if err != nil {
			return err
		}
		if ctx != nil && len(txCtx.amounts) > 0 {
			relatedTxs = append(relatedTxs, txCtx)
		}
		if int(userTxReceipt.TransactionIndex)-i > maxCheckTxCount {
			break
		}
	}

	relatedTxs = append(relatedTxs, userTxCtx)

	for i := int(userTxReceipt.TransactionIndex + 1); i < len(txsInBlock); i++ {
		tx := txsInBlock[i]
		log.Printf("Checking selling tx: %d/%d %s", i, len(txsInBlock), tx.Hash().Hex())
		txCtx, _, err := analysisTx(ctx, ethClient, tx, tokenAddressParsed, true)
		if err != nil {
			return err
		}
		if txCtx != nil {
			relatedTxs = append(relatedTxs, txCtx)
		}
		if checkSellTx(relatedTxs, txCtx) {
			break
		}
		if i-int(userTxReceipt.TransactionIndex) > maxCheckTxCount {
			break
		}
	}

	log.Println(">>>>>>>>> Related transactions <<<<<<<<<")
	for _, relatedTx := range relatedTxs {
		var desc string
		if relatedTx.txIndex == int(userTxReceipt.TransactionIndex) {
			desc = "(user)"
		}
		log.Printf("idx: %d%s, %s tx: %s", relatedTx.txIndex, desc, formatTxCtx(relatedTx), relatedTx.txHash)
	}

	return nil
}

func formatTxCtx(txCtx *txContext) string {
	if txCtx == nil {
		return "nil"
	}
	if len(txCtx.amounts) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteString("transfers: [")
	for addr, amt := range txCtx.amounts {
		sb.WriteString(fmt.Sprintf("%s: %s, ", addr, amt.String()))
	}
	sb.WriteString("]")
	return sb.String()
}

func analysisTx(ctx context.Context, ethClient *ethclient.Client, tx *types.Transaction, tokenAddress common.Address, reverseFromTo bool) (*txContext, *types.Receipt, error) {
	ret, err := ethClient.TransactionReceipt(ctx, tx.Hash())
	if err != nil {
		return nil, nil, err
	}
	if ret.Status == types.ReceiptStatusFailed {
		return nil, nil, nil
	}
	txCtx := txContext{txHash: tx.Hash(), txIndex: int(ret.TransactionIndex), amounts: make(map[string]*big.Int)}
	for j := 0; j < len(ret.Logs); j++ {
		amount, fromAddr, toAddr, err := parseTransferAmount(ret.Logs[j], tokenAddress)
		if err != nil {
			continue
		}
		if reverseFromTo {
			fromAddr, toAddr = toAddr, fromAddr
		}
		if fromAmtOld, has := txCtx.amounts[fromAddr.Hex()]; has {
			txCtx.amounts[fromAddr.Hex()] = subOrZero(fromAmtOld, amount)
		}
		if toAmtOld, has := txCtx.amounts[toAddr.Hex()]; has {
			txCtx.amounts[toAddr.Hex()] = new(big.Int).Add(toAmtOld, amount)
		} else {
			txCtx.amounts[toAddr.Hex()] = amount
		}
	}
	for addr, amt := range txCtx.amounts {
		if amt.Cmp(big.NewInt(0)) == 0 {
			delete(txCtx.amounts, addr)
		}
	}
	return &txCtx, ret, nil
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

func subOrZero(a, b *big.Int) *big.Int {
	if a.Cmp(b) < 0 {
		return big.NewInt(0)
	}
	return new(big.Int).Sub(a, b)
}

func checkSellTx(relatedTxs []*txContext, txCtx *txContext) bool {
	if txCtx == nil {
		return false
	}
	for _, relatedTx := range relatedTxs {
		for addr := range txCtx.amounts {
			if _, has := relatedTx.amounts[addr]; has {
				return true
			}
		}
	}
	return false
}
