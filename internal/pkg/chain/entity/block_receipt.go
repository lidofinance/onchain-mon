package entity

import "strconv"

type BlockReceipt struct {
	BlockHash         string `json:"blockHash"`
	BlockNumber       string `json:"blockNumber"`
	ContractAddress   string `json:"contractAddress,omitempty"`
	CumulativeGasUsed string `json:"cumulativeGasUsed"`
	EffectiveGasPrice string `json:"effectiveGasPrice"`
	From              string `json:"from"`
	GasUsed           string `json:"gasUsed"`
	Logs              []Log  `json:"logs"`
	LogsBloom         string `json:"logsBloom"`
	Status            string `json:"status"`
	To                string `json:"to"`
	TransactionHash   string `json:"transactionHash"`
	TransactionIndex  string `json:"transactionIndex"`
	Type              string `json:"type"`
	BlobGasPrice      string `json:"blobGasPrice,omitempty"`
	BlobGasUsed       string `json:"blobGasUsed,omitempty"`
}

func (e *BlockReceipt) GetNumber() uint64 {
	value, _ := strconv.ParseUint(e.BlockNumber, 0, 64)
	return value
}
