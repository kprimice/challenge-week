package types

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Config is reused in scanner.go
type Config struct {
	MarketID   string
	StartBlock uint64
	EndBlock   uint64
}

// CSVRecord is a single row in the CSV output. Each parse function returns one or more CSVRecords.
type CSVRecord struct {
	TxHash         string
	Block          uint64
	BlockTimestamp string
	Action         string
	MarketID       string
	Price          string
	Quantity       string
	OrderType      string
	SubaccountID   string
	Margin         string
	ExecPrice      string
	ExecQuantity   string
	ExecFee        string
	OrderHash      string

	// Fields for derivative executions
	IsBuy         bool
	IsLiquidation bool
	Pnl           string
	Payout        string
}

func (r CSVRecord) AsOrderRow() []string {
	return []string{
		r.OrderHash,
		uint64ToStr(r.Block),
		r.Action,
		trimTrailingZeros(r.Price),
		trimTrailingZeros(r.Quantity),
		r.OrderType,
		r.SubaccountID,
		// trimTrailingZeros(r.Margin),
		// TxHash,
		// parseBlockTime(r.BlockTimestamp),
	}
}

func (r CSVRecord) AsTradeRow() []string {
	// ExecPrice, ExecQuantity, ExecFee, Pnl, Payout might have trailing zeros
	return []string{
		r.OrderHash,
		uint64ToStr(r.Block),
		r.Action,
		trimTrailingZeros(r.ExecPrice),
		trimTrailingZeros(r.ExecQuantity),
		trimTrailingZeros(r.ExecFee),
		boolToStr(r.IsBuy),
		boolToStr(r.IsLiquidation),
		trimTrailingZeros(r.Pnl),
		trimTrailingZeros(r.Payout),
		r.SubaccountID,
		// parseBlockTime(r.BlockTimestamp),
		// r.TxHash,
	}
}

func parseBlockTime(raw string) string {
	// The Explorer often returns times like: "2024-12-27 17:03:37.467 +0000 UTC"
	// which matches the Go layout: "2006-01-02 15:04:05.999999999 -0700 MST"

	layout := "2006-01-02 15:04:05.999999999 -0700 MST"
	t, err := time.Parse(layout, strings.TrimSpace(raw))
	if err != nil {
		// Fallback: just return raw string if parse fails
		return raw
	}

	// Return RFC 3339 format (ISO8601) with nanosecond precision in UTC.
	// e.g. "2024-12-27T17:03:37.467Z"
	return t.UTC().Format(time.RFC3339Nano)
}

func trimTrailingZeros(s string) string {
	if s == "" || !strings.Contains(s, ".") {
		return s
	}
	out := strings.TrimRight(s, "0")
	out = strings.TrimRight(out, ".")
	return out
}

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func uint64ToStr(val uint64) string {
	return fmt.Sprintf("%d", val)
}

// TxLog is the JSON format returned in transaction logs. Each TxData in explorerPB has logs in JSON.
type TxLog struct {
	MsgIndex int `json:"msg_index,string"`
	Events   []struct {
		Type       string           `json:"type"`
		Attributes []EventAttribute `json:"attributes"`
	} `json:"events"`
}

type EventAttribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Index bool   `json:"index"`
}

// AuthzMsgExecWrapper is used for decoding /cosmos.authz.v1beta1.MsgExec
type AuthzMsgExecWrapper struct {
	Type  string `json:"type"`
	AType string `json:"@type"`
	Value struct {
		Grantee   string            `json:"grantee"`
		Msgs      []json.RawMessage `json:"msgs"`
		NoSummary bool              `json:"noSummary"`
	} `json:"value"`
}

// MsgBatchUpdateOrders includes both create/cancel derivative orders in a single transaction message
type MsgBatchUpdateOrders struct {
	Type string `json:"@type"`

	DerivativeOrdersToCreate []struct {
		MarketID  string `json:"market_id"`
		OrderInfo struct {
			SubaccountID string `json:"subaccount_id"`
			Price        string `json:"price"`
			Quantity     string `json:"quantity"`
			FeeRecipient string `json:"fee_recipient"`
			Cid          string `json:"cid"`
		} `json:"order_info"`
		OrderType    string `json:"order_type"`
		Margin       string `json:"margin"`
		TriggerPrice string `json:"trigger_price"`
	} `json:"derivative_orders_to_create"`

	DerivativeOrdersToCancel []struct {
		MarketID     string `json:"market_id"`
		SubaccountID string `json:"subaccount_id"`
		OrderMask    uint32 `json:"order_mask"`
		Cid          string `json:"cid"`
		OrderHash    string `json:"order_hash,omitempty"`
	} `json:"derivative_orders_to_cancel"`
}

type BatchDerivativeTrade struct {
	SubaccountId string `json:"subaccount_id"`
	OrderHash    string `json:"order_hash"`
	Fee          string `json:"fee"`
	Payout       string `json:"payout"`
	Pnl          string `json:"pnl"`
	Cid          string `json:"cid"`
	FeeRecipient string `json:"fee_recipient_address"`

	PositionDelta struct {
		IsLong            bool   `json:"is_long"`
		ExecutionQuantity string `json:"execution_quantity"`
		ExecutionMargin   string `json:"execution_margin"`
		ExecutionPrice    string `json:"execution_price"`
	} `json:"position_delta"`
}

// MsgCreateDerivativeOrder captures a single “create derivative limit/market order”
type MsgCreateDerivativeOrder struct {
	Type      string `json:"@type"`
	MarketId  string `json:"market_id"`
	OrderType string `json:"order_type"`
	OrderInfo struct {
		SubaccountId string `json:"subaccount_id"`
		Price        string `json:"price"`
		Quantity     string `json:"quantity"`
		FeeRecipient string `json:"fee_recipient"`
		Cid          string `json:"cid"`
	} `json:"order_info"`
	Margin       string `json:"margin"`
	TriggerPrice string `json:"trigger_price"`
}

// LimitOrder is used in logs for new/cancelled orders
type LimitOrder struct {
	OrderInfo struct {
		SubaccountID string `json:"subaccount_id"`
		Price        string `json:"price"`
		Quantity     string `json:"quantity"`
		FeeRecipient string `json:"fee_recipient"`
		Cid          string `json:"cid"`
	} `json:"order_info"`
	OrderType    string `json:"order_type"`
	Margin       string `json:"margin"`
	MarketId     string `json:"market_id,omitempty"`
	TriggerPrice string `json:"trigger_price"`
	OrderHash    string `json:"order_hash"`
	Fillable     string `json:"fillable"`
}

// Execution is used in logs for fills/executions
type Execution struct {
	MarketId       string `json:"market_id,omitempty"`
	OrderHash      string `json:"order_hash,omitempty"`
	SubaccountId   string `json:"subaccount_id,omitempty"`
	QuantityFilled string `json:"quantity_filled,omitempty"`
	Price          string `json:"price,omitempty"`
	Fee            string `json:"fee,omitempty"`
}
