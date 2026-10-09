package blockchain

import (
	"encoding/hex"
	"errors"
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
			return nil, err
		}

		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)

		Outputs:
			// Collect outputs belonging to this key unless a later transaction
			// already recorded the same output as spent.
			for outIdx, out := range tx.Vout {
				if spentTXOs[txID] != nil {
					for _, spentOut := range spentTXOs[txID] {
						if spentOut == outIdx {
							continue Outputs
						}
					}
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
func (bc *Blockchain) FindUTXO(pubKeyHash []byte) ([]TXOutput, error) {
	// FindUnspentTransactions performs chain traversal; this method projects
	// those matching transactions down to just their spendable outputs.
	var utxos []TXOutput
	unspentTransactions, err := bc.FindUnspentTransactions(pubKeyHash)

	if err != nil {
		return nil, err
	}

	for _, tx := range unspentTransactions {
		for _, out := range tx.Vout {
			if out.IsLockedWithKey(pubKeyHash) {
				utxos = append(utxos, out)
			}
		}
	}
	return utxos, nil
}

// FindSpendableOutputs selects enough unspent outputs to cover amount.
func (bc *Blockchain) FindSpendableOutputs(pubKeyHash []byte, amount int) (int, map[string][]int, error) {
	if amount <= 0 {
		return -1, nil, errors.New("amount must be greater than 0")
	}
	unspentOutputs := make(map[string][]int)
	unspentTXs, err := bc.FindUnspentTransactions(pubKeyHash)
	if err != nil {
		return -1, nil, err
	}
	accumulated := 0

	// Select outputs until the requested amount is covered. The map preserves
	// both the transaction ID and output indexes needed to build inputs.
Work:
	for _, tx := range unspentTXs {
		txID := hex.EncodeToString(tx.ID)

		for outIdx, out := range tx.Vout {
			if out.IsLockedWithKey(pubKeyHash) && accumulated < amount {
				accumulated += out.Value
				unspentOutputs[txID] = append(unspentOutputs[txID], outIdx)

				if accumulated >= amount {
					break Work
				}
			}
		}
	}
	return accumulated, unspentOutputs, nil
}

// NewUTXOTransaction creates, signs, and returns a transaction spending the
// sender's unspent outputs. Any excess value is returned to the sender as
// change.
func NewUTXOTransaction(from, to string, amount int, bc *Blockchain) (*Transaction, error) {
	if amount <= 0 {
		return nil, errors.New("amount must be greater than zero")
	}

	// Load the sender's private/public key pair so the new inputs can be signed.
	wallets, err := NewWallets()
	if err != nil {
		return nil, err
	}
	wallet, err := wallets.GetWallet(from)
	if err != nil {
		return nil, err
	}
	// Use the sender's public-key hash to discover outputs the sender can spend.
	pubKeyHash := wallet.GetPubKeyHash()
	acc, validOutputs, err := bc.FindSpendableOutputs(pubKeyHash, amount)
	if err != nil {
		return nil, err
	}

	if acc < amount {
		return nil, errors.New("Not enough funds")
	}

	// Build recipient and change outputs separately from the inputs being spent.
	inputs, err := newTransactionInputs(validOutputs, wallet.PublicKey)
	if err != nil {
		return nil, err
	}

	outputs, err := newTransactionOutputs(amount, acc, from, to)
	if err != nil {
		return nil, err
	}

	// Hash the unsigned transaction first, then sign each input against the
	// previous output it spends.
	tx := Transaction{nil, inputs, outputs}
	tx.ID, err = tx.Hash()
	if err != nil {
		return nil, err
	}
	if err := bc.SignTransaction(&tx, wallet.PrivateKey); err != nil {
		return nil, err
	}

	return &tx, nil
}

func newTransactionInputs(validOutputs map[string][]int, publicKey []byte) ([]TXInput, error) {
	var inputs []TXInput

	for txid, outs := range validOutputs {
		txID, err := hex.DecodeString(txid)
		if err != nil {
			return nil, err
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
		return nil, err
	}
	outputs := []TXOutput{*newtxo}

	if available > amount {
		newtxo, err = NewTXOutput(available-amount, from)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, *newtxo)
	}

	return outputs, nil
}
