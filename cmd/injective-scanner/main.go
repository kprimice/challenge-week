package main

import (
	"flag"
	"log"
	"os"

	"github.com/kprimice/challenge-week/pkg/scanner"
	"github.com/kprimice/challenge-week/pkg/scanner/types"
)

func main() {
	startFlag := flag.Uint64("start", 100000000, "Block number to start scanning downward from.")
	endFlag := flag.Uint64("end", 103000000, "Block number to stop at (inclusive).")
	marketFlag := flag.String("market", "0x4ca0f92fc28be0c9761326016b5a1a2177dd6375558365116b5bdda9abc229ce", "Market ID to filter (optional). If empty, parse all BTC-PERP.")

	flag.Parse()

	ordersFile, err := os.Create("./data/orders.csv")
	if err != nil {
		log.Fatalf("failed to create orders CSV file: %w", err)
	}
	defer ordersFile.Close()

	tradesFile, err := os.Create("./data/trades.csv")
	if err != nil {
		log.Fatalf("failed to create trades CSV file: %w", err)
	}
	defer tradesFile.Close()

	cfg := types.Config{
		StartBlock: *startFlag,
		EndBlock:   *endFlag,
		MarketID:   *marketFlag,

		// You can add more fields if needed (like concurrency, pageSize, chain network, etc.)
	}

	if err := scanner.RunScanner(cfg, ordersFile, tradesFile); err != nil {
		log.Fatalf("Scanner error: %v", err)
	}

	log.Println("Done!")
}
