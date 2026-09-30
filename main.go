package main

import (
	"log"

	"github.com/ExuApplePie/simple-blockchain/blockchain"
)

func main() {
	bc, err := blockchain.NewBlockchain()
	if err != nil {
		log.Fatalf("Error details: %v", err)
	}
	defer func() {
		if err := bc.Close(); err != nil {
			log.Printf("failed to close database: %v", err)
		}
	}()

	cli := blockchain.NewCLI(bc)
	if err := cli.Run(); err != nil {
		log.Printf("CLI error: %v", err)
		return
	}

	// bc.AddBlock("Send 1 BTC to Exusiai")
	// bc.AddBlock("Send 2 more BTC to Exusiai")

	// for _, block := range bc.Blocks() {
	// 	fmt.Printf("Prev. Hash: %x\n", block.PrevBlockHash)
	// 	fmt.Printf("Data: %s\n", block.Data)
	// 	fmt.Printf("Hash: %x\n", block.Hash)
	// 	pow := blockchain.NewProofOfWork(block)
	// 	fmt.Printf("PoW: %s\n", strconv.FormatBool(pow.Validate()))
	// 	fmt.Println()
	// }
}
