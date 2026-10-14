package blockchain

import (
	"encoding/hex"
	"errors"
	"fmt"
)

// FindUnspentTransactions returns transactions containing outputs that are
// currently unspent and locked to pubKeyHash.
func (bc *Blockchain) FindUnspentTransactions(pubKeyHash []byte) ([]Transaction, error) {
	// TODO: Replace this transaction-level result with a UTXO struct containing
	// the transaction ID, output index, and exact TXOutput. Returning whole
	// transactions can duplicate a transaction when multiple outputs match and
	// can make callers accidentally count outputs that were already spent.
	// Update FindUTXO and FindSpendableOutputs to consume those exact UTXOs.
	var unspentTXs []Transaction
	spentTXOs := make(map[string][]int)
	bci := bc.Iterator()

	// Walk newest-to-oldest so later inputs can mark earlier outputs as spent.
	for {
		block, err := bci.Next()
		if err != nil {
			return nil, fmt.Errorf("scan blockchain for unspent transactions: %w", err)
		}

		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)

		Outputs:
			// Collect outputs belonging to this key unless a later transaction
			// already recorded the same output as spent.
			for outIdx, out := range tx.Vout {
				if isOutputSpent(spentTXOs, txID, outIdx) {
					continue Outputs
				}

				if out.IsLockedWithKey(pubKeyHash) {
					unspentTXs = append(unspentTXs, *tx)
				}
			}

			if !tx.IsCoinbase() {
				// Record spends after checking outputs so the current transaction's
				// inputs affect older transactions on the next loop iterations.
				for _, in := range tx.Vin {
					if in.UsesKey(pubKeyHash) {
						inTxID := hex.EncodeToString(in.Txid)
						spentTXOs[inTxID] = append(spentTXOs[inTxID], in.Vout)
					}
				}
			}
		}

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}
	return unspentTXs, nil
}

// FindUTXO returns all unspent outputs locked to pubKeyHash.
func (bc *Blockchain) FindUTXO() (map[string]TXOutputs, error) {
	// FindUnspentTransactions performs chain traversal; this method projects
	// those matching transactions down to just their spendable outputs.
	UTXO := make(map[string]TXOutputs)
	spentTXOs := make(map[string][]int)
	bci := bc.Iterator()

	for {
		block, err := bci.Next()
		if err != nil {
			return nil, fmt.Errorf("get next block: %w", err)
		}
		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)

		Outputs:
			for outIdx, out := range tx.Vout {
				// was output spent?
				if isOutputSpent(spentTXOs, txID, outIdx) {
					continue Outputs
				}

				outs := UTXO[txID]
				outs.Outputs = append(outs.Outputs, out)
				UTXO[txID] = outs
			}

			if !tx.IsCoinbase() {
				for _, in := range tx.Vin {
					inTxID := hex.EncodeToString(in.Txid)
					spentTXOs[inTxID] = append(spentTXOs[inTxID], in.Vout)
				}
			}
		}

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}
	return UTXO, nil
}

// isOutputSpent reports whether a later transaction consumed this output.
func isOutputSpent(spentOutputs map[string][]int, txID string, outputIndex int) bool {
	for _, spentIndex := range spentOutputs[txID] {
		if spentIndex == outputIndex {
			return true
		}
	}
	return false
}

// NewUTXOTransaction creates, signs, and returns a transaction spending the
// sender's unspent outputs. Any excess value is returned to the sender as
// change.
func NewUTXOTransaction(from, to string, amount int, UTXOSet *UTXOSet) (*Transaction, error) {
	if amount <= 0 {
		return nil, errors.New("amount must be greater than zero")
	}

	// Load the sender's private/public key pair so the new inputs can be signed.
	wallets, err := NewWallets()
	if err != nil {
		return nil, fmt.Errorf("load wallets for transaction: %w", err)
	}
	wallet, err := wallets.GetWallet(from)
	if err != nil {
		return nil, fmt.Errorf("load sender wallet %q: %w", from, err)
	}
	// Use the sender's public-key hash to discover outputs the sender can spend.
	pubKeyHash := wallet.GetPubKeyHash()
	selection, err := UTXOSet.FindSpendableOutputs(pubKeyHash, amount)
	if err != nil {
		return nil, fmt.Errorf("select spendable outputs for %q: %w", from, err)
	}

	if selection.Amount < amount {
		return nil, errors.New("Not enough funds")
	}

	// Build recipient and change outputs separately from the inputs being spent.
	inputs, err := newTransactionInputs(selection.Outputs, wallet.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("build transaction inputs: %w", err)
	}

	outputs, err := newTransactionOutputs(amount, selection.Amount, from, to)
	if err != nil {
		return nil, fmt.Errorf("build transaction outputs: %w", err)
	}

	// Hash the unsigned transaction first, then sign each input against the
	// previous output it spends. The transaction ID is recalculated by Hash
	// before signing, while signatures are added afterward.
	tx := Transaction{nil, inputs, outputs}
	tx.ID, err = tx.Hash()
	if err != nil {
		return nil, fmt.Errorf("hash new transaction: %w", err)
	}
	if err := UTXOSet.Blockchain.SignTransaction(&tx, wallet.PrivateKey); err != nil {
		return nil, fmt.Errorf("sign new transaction: %w", err)
	}

	return &tx, nil
}

func newTransactionInputs(validOutputs map[string][]int, publicKey []byte) ([]TXInput, error) {
	var inputs []TXInput

	for txid, outs := range validOutputs {
		txID, err := hex.DecodeString(txid)
		if err != nil {
			return nil, fmt.Errorf("decode transaction ID %q: %w", txid, err)
		}

		for _, out := range outs {
			inputs = append(inputs, TXInput{txID, out, nil, publicKey})
		}
	}

	return inputs, nil
}

func newTransactionOutputs(amount, available int, from, to string) ([]TXOutput, error) {
	newtxo, err := NewTXOutput(amount, to)
	if err != nil {
		return nil, fmt.Errorf("create recipient output: %w", err)
	}
	outputs := []TXOutput{*newtxo}

	if available > amount {
		newtxo, err = NewTXOutput(available-amount, from)
		if err != nil {
			return nil, fmt.Errorf("create change output: %w", err)
		}
		outputs = append(outputs, *newtxo)
	}

	return outputs, nil
}
