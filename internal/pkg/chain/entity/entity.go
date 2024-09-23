package entity

type RpcRequest struct {
	JsonRpc string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	ID      string `json:"id"`
}

type RpcResponse[T any] struct {
	JsonRpc string    `json:"jsonrpc"`
	ID      string    `json:"id"`
	Result  *T        `json:"result,omitempty"`
	Error   *RpcError `json:"error,omitempty"`
}

type RpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
