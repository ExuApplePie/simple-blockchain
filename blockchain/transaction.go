package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/btcsuite/btcutil/base58"
)

const subsidy = 10 // This is the reward given, just leaving as constant instead of dividing in half every X amount of blocks

type TXOutput struct {
	Value      int
	PubKeyHash []byte
}

type TXInput struct {
	Txid   []byte
	Vout   int
	PubKey []byte
}

type Transaction struct {
	ID   []byte
	Vin  []TXInput
	Vout []TXOutput
}

func (tx *Transaction) SetID() error {
	data, err := json.Marshal(tx)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(data)
	tx.ID = hash[:]
	return nil
}

func (tx *Transaction) IsCoinbase() bool {
	return len(tx.Vin) == 1 && len(tx.Vin[0].Txid) == 0 && tx.Vin[0].Vout == -1
}

func NewCoinbaseTX(to, data []byte) (*Transaction, error) {
	if len(data) == 0 {
		data = fmt.Appendf(nil, "Reward to '%x'", to)
	}

	txin := TXInput{[]byte{}, -1, data}
	txout := TXOutput{subsidy, to}
	tx := Transaction{nil, []TXInput{txin}, []TXOutput{txout}}
	if err := tx.SetID(); err != nil {
		return nil, err
	}

	return &tx, nil
}

func (in *TXInput) UsesKey(pubKeyHash []byte) bool {
	lockingHash := HashPubKey(in.PubKey)

	return bytes.Equal(lockingHash, pubKeyHash)
}
func (out *TXOutput) Lock(pubKeyHash []byte) {
	// decoded := base58.Decode(address)
	pubKeyHash = pubKeyHash[addressVersionLen : len(pubKeyHash)-addressChecksumLen]
	out.PubKeyHash = pubKeyHash
}

func (out *TXOutput) IsLockedWithKey(pubKeyHash []byte) bool {
	return bytes.Equal(out.PubKeyHash, pubKeyHash)
}

func PubKeyHashFromAddress(address string) ([]byte, error) {
	decoded := base58.Decode(address)

	if len(decoded) < addressChecksumLen+addressVersionLen {
		return nil, errors.New("invalid address")
	}

	payloadEnd := len(decoded) - addressChecksumLen
	payload := decoded[:payloadEnd]

	expectedChecksum := checksum(payload)
	actualChecksum := decoded[payloadEnd:]

	if !bytes.Equal(expectedChecksum, actualChecksum) {
		return nil, errors.New("invalid address checksum")
	}

	return payload[addressVersionLen:], nil
}

func (bc *Blockchain) FindUnspentTransactions(pubKeyHash []byte) ([]Transaction, error) {
	var unspentTXs []Transaction
	spentTXOs := make(map[string][]int)
	bci := bc.Iterator()

	for {
		block, err := bci.Next()
		if err != nil {
			return nil, err
		}

		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)

		Outputs:
			for outIdx, out := range tx.Vout {
				// was output spent?
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

			if tx.IsCoinbase() == false {
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

func (bc *Blockchain) FindUTXO(pubKeyHash []byte) ([]TXOutput, error) {
	var UTXOs []TXOutput
	unspentTransactions, err := bc.FindUnspentTransactions(pubKeyHash)

	if err != nil {
		return nil, err
	}

	for _, tx := range unspentTransactions {
		for _, out := range tx.Vout {
			if out.IsLockedWithKey(pubKeyHash) {
				UTXOs = append(UTXOs, out)
			}
		}
	}
	return UTXOs, nil
}

func (bc *Blockchain) FindSpendableOutputs(pubKeyHash []byte, amount int) (int, map[string][]int, error) {
	unspentOutputs := make(map[string][]int)
	unspentTXs, err := bc.FindUnspentTransactions(pubKeyHash)
	if err != nil {
		return -1, nil, err
	}
	accumulated := 0

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

func NewUTXOTransaction(from, to []byte, amount int, bc *Blockchain) (*Transaction, error) {
	var inputs []TXInput
	var outputs []TXOutput

	acc, validOutputs, err := bc.FindSpendableOutputs(from, amount)
	if err != nil {
		return nil, err
	}

	if acc < amount {
		return nil, errors.New("Not enough funds")
	}

	// Build list of inputs
	for txid, outs := range validOutputs {
		txID, err := hex.DecodeString(txid)
		if err != nil {
			return nil, err
		}

		for _, out := range outs {
			input := TXInput{txID, out, from}
			inputs = append(inputs, input)
		}
	}

	// Build list of outputs
	outputs = append(outputs, TXOutput{amount, to})
	if acc > amount {
		outputs = append(outputs, TXOutput{acc - amount, from})
	}

	tx := Transaction{nil, inputs, outputs}
	tx.SetID()

	return &tx, nil
}
