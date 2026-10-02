package blockchain

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
)

type CLI struct{}

func NewCLI() *CLI {
	return &CLI{}
}

func (cli *CLI) addBlock(transaction []*Transaction) error {
	bc, err := OpenBlockchain()
	if err != nil {
		return err
	}
	defer func() {
		if err := bc.Close(); err != nil {
			fmt.Printf("failed to close database: %v", err)
		}
	}()

	if err := bc.MineBlock(transaction); err != nil {
		return err
	}
	fmt.Println("Success!")
	return nil
}

func (cli *CLI) printChain() error {
	bc, err := OpenBlockchain()
	if err != nil {
		return err
	}
	defer func() {
		if err := bc.Close(); err != nil {
			fmt.Printf("failed to close database: %v", err)
		}
	}()
	bci := bc.Iterator()

	for {
		block, err := bci.Next()
		if err != nil {
			return err
		}

		fmt.Printf("Prev. hash: %x\n", block.PrevBlockHash)
		fmt.Printf("Transaction: %s\n", block.Transactions)
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

func (cli *CLI) createBlockchain(address string) error {
	_, err := CreateBlockchain(address)
	if err != nil {
		return err
	}
	fmt.Println("Done!")

	return nil
}

func (cli *CLI) getBalance(address string) error {
	bc, err := OpenBlockchain()
	if err != nil {
		return err
	}
	defer func() {
		if err := bc.Close(); err != nil {
			fmt.Printf("failed to close database: %v", err)
		}
	}()

	balance := 0
	UTXOs, err := bc.FindUTXO(address)
	if err != nil {
		return err
	}

	for _, out := range UTXOs {
		balance += out.Value
	}

	fmt.Printf("Balance of '%s': %d\n", address, balance)
	return nil
}

func (cli *CLI) send(from, to string, amount int) error {
	bc, err := OpenBlockchain()
	if err != nil {
		return err
	}
	defer func() {
		if err := bc.Close(); err != nil {
			fmt.Printf("failed to close database: %v", err)
		}
	}()

	tx, err := NewUTXOTransaction(from, to, amount, bc)
	if err != nil {
		return err
	}
	bc.MineBlock([]*Transaction{tx})
	fmt.Println("Success!")

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
	createBlockchainCmd := flag.NewFlagSet("createblockchain", flag.ContinueOnError)
	printChainCmd := flag.NewFlagSet("printchain", flag.ContinueOnError)
	getBalanceCmd := flag.NewFlagSet("getbalance", flag.ContinueOnError)
	sendCmd := flag.NewFlagSet("send", flag.ContinueOnError)

	addBlockData := addBlockCmd.String("data", "", "Block data")
	createBlockchainAddress := createBlockchainCmd.String("address", "", "The address to send genesis block reward to")
	getBalanceAddress := getBalanceCmd.String("address", "", "The address to check the balance of")
	sendAmountSender := sendCmd.String("from", "", "sender address")
	sendAmountReceiver := sendCmd.String("to", "", "recipient address")
	sendAmountValue := sendCmd.Int("amount", 0, "amount to send")

	switch os.Args[1] {
	case "addblock":
		if err := addBlockCmd.Parse(os.Args[2:]); err != nil {
			return err
		}
	case "printchain":
		if err := printChainCmd.Parse(os.Args[2:]); err != nil {
			return err
		}
	case "createblockchain":
		if err := createBlockchainCmd.Parse(os.Args[2:]); err != nil {
			return err
		}
	case "getbalance":
		if err := getBalanceCmd.Parse(os.Args[2:]); err != nil {
			return err
		}
	case "send":
		if err := sendCmd.Parse(os.Args[2:]); err != nil {
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
		if err := cli.addBlock(nil); err != nil {
			return err
		}
	}
	if createBlockchainCmd.Parsed() {
		if *createBlockchainAddress == "" {
			createBlockchainCmd.Usage()
			return fmt.Errorf("address cannot be empty")
		}
		if err := cli.createBlockchain(*createBlockchainAddress); err != nil {
			return err
		}
	}
	if printChainCmd.Parsed() {
		return cli.printChain()
	}
	if getBalanceCmd.Parsed() {
		if *getBalanceAddress == "" {
			getBalanceCmd.Usage()
			return fmt.Errorf("address cannot be empty")
		}
		if err := cli.getBalance(*getBalanceAddress); err != nil {
			return err
		}
	}
	if sendCmd.Parsed() {
		if *sendAmountReceiver == "" {
			sendCmd.Usage()
			return fmt.Errorf("receiver cannot be empty")
		}
		if *sendAmountSender == "" {
			sendCmd.Usage()
			return fmt.Errorf("sender cannot be empty")
		}
		// I guess its fine to let people attempt to send 0 values? it might let people spam system with useless requests though
		if err := cli.send(*sendAmountSender, *sendAmountReceiver, *sendAmountValue); err != nil {
			return err
		}
	}

	return nil
}
