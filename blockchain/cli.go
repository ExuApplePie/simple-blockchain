package blockchain

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
)

type CLI struct {
	bc *Blockchain
}

func NewCLI(bc *Blockchain) *CLI {
	return &CLI{bc: bc}
}

func (cli *CLI) addBlock(data string) error {
	if err := cli.bc.AddBlock(data); err != nil {
		return err
	}
	fmt.Println("Success!")
	return nil
}

func (cli *CLI) printChain() error {
	bci := cli.bc.Iterator()

	for {
		block, err := bci.Next()
		if err != nil {
			return err
		}

		fmt.Printf("Prev. hash: %x\n", block.PrevBlockHash)
		fmt.Printf("Data: %s\n", block.Data)
		fmt.Printf("Hash: %x\n", block.Hash)
		pow := NewProofOfWork(block)
		fmt.Printf("PoW: %s\n", strconv.FormatBool(pow.Validate()))
		fmt.Println()

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}
	return nil
}

func (cli *CLI) validateArgs() error {
	if len(os.Args) < 2 {
		return errors.New("command required")
	}
	return nil
}

func (cli *CLI) printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  addblock -data \"<block data>\"")
	fmt.Println("  printchain")
}

func (cli *CLI) Run() error {
	if err := cli.validateArgs(); err != nil {
		cli.printUsage()
		return err
	}

	addBlockCmd := flag.NewFlagSet("addblock", flag.ContinueOnError)
	printChainCmd := flag.NewFlagSet("printchain", flag.ContinueOnError)

	addBlockData := addBlockCmd.String("data", "", "Block data")

	switch os.Args[1] {
	case "addblock":
		if err := addBlockCmd.Parse(os.Args[2:]); err != nil {
			return err
		}
	case "printchain":
		if err := printChainCmd.Parse(os.Args[2:]); err != nil {
			return err
		}
	default:
		cli.printUsage()
		return fmt.Errorf("unknown command: %s", os.Args[1])
	}

	if addBlockCmd.Parsed() {
		if *addBlockData == "" {
			addBlockCmd.Usage()
			return fmt.Errorf("block data cannot be empty")
		}
		if err := cli.addBlock(*addBlockData); err != nil {
			return err
		}
	}

	if printChainCmd.Parsed() {
		return cli.printChain()
	}
	return nil
}
