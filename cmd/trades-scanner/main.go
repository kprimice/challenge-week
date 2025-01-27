package main

import (
	"flag"
	"log"
	"os"

	"github.com/kprimice/challenge-week/pkg/scanner"
)

func main() {
	marketFlag := flag.String("market", "0x4ca0f92fc28be0c9761326016b5a1a2177dd6375558365116b5bdda9abc229ce", "Market ID to filter.")
	startBlockFlag := flag.Uint64("start", 0, "Start block (optional). If zero, no time-based filtering is applied.")
	endBlockFlag := flag.Uint64("end", 0, "End block (optional). If zero, no time-based filtering is applied.")

	flag.Parse()

	file, err := os.Create("./data/derivative_trades.csv")
	if err != nil {
		log.Fatalf("Failed to create trades CSV file: %v", err)
	}
	defer file.Close()

	// Build config
	cfg := scanner.DerivativeTradesConfig{
		MarketID:   *marketFlag,
		StartBlock: *startBlockFlag,
		EndBlock:   *endBlockFlag,
		PageSize:   100, // default or let user pass a --limit if you want
	}

	if err := scanner.RunDerivativeTrades(cfg, file); err != nil {
		log.Fatalf("RunDerivativeTrades error: %v", err)
	}
	log.Println("Done fetching derivative trades!")
}
