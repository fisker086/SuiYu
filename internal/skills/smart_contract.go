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

const toolSmartContract = "builtin_smart_contract"

var erc20ABI = map[string]string{
	"name":        "0x06fdde03",
	"symbol":      "0x95d89b41",
	"decimals":    "0x313ce567",
	"totalSupply": "0x18160ddd",
	"balanceOf":   "0x70a08231",
	"allowance":   "0xdd62ed3e",
}

func padAddress(addr string) string {
	addr = strings.TrimPrefix(addr, "0x")
	return strings.Repeat("0", 64-len(addr)) + addr
}

func execBuiltinSmartContract(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action", "command")
	network := strArg(in, "network", "net")
	if network == "" {
		network = "mainnet"
	}
	rpcURL := ethRPCURL(network)
	contractAddr := strArg(in, "contract_address", "contract", "addr")
	if contractAddr == "" {
		return "", fmt.Errorf("missing contract_address parameter")
	}
	if !strings.HasPrefix(contractAddr, "0x") {
		contractAddr = "0x" + contractAddr
	}

	switch op {
	case "erc20_info":
		results := make(map[string]string)
		for _, field := range []string{"name", "symbol", "decimals", "totalSupply"} {
			data := erc20ABI[field]
			result, err := ethCallRPC(rpcURL, "eth_call", []any{
				map[string]string{
					"to":   contractAddr,
					"data": data,
				},
				"latest",
			})
			if err != nil {
				return "", fmt.Errorf("failed to call %s: %w", field, err)
			}
			var hexResult string
			if err := json.Unmarshal(result, &hexResult); err != nil {
				return "", fmt.Errorf("failed to parse %s result: %w", field, err)
			}
			results[field] = hexResult
		}

		name := decodeStringResult(results["name"])
		symbol := decodeStringResult(results["symbol"])
		decimals := hexToBigInt(results["decimals"]).Int64()
		totalSupply := hexToBigInt(results["totalSupply"])

		if decimals > 0 {
			div := new(big.Int).Exp(big.NewInt(10), big.NewInt(decimals), nil)
			supplyInt := new(big.Int).Div(totalSupply, div)
			remainder := new(big.Int).Mod(totalSupply, div)
			totalSupplyStr := fmt.Sprintf("%s.%s", supplyInt.String(), fmt.Sprintf("%0*d", int(decimals), remainder.Int64()))
			return fmt.Sprintf("ERC20 Token Info:\n  Name: %s\n  Symbol: %s\n  Decimals: %d\n  Total Supply: %s", name, symbol, decimals, totalSupplyStr), nil
		}

		return fmt.Sprintf("ERC20 Token Info:\n  Name: %s\n  Symbol: %s\n  Decimals: %d\n  Total Supply: %s", name, symbol, decimals, totalSupply.String()), nil

	case "erc20_balance":
		address := strArg(in, "address", "addr", "wallet")
		if address == "" {
			return "", fmt.Errorf("missing address parameter for balance query")
		}
		if !strings.HasPrefix(address, "0x") {
			address = "0x" + address
		}

		data := erc20ABI["balanceOf"] + padAddress(address)
		result, err := ethCallRPC(rpcURL, "eth_call", []any{
			map[string]string{
				"to":   contractAddr,
				"data": data,
			},
			"latest",
		})
		if err != nil {
			return "", fmt.Errorf("failed to call balanceOf: %w", err)
		}
		var balanceHex string
		if err := json.Unmarshal(result, &balanceHex); err != nil {
			return "", fmt.Errorf("failed to parse balance: %w", err)
		}

		decimalsResult, _ := ethCallRPC(rpcURL, "eth_call", []any{
			map[string]string{"to": contractAddr, "data": erc20ABI["decimals"]},
			"latest",
		})
		var decHex string
		json.Unmarshal(decimalsResult, &decHex)
		decimals := hexToBigInt(decHex).Int64()

		balance := hexToBigInt(balanceHex)
		if decimals > 0 && balance.Cmp(big.NewInt(0)) > 0 {
			div := new(big.Int).Exp(big.NewInt(10), big.NewInt(decimals), nil)
			balanceInt := new(big.Int).Div(balance, div)
			remainder := new(big.Int).Mod(balance, div)
			balanceStr := fmt.Sprintf("%s.%s", balanceInt.String(), fmt.Sprintf("%0*d", int(decimals), remainder.Int64()))
			return fmt.Sprintf("Token Balance:\n  Contract: %s\n  Address: %s\n  Balance: %s (raw: %s)", contractAddr, address, balanceStr, balance.String()), nil
		}

		return fmt.Sprintf("Token Balance:\n  Contract: %s\n  Address: %s\n  Balance: %s", contractAddr, address, balance.String()), nil

	case "erc20_allowance":
		address := strArg(in, "address", "addr", "owner", "wallet")
		spender := strArg(in, "spender", "spender_address")
		if address == "" || spender == "" {
			return "", fmt.Errorf("missing address or spender parameter for allowance query")
		}
		if !strings.HasPrefix(address, "0x") {
			address = "0x" + address
		}
		if !strings.HasPrefix(spender, "0x") {
			spender = "0x" + spender
		}

		data := erc20ABI["allowance"] + padAddress(address) + padAddress(spender)
		result, err := ethCallRPC(rpcURL, "eth_call", []any{
			map[string]string{
				"to":   contractAddr,
				"data": data,
			},
			"latest",
		})
		if err != nil {
			return "", fmt.Errorf("failed to call allowance: %w", err)
		}
		var allowanceHex string
		if err := json.Unmarshal(result, &allowanceHex); err != nil {
			return "", fmt.Errorf("failed to parse allowance: %w", err)
		}
		allowance := hexToBigInt(allowanceHex)

		return fmt.Sprintf("ERC20 Allowance:\n  Contract: %s\n  Owner: %s\n  Spender: %s\n  Allowance: %s", contractAddr, address, spender, allowance.String()), nil

	case "erc721_owner":
		tokenId := strArg(in, "token_id", "tokenId", "id")
		if tokenId == "" {
			return "", fmt.Errorf("missing token_id parameter")
		}
		tokenIdBig, ok := new(big.Int).SetString(tokenId, 10)
		if !ok {
			tokenIdBig, _ = new(big.Int).SetString(tokenId, 16)
		}
		tokenIdHex := fmt.Sprintf("0x%s", fmt.Sprintf("%064s", fmt.Sprintf("%x", tokenIdBig)))

		data := "0x6352211e" + tokenIdHex[2:]
		result, err := ethCallRPC(rpcURL, "eth_call", []any{
			map[string]string{
				"to":   contractAddr,
				"data": data,
			},
			"latest",
		})
		if err != nil {
			return "", fmt.Errorf("failed to call ownerOf: %w", err)
		}
		var ownerHex string
		if err := json.Unmarshal(result, &ownerHex); err != nil {
			return "", fmt.Errorf("failed to parse owner: %w", err)
		}
		owner := "0x" + ownerHex[26:]

		return fmt.Sprintf("NFT Owner:\n  Contract: %s\n  Token ID: %s\n  Owner: %s", contractAddr, tokenIdBig.String(), owner), nil

	case "erc721_metadata":
		tokenId := strArg(in, "token_id", "tokenId", "id")
		if tokenId == "" {
			return "", fmt.Errorf("missing token_id parameter")
		}
		tokenIdBig, ok := new(big.Int).SetString(tokenId, 10)
		if !ok {
			tokenIdBig, _ = new(big.Int).SetString(tokenId, 16)
		}
		tokenIdHex := fmt.Sprintf("0x%s", fmt.Sprintf("%064s", fmt.Sprintf("%x", tokenIdBig)))

		data := "0xc87b56dd" + tokenIdHex[2:]
		result, err := ethCallRPC(rpcURL, "eth_call", []any{
			map[string]string{
				"to":   contractAddr,
				"data": data,
			},
			"latest",
		})
		if err != nil {
			return "", fmt.Errorf("failed to call tokenURI: %w", err)
		}
		var uriHex string
		if err := json.Unmarshal(result, &uriHex); err != nil {
			return "", fmt.Errorf("failed to parse tokenURI: %w", err)
		}
		uri := decodeStringResult(uriHex)

		return fmt.Sprintf("NFT Metadata:\n  Contract: %s\n  Token ID: %s\n  Token URI: %s", contractAddr, tokenIdBig.String(), uri), nil

	case "code":
		result, err := ethCallRPC(rpcURL, "eth_getCode", []any{contractAddr, "latest"})
		if err != nil {
			return "", fmt.Errorf("failed to get contract code: %w", err)
		}
		var codeHex string
		if err := json.Unmarshal(result, &codeHex); err != nil {
			return "", fmt.Errorf("failed to parse code: %w", err)
		}
		codeLen := (len(codeHex) - 2) / 2
		if codeLen == 0 {
			return fmt.Sprintf("Contract %s has no code (not a contract address)", contractAddr), nil
		}
		return fmt.Sprintf("Contract Code:\n  Address: %s\n  Size: %d bytes\n  Code: %s...", contractAddr, codeLen, codeHex[:100]), nil

	case "storage":
		slot := strArg(in, "slot", "storage_slot", "position")
		if slot == "" {
			return "", fmt.Errorf("missing slot parameter for storage query")
		}
		slotBig, ok := new(big.Int).SetString(slot, 10)
		if !ok {
			slotBig, _ = new(big.Int).SetString(slot, 16)
		}
		slotHex := fmt.Sprintf("0x%s", fmt.Sprintf("%064s", fmt.Sprintf("%x", slotBig)))

		result, err := ethCallRPC(rpcURL, "eth_getStorageAt", []any{contractAddr, slotHex, "latest"})
		if err != nil {
			return "", fmt.Errorf("failed to get storage: %w", err)
		}
		var valueHex string
		if err := json.Unmarshal(result, &valueHex); err != nil {
			return "", fmt.Errorf("failed to parse storage value: %w", err)
		}
		value := hexToBigInt(valueHex)

		return fmt.Sprintf("Contract Storage:\n  Contract: %s\n  Slot: %s\n  Value: %s (hex: %s)", contractAddr, slotBig.String(), value.String(), valueHex), nil

	default:
		return "", fmt.Errorf("unknown operation: %s (supported: erc20_info, erc20_balance, erc20_allowance, erc721_owner, erc721_metadata, code, storage)", op)
	}
}

func decodeStringResult(hex string) string {
	if len(hex) < 130 {
		return hex
	}
	data := hex[130:]
	bytes := make([]byte, len(data)/2)
	for i := 0; i < len(data); i += 2 {
		fmt.Sscanf(data[i:i+2], "%02x", &bytes[i/2])
	}
	length := 0
	for _, b := range bytes {
		if b == 0 {
			break
		}
		length++
	}
	return string(bytes[:length])
}

func NewBuiltinSmartContractTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolSmartContract,
			Desc:  "Read smart contract state: ERC20/ERC721 info, balances, allowances, contract code, and storage",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":        {Type: einoschema.String, Desc: "Operation: erc20_info, erc20_balance, erc20_allowance, erc721_owner, erc721_metadata, code, storage", Required: true},
				"contract_address": {Type: einoschema.String, Desc: "Ethereum contract address", Required: true},
				"address":          {Type: einoschema.String, Desc: "Wallet address for balance/allowance queries", Required: false},
				"spender":          {Type: einoschema.String, Desc: "Spender address for allowance query", Required: false},
				"token_id":         {Type: einoschema.String, Desc: "NFT token ID for ERC721 queries", Required: false},
				"slot":             {Type: einoschema.String, Desc: "Storage slot for storage query", Required: false},
				"network":          {Type: einoschema.String, Desc: "Ethereum network: mainnet, sepolia, holesky", Required: false},
			}),
		},
		execBuiltinSmartContract,
	)
}
