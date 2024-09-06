package entity

import (
	"strconv"
)

type EthBlock struct {
	BaseFeePerGas         string   `json:"baseFeePerGas"`
	BlobGasUsed           string   `json:"blobGasUsed"`
	Difficulty            string   `json:"difficulty"`
	ExcessBlobGas         string   `json:"excessBlobGas"`
	ExtraData             string   `json:"extraData"`
	GasLimit              string   `json:"gasLimit"`
	GasUsed               string   `json:"gasUsed"`
	Hash                  string   `json:"hash"`
	LogsBloom             string   `json:"logsBloom"`
	Miner                 string   `json:"miner"`
	MixHash               string   `json:"mixHash"`
	Nonce                 string   `json:"nonce"`
	Number                string   `json:"number"`
	ParentBeaconBlockRoot string   `json:"parentBeaconBlockRoot"`
	ParentHash            string   `json:"parentHash"`
	ReceiptsRoot          string   `json:"receiptsRoot"`
	Sha3Uncles            string   `json:"sha3Uncles"`
	Size                  string   `json:"size"`
	StateRoot             string   `json:"stateRoot"`
	Timestamp             string   `json:"timestamp"`
	TotalDifficulty       string   `json:"totalDifficulty"`
	Transactions          []string `json:"transactions"`
}

func (e *EthBlock) GetNumber() int64 {
	value, _ := strconv.ParseInt(e.Number, 0, 64)
	return value
}

func (e *EthBlock) GetTimestamp() int64 {
	value, _ := strconv.ParseInt(e.Timestamp, 0, 32)
	return value
}
