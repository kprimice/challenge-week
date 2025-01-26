# Injective On-Chain Scanner

A **Go-based toolkit** for fetching and parsing **derivative orders** and **trades** from the [Injective](https://injective.com/) blockchain via its Explorer API. This scanner retrieves transaction logs for a specified **block range** and **market** (optional), then exports them as **CSV files** for straightforward analysis, backtesting, or research.

---

## Features

- **Filter by Block Range**: Specify a start and end block to focus on any historical segment of Injective.
- **Optional Market Filter**: Provide a market ID to only capture orders/trades for that specific market. Otherwise, process **all** derivative markets in the given block range.
- **Retries & Pagination**: Includes built-in retry logic (`GetTxsWithRetry`) and paginated fetching to handle large queries and transient node issues.
- **Two CSV Outputs**:
  - **`data/orders.csv`**: Contains “new” and “cancel” events.
  - **`data/trades.csv`**: Contains execution (fill) events with fees, PnL, etc.

---

## Getting Started

### Prerequisites

- **Go 1.18+** or later.
- A stable internet connection for querying Injective’s Explorer node.
- (Optional) Docker if you want a containerized environment.

### Installation

Clone this repository and navigate into it:

```bash
git clone https://github.com/<your-org>/injective-scanner.git
cd injective-scanner
```

Build the binary (optional, you can also `go run`):

```bash
go build -o injective-scanner ./cmd/injective-scanner
```

### Usage

Run the scanner with default options:

```bash
go run ./cmd/injective-scanner/main.go
```

This will:

1. Fetch blocks **100,000,000 to 103,000,000** (default flags).
2. Focus on the **BTC-PERP** market at address `0x4ca0f92fc28be0c9761326016b5a1a2177dd6375558365116b5bdda9abc229ce`.
3. Write outputs to:
   - `./data/orders.csv`
   - `./data/trades.csv`

---

## Command-Line Flags

| Flag      | Type    | Default                                                     | Description                                                                                                       |
|-----------|---------|-------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------|
| `-start`  | uint64  | `100000000`                                                | **Starting block** to scan.                                                                                       |
| `-end`    | uint64  | `103000000`                                                | **Ending block** (inclusive) to scan.                                                                             |
| `-market` | string  | `0x4ca0f92fc28be0c9761326016b5a1a2177dd6375558365116b5bdda9abc229ce` | **Market ID** to filter. If empty, scanner retrieves **all** derivative orders for the block range. |

**Example**:  
```bash
go run ./cmd/injective-scanner/main.go \
  -start=120000000 \
  -end=120001000 \
  -market=0x4ca0f92fc28be0c9761326016b5a1a2177dd6375558365116b5bdda9abc229ce
```
This scans for blocks **120,000,000 through 120,001,000** on the specified market.

---

## Output CSVs

### 1. `data/orders.csv`

Columns:

| Column      | Description                                                   |
|-------------|---------------------------------------------------------------|
| `OrderHash` | Unique on-chain identifier of the limit/market order.         |
| `Block`     | Block height where event occurred.                            |
| `Action`    | `"EVENT_NEW"` or `"EVENT_CANCEL"`.                            |
| `Price`     | Limit order price (string from logs).                         |
| `Quantity`  | Order size (string from logs).                                |
| `OrderType` | E.g. `"BUY_PO"`, `"SELL_PO"`, `"MARKET"`.                     |
| `SubaccountID` | Anonymized trader ID.                                     |

### 2. `data/trades.csv`

Columns:

| Column         | Description                                                                       |
|----------------|-----------------------------------------------------------------------------------|
| `OrderHash`    | Unique order hash for the fill.                                                   |
| `Block`        | Block height where the execution occurred.                                        |
| `Action`       | Always `"EXECUTION"`.                                                             |
| `ExecPrice`    | Fill execution price.                                                             |
| `ExecQuantity` | Filled quantity.                                                                   |
| `ExecFee`      | Fee paid.                                                                         |
| `IsBuy`        | `"true"` if fill was a buy, else `"false"`.                                       |
| `IsLiquidation`| `"true"` if fill was triggered by forced liquidation.                              |
| `Pnl`          | Profit/loss (string) if available.                                                |
| `Payout`       | Payout from the trade, if present.                                                |
| `SubaccountID` | Trader’s subaccount receiving the fill.                                           |

---

## Project Layout

```
├── cmd
│   └── injective-scanner
│       └── main.go       # CLI entry point
├── pkg
│   └── scanner
│       ├── logs          # Parsing Tx logs (EventNew, EventCancel, EventBatchDerivativeExecution, etc.)
│       ├── msg           # (Optional) If you'd like to parse transaction messages like MsgBatchUpdateOrders
│       ├── types         # Shared structs (CSVRecord, TxLog, etc.)
│       ├── retry.go      # Retry logic for RPC calls
│       └── scanner.go    # Core scanning logic, chunking blocks & writing to CSV
└── go.mod
```

- **`scanner.go`**:  
  Implements `RunScanner`, which queries blocks in chunks and processes each transaction’s logs or messages.
- **`logs/handlers.go`**:  
  Contains the main **event** parsing logic (cancellations, new orders, batch derivative executions, etc.).
- **`types/types.go`**:  
  Defines data structures like `CSVRecord` and helper functions for formatting.

---

## Extensions & Customization

- **Spot Orders**:  
  Adapt `logs/handlers.go` to capture `EventNewSpotOrders` or `EventBatchSpotExecution` if you need spot trades.
- **Message Parsing**:  
  The package `pkg/scanner/msg` can parse messages like `MsgBatchUpdateOrders`. Enable and integrate it if you need more detail beyond logs.
- **Parallelism**:  
  For large block ranges, consider sharding or parallelizing the chunk loops (mind rate limits).
- **Streaming**:  
  If you want near real-time updates, explore `StreamTxs` from the Injective Explorer.
