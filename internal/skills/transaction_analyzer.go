package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolTransactionAnalyzer = "builtin_transaction_analyzer"

var erc20TransferTopic = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"

func execBuiltinTransactionAnalyzer(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action", "command")
	network := strArg(in, "network", "net")
	if network == "" {
		network = "mainnet"
	}
	rpcURL := ethRPCURL(network)
	txHash := strArg(in, "tx_hash", "hash", "transaction")
	if txHash == "" {
		return "", fmt.Errorf("missing tx_hash parameter")
	}
	if !strings.HasPrefix(txHash, "0x") {
		txHash = "0x" + txHash
	}

	switch op {
	case "details":
		result, err := ethCallRPC(rpcURL, "eth_getTransactionByHash", []any{txHash})
		if err != nil {
			return "", err
		}
		if string(result) == "null" {
			return fmt.Sprintf("Transaction %s not found on %s", txHash, network), nil
		}
		var tx map[string]any
		if err := json.Unmarshal(result, &tx); err != nil {
			return "", fmt.Errorf("failed to parse transaction: %w", err)
		}
		from := tx["from"].(string)
		to := tx["to"]
		value := tx["value"].(string)
		gas := tx["gas"].(string)
		gasPrice := tx["gasPrice"]
		input := tx["input"].(string)
		nonce := tx["nonce"].(string)
		blockNumber := tx["blockNumber"]

		toStr := "Contract Creation"
		if to != nil {
			toStr = to.(string)
		}

		var gasPriceStr string
		if gasPrice != nil {
			gasPriceStr = hexToBigInt(gasPrice.(string)).String()
		}

		blockStr := "Pending"
		if blockNumber != nil {
			blockStr = hexToBigInt(blockNumber.(string)).String()
		}

		inputPreview := input
		if len(inputPreview) > 66 {
			inputPreview = inputPreview[:66] + "..."
		}

		return fmt.Sprintf("Transaction Details (%s):\n  Hash: %s\n  Block: %s\n  From: %s\n  To: %s\n  Value: %s wei (%s ETH)\n  Gas Limit: %s\n  Gas Price: %s wei\n  Nonce: %s\n  Input Data: %s",
			network, txHash, blockStr, from, toStr, hexToBigInt(value).String(), weiToEth(hexToBigInt(value)), hexToBigInt(gas).String(), gasPriceStr, hexToBigInt(nonce).String(), inputPreview), nil

	case "receipt":
		result, err := ethCallRPC(rpcURL, "eth_getTransactionReceipt", []any{txHash})
		if err != nil {
			return "", err
		}
		if string(result) == "null" {
			return fmt.Sprintf("Transaction receipt not found for %s on %s (may be pending)", txHash, network), nil
		}
		var receipt map[string]any
		if err := json.Unmarshal(result, &receipt); err != nil {
			return "", fmt.Errorf("failed to parse receipt: %w", err)
		}
		status := receipt["status"].(string)
		blockNumber := receipt["blockNumber"].(string)
		gasUsed := receipt["gasUsed"].(string)
		cumulativeGasUsed := receipt["cumulativeGasUsed"].(string)
		effectiveGasPrice := receipt["effectiveGasPrice"]
		contractAddress := receipt["contractAddress"]
		logCount := len(receipt["logs"].([]any))

		statusStr := "Success"
		if status == "0x0" {
			statusStr = "Failed"
		}

		var gasPriceStr string
		if effectiveGasPrice != nil {
			gasPriceStr = hexToBigInt(effectiveGasPrice.(string)).String()
		}

		contractStr := "N/A"
		if contractAddress != nil && contractAddress.(string) != "" {
			contractStr = contractAddress.(string)
		}

		return fmt.Sprintf("Transaction Receipt (%s):\n  Hash: %s\n  Status: %s\n  Block: %s\n  Gas Used: %s\n  Cumulative Gas Used: %s\n  Effective Gas Price: %s wei\n  Contract Address: %s\n  Log Entries: %d",
			network, txHash, statusStr, hexToBigInt(blockNumber).String(), hexToBigInt(gasUsed).String(), hexToBigInt(cumulativeGasUsed).String(), gasPriceStr, contractStr, logCount), nil

	case "gas_analysis":
		txResult, err := ethCallRPC(rpcURL, "eth_getTransactionByHash", []any{txHash})
		if err != nil {
			return "", err
		}
		if string(txResult) == "null" {
			return fmt.Sprintf("Transaction %s not found on %s", txHash, network), nil
		}
		var tx map[string]any
		if err := json.Unmarshal(txResult, &tx); err != nil {
			return "", fmt.Errorf("failed to parse transaction: %w", err)
		}

		receiptResult, err := ethCallRPC(rpcURL, "eth_getTransactionReceipt", []any{txHash})
		if err != nil {
			return "", err
		}
		if string(receiptResult) == "null" {
			return fmt.Sprintf("Transaction receipt not found for %s", txHash), nil
		}
		var receipt map[string]any
		if err := json.Unmarshal(receiptResult, &receipt); err != nil {
			return "", fmt.Errorf("failed to parse receipt: %w", err)
		}

		gasLimit := hexToBigInt(tx["gas"].(string))
		gasUsed := hexToBigInt(receipt["gasUsed"].(string))
		effectiveGasPrice := hexToBigInt(receipt["effectiveGasPrice"].(string))

		gasUsedPercent := new(big.Float).Quo(new(big.Float).SetInt(gasUsed), new(big.Float).SetInt(gasLimit))
		gasUsedPercent.Mul(gasUsedPercent, big.NewFloat(100))

		totalCost := new(big.Int).Mul(gasUsed, effectiveGasPrice)

		return fmt.Sprintf("Gas Analysis (%s):\n  Hash: %s\n  Gas Limit: %s\n  Gas Used: %s\n  Gas Used: %s%%\n  Effective Gas Price: %s wei (%s Gwei)\n  Total Cost: %s wei (%s ETH)",
			network, txHash, gasLimit.String(), gasUsed.String(), gasUsedPercent.Text('f', 2), effectiveGasPrice.String(), new(big.Float).Quo(new(big.Float).SetInt(effectiveGasPrice), big.NewFloat(1e9)).Text('f', 2), totalCost.String(), weiToEth(totalCost)), nil

	case "cost":
		receiptResult, err := ethCallRPC(rpcURL, "eth_getTransactionReceipt", []any{txHash})
		if err != nil {
			return "", err
		}
		if string(receiptResult) == "null" {
			return fmt.Sprintf("Transaction receipt not found for %s", txHash), nil
		}
		var receipt map[string]any
		if err := json.Unmarshal(receiptResult, &receipt); err != nil {
			return "", fmt.Errorf("failed to parse receipt: %w", err)
		}

		gasUsed := hexToBigInt(receipt["gasUsed"].(string))
		effectiveGasPrice := hexToBigInt(receipt["effectiveGasPrice"].(string))
		totalCost := new(big.Int).Mul(gasUsed, effectiveGasPrice)

		return fmt.Sprintf("Transaction Cost (%s):\n  Hash: %s\n  Gas Used: %s\n  Gas Price: %s wei (%s Gwei)\n  Total Cost: %s wei\n  Total Cost: %s ETH",
			network, txHash, gasUsed.String(), effectiveGasPrice.String(), new(big.Float).Quo(new(big.Float).SetInt(effectiveGasPrice), big.NewFloat(1e9)).Text('f', 4), totalCost.String(), weiToEth(totalCost)), nil

	case "token_transfers":
		result, err := ethCallRPC(rpcURL, "eth_getTransactionReceipt", []any{txHash})
		if err != nil {
			return "", err
		}
		if string(result) == "null" {
			return fmt.Sprintf("Transaction receipt not found for %s", txHash), nil
		}
		var receipt map[string]any
		if err := json.Unmarshal(result, &receipt); err != nil {
			return "", fmt.Errorf("failed to parse receipt: %w", err)
		}

		logs := receipt["logs"].([]any)
		var transfers []string
		for i, log := range logs {
			logMap := log.(map[string]any)
			topics := logMap["topics"].([]any)
			if len(topics) < 3 {
				continue
			}
			if topics[0].(string) != erc20TransferTopic {
				continue
			}

			contract := logMap["address"].(string)
			from := "0x" + topics[1].(string)[26:]
			to := "0x" + topics[2].(string)[26:]
			data := logMap["data"].(string)
			value := hexToBigInt(data)

			transfers = append(transfers, fmt.Sprintf("  Transfer #%d:\n    Contract: %s\n    From: %s\n    To: %s\n    Value: %s", i+1, contract, from, to, value.String()))
		}

		if len(transfers) == 0 {
			return fmt.Sprintf("No ERC20 token transfers found in transaction %s", txHash), nil
		}

		return fmt.Sprintf("Token Transfers (%s):\n  Hash: %s\n  Count: %d\n\n%s", network, txHash, len(transfers), strings.Join(transfers, "\n\n")), nil

	case "internal_txs":
		return fmt.Sprintf("Internal transaction analysis requires an archive node with tracing enabled.\nUse a service like Etherscan or an archive node RPC to get internal transactions for: %s", txHash), nil

	default:
		return "", fmt.Errorf("unknown operation: %s (supported: details, receipt, gas_analysis, cost, token_transfers, internal_txs)", op)
	}
}

func NewBuiltinTransactionAnalyzerTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolTransactionAnalyzer,
			Desc:  "Analyze Ethereum transactions: details, receipts, gas usage, costs, and token transfers",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation": {Type: einoschema.String, Desc: "Operation: details, receipt, gas_analysis, cost, token_transfers, internal_txs", Required: true},
				"tx_hash":   {Type: einoschema.String, Desc: "Transaction hash to analyze", Required: true},
				"network":   {Type: einoschema.String, Desc: "Ethereum network: mainnet, sepolia, holesky", Required: false},
			}),
		},
		execBuiltinTransactionAnalyzer,
	)
}
