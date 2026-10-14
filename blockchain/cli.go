package blockchain

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

// CLI dispatches command-line arguments to blockchain operations.
type CLI struct{}

// NewCLI creates a command-line interface.
func NewCLI() *CLI {
	return &CLI{}
}

func (cli *CLI) validateArgs() error {
	if len(os.Args) < 2 {
		return errors.New("command required")
	}
	return nil
}

func (cli *CLI) printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  createblockchain -address ADDRESS - Create a blockchain and send genesis block reward to ADDRESS")
	fmt.Println("  createwallet - Generates a new key-pair and saves it into the wallet file")
	fmt.Println("  getbalance -address ADDRESS - Get balance of ADDRESS")
	fmt.Println("  listaddresses - Lists all addresses from the wallet file")
	fmt.Println("  printchain - Print all the blocks of the blockchain")
	fmt.Println("  send -from FROM -to TO -amount AMOUNT - Send AMOUNT of coins from FROM address to TO")
}

// Run validates and dispatches the process command-line arguments.
func (cli *CLI) Run() error {
	if err := cli.validateArgs(); err != nil {
		cli.printUsage()
		return fmt.Errorf("validate CLI arguments: %w", err)
	}

	createBlockchainCmd := flag.NewFlagSet("createblockchain", flag.ContinueOnError)
	printChainCmd := flag.NewFlagSet("printchain", flag.ContinueOnError)
	getBalanceCmd := flag.NewFlagSet("getbalance", flag.ContinueOnError)
	sendCmd := flag.NewFlagSet("send", flag.ContinueOnError)

	createBlockchainAddress := createBlockchainCmd.String("address", "", "The address to send genesis block reward to")
	getBalanceAddress := getBalanceCmd.String("address", "", "The address to check the balance of")
	sendAmountSender := sendCmd.String("from", "", "sender address")
	sendAmountReceiver := sendCmd.String("to", "", "recipient address")
	sendAmountValue := sendCmd.Int("amount", 0, "amount to send")

	// Parse only the flag set for the selected command, then dispatch to the
	// command implementation after validating command-specific requirements.
	switch os.Args[1] {
	case "printchain":
		if err := printChainCmd.Parse(os.Args[2:]); err != nil {
			return fmt.Errorf("parse printchain arguments: %w", err)
		}
		if printChainCmd.NArg() != 0 {
			return errors.New("printchain takes no arguments")
		}
		return cli.printChain()
	case "createblockchain":
		if err := createBlockchainCmd.Parse(os.Args[2:]); err != nil {
			return fmt.Errorf("parse createblockchain arguments: %w", err)
		}
		if createBlockchainCmd.NArg() != 0 {
			return errors.New("createblockchain takes no positional arguments")
		}
		if *createBlockchainAddress == "" {
			createBlockchainCmd.Usage()
			return fmt.Errorf("address cannot be empty")
		}
		return cli.createBlockchain(*createBlockchainAddress)
	case "getbalance":
		if err := getBalanceCmd.Parse(os.Args[2:]); err != nil {
			return fmt.Errorf("parse getbalance arguments: %w", err)
		}
		if getBalanceCmd.NArg() != 0 {
			return errors.New("getbalance takes no positional arguments")
		}
		if *getBalanceAddress == "" {
			getBalanceCmd.Usage()
			return fmt.Errorf("address cannot be empty")
		}
		return cli.getBalance(*getBalanceAddress)
	case "send":
		if err := sendCmd.Parse(os.Args[2:]); err != nil {
			return fmt.Errorf("parse send arguments: %w", err)
		}
		if sendCmd.NArg() != 0 {
			return errors.New("send takes no positional arguments")
		}
		if *sendAmountReceiver == "" {
			sendCmd.Usage()
			return fmt.Errorf("receiver cannot be empty")
		}
		if *sendAmountSender == "" {
			sendCmd.Usage()
			return fmt.Errorf("sender cannot be empty")
		}
		if *sendAmountReceiver == *sendAmountSender {
			sendCmd.Usage()
			return fmt.Errorf("sender and receiver cannot be the same person")
		}
		if *sendAmountValue <= 0 {
			sendCmd.Usage()
			return fmt.Errorf("value to send must be greater than 0")
		}
		return cli.send(*sendAmountSender, *sendAmountReceiver, *sendAmountValue)
	case "createwallet":
		if len(os.Args[2:]) != 0 {
			return errors.New("createwallet takes no arguments")
		}
		return createWallet()
	case "listaddresses":
		if len(os.Args[2:]) != 0 {
			return errors.New("listaddresses takes no arguments")
		}
		return listAddresses()
	default:
		cli.printUsage()
		return fmt.Errorf("unknown command: %s", os.Args[1])
	}
}
