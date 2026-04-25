package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolEthereumQuery = "builtin_ethereum_query"

var ethNetworks = map[string]string{
	"mainnet": "https://eth.llamarpc.com",
	"sepolia": "https://rpc.sepolia.org",
	"holesky": "https://rpc.holesky.ethpandaops.io",
}

func ethRPCURL(network string) string {
	if url, ok := ethNetworks[network]; ok {
		return url
	}
	return ethNetworks["mainnet"]
}

func ethCallRPC(rpcURL, method string, params []any) (json.RawMessage, error) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal RPC payload: %w", err)
	}

	resp, err := http.Post(rpcURL, "application/json", strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("RPC request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read RPC response: %w", err)
	}

	var result struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse RPC response: %w", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", result.Error.Code, result.Error.Message)
	}

	return result.Result, nil
}

func hexToBigInt(hex string) *big.Int {
	hex = strings.TrimPrefix(hex, "0x")
	if hex == "" {
		return big.NewInt(0)
	}
	n := new(big.Int)
	n.SetString(hex, 16)
	return n
}

func weiToEth(wei *big.Int) string {
	eth := new(big.Float).SetInt(wei)
	ethWei := new(big.Float).SetInt(big.NewInt(1e18))
	result := new(big.Float).Quo(eth, ethWei)
	return result.Text('f', 6)
}

func execBuiltinEthereumQuery(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action", "command")
	network := strArg(in, "network", "net")
	if network == "" {
		network = "mainnet"
	}
	rpcURL := ethRPCURL(network)

	switch op {
	case "block_number":
		result, err := ethCallRPC(rpcURL, "eth_blockNumber", nil)
		if err != nil {
			return "", err
		}
		var blockHex string
		if err := json.Unmarshal(result, &blockHex); err != nil {
			return "", fmt.Errorf("failed to parse block number: %w", err)
		}
		blockNum := hexToBigInt(blockHex)
		return fmt.Sprintf("Current block number on %s: %s", network, blockNum.String()), nil

	case "block":
		blockNum := strArg(in, "block_number", "block", "number")
		var blockParam string
		if blockNum == "" {
			blockParam = "latest"
		} else {
			blockParam = fmt.Sprintf("0x%x", blockNum)
		}
		result, err := ethCallRPC(rpcURL, "eth_getBlockByNumber", []any{blockParam, false})
		if err != nil {
			return "", err
		}
		var block map[string]any
		if err := json.Unmarshal(result, &block); err != nil {
			return "", fmt.Errorf("failed to parse block: %w", err)
		}
		number := block["number"].(string)
		hash := block["hash"].(string)
		timestamp := block["timestamp"].(string)
		txCount := len(block["transactions"].([]any))
		gasUsed := block["gasUsed"].(string)
		gasLimit := block["gasLimit"].(string)

		return fmt.Sprintf("Block %s on %s:\n  Hash: %s\n  Timestamp: %s\n  Transactions: %d\n  Gas Used: %s\n  Gas Limit: %s",
			hexToBigInt(number).String(), network, hash, hexToBigInt(timestamp).String(), txCount, hexToBigInt(gasUsed).String(), hexToBigInt(gasLimit).String()), nil

	case "balance":
		address := strArg(in, "address", "addr", "wallet")
		if address == "" {
			return "", fmt.Errorf("missing address parameter")
		}
		if !strings.HasPrefix(address, "0x") {
			address = "0x" + address
		}
		result, err := ethCallRPC(rpcURL, "eth_getBalance", []any{address, "latest"})
		if err != nil {
			return "", err
		}
		var balanceHex string
		if err := json.Unmarshal(result, &balanceHex); err != nil {
			return "", fmt.Errorf("failed to parse balance: %w", err)
		}
		balanceWei := hexToBigInt(balanceHex)
		balanceEth := weiToEth(balanceWei)
		return fmt.Sprintf("Address %s balance on %s:\n  Wei: %s\n  ETH: %s", address, network, balanceWei.String(), balanceEth), nil

	case "transaction":
		txHash := strArg(in, "tx_hash", "hash", "transaction")
		if txHash == "" {
			return "", fmt.Errorf("missing tx_hash parameter")
		}
		if !strings.HasPrefix(txHash, "0x") {
			txHash = "0x" + txHash
		}
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
		nonce := tx["nonce"].(string)

		toStr := "Contract Creation"
		if to != nil {
			toStr = to.(string)
		}

		var gasPriceStr string
		if gasPrice != nil {
			gasPriceStr = hexToBigInt(gasPrice.(string)).String()
		}

		return fmt.Sprintf("Transaction %s on %s:\n  From: %s\n  To: %s\n  Value: %s wei (%s ETH)\n  Gas: %s\n  Gas Price: %s wei\n  Nonce: %s",
			txHash, network, from, toStr, hexToBigInt(value).String(), weiToEth(hexToBigInt(value)), hexToBigInt(gas).String(), gasPriceStr, hexToBigInt(nonce).String()), nil

	case "gas_price":
		result, err := ethCallRPC(rpcURL, "eth_gasPrice", nil)
		if err != nil {
			return "", err
		}
		var gasPriceHex string
		if err := json.Unmarshal(result, &gasPriceHex); err != nil {
			return "", fmt.Errorf("failed to parse gas price: %w", err)
		}
		gasPriceWei := hexToBigInt(gasPriceHex)
		gwei := new(big.Float).SetInt(gasPriceWei)
		gweiDiv := new(big.Float).SetInt(big.NewInt(1e9))
		gweiResult := new(big.Float).Quo(gwei, gweiDiv)
		return fmt.Sprintf("Current gas price on %s:\n  Wei: %s\n  Gwei: %s", network, gasPriceWei.String(), gweiResult.Text('f', 2)), nil

	case "nonce":
		address := strArg(in, "address", "addr", "wallet")
		if address == "" {
			return "", fmt.Errorf("missing address parameter")
		}
		if !strings.HasPrefix(address, "0x") {
			address = "0x" + address
		}
		result, err := ethCallRPC(rpcURL, "eth_getTransactionCount", []any{address, "latest"})
		if err != nil {
			return "", err
		}
		var nonceHex string
		if err := json.Unmarshal(result, &nonceHex); err != nil {
			return "", fmt.Errorf("failed to parse nonce: %w", err)
		}
		nonce := hexToBigInt(nonceHex)
		return fmt.Sprintf("Address %s nonce on %s: %s", address, network, nonce.String()), nil

	case "chain_id":
		result, err := ethCallRPC(rpcURL, "eth_chainId", nil)
		if err != nil {
			return "", err
		}
		var chainIdHex string
		if err := json.Unmarshal(result, &chainIdHex); err != nil {
			return "", fmt.Errorf("failed to parse chain ID: %w", err)
		}
		chainId := hexToBigInt(chainIdHex)
		networkNames := map[string]string{
			"1":        "Ethereum Mainnet",
			"11155111": "Sepolia Testnet",
			"17000":    "Holesky Testnet",
		}
		name := networkNames[chainId.String()]
		if name == "" {
			name = "Unknown Network"
		}
		return fmt.Sprintf("Chain ID: %s (%s)", chainId.String(), name), nil

	default:
		return "", fmt.Errorf("unknown operation: %s (supported: block_number, block, balance, transaction, gas_price, nonce, chain_id)", op)
	}
}

func NewBuiltinEthereumQueryTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolEthereumQuery,
			Desc:  "Query Ethereum blockchain data: blocks, transactions, balances, gas prices, and network info",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":    {Type: einoschema.String, Desc: "Operation: block_number, block, balance, transaction, gas_price, nonce, chain_id", Required: true},
				"block_number": {Type: einoschema.String, Desc: "Block number for block query", Required: false},
				"address":      {Type: einoschema.String, Desc: "Ethereum address for balance/nonce queries", Required: false},
				"tx_hash":      {Type: einoschema.String, Desc: "Transaction hash for transaction query", Required: false},
				"network":      {Type: einoschema.String, Desc: "Ethereum network: mainnet, sepolia, holesky", Required: false},
			}),
		},
		execBuiltinEthereumQuery,
	)
}
