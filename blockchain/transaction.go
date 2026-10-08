package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/btcsuite/btcutil/base58"
)

const subsidy = 10 // This is the reward given, just leaving as constant instead of dividing in half every X amount of blocks

type TXOutput struct {
	Value      int
	PubKeyHash []byte
}

type TXInput struct {
	Txid      []byte
	Vout      int
	Signature []byte
	PubKey    []byte
}

type Transaction struct {
	ID   []byte
	Vin  []TXInput
	Vout []TXOutput
}

func (tx *Transaction) Serialize() ([]byte, error) {
	data, err := json.Marshal(tx)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (tx *Transaction) IsCoinbase() bool {
	return len(tx.Vin) == 1 && len(tx.Vin[0].Txid) == 0 && tx.Vin[0].Vout == -1
}

func NewCoinbaseTX(to, data string) (*Transaction, error) {
	if len(data) == 0 {
		data = fmt.Sprintf("Reward to '%s'", to)
	}

	txin := TXInput{[]byte{}, -1, nil, []byte(data)}
	txout, err := NewTXOutput(subsidy, to)
	if err != nil {
		return nil, err
	}
	tx := Transaction{nil, []TXInput{txin}, []TXOutput{*txout}}
	hash, err := tx.Hash()
	if err != nil {
		return nil, err
	}
	tx.ID = hash

	return &tx, nil
}

func (in *TXInput) UsesKey(pubKeyHash []byte) bool {
	lockingHash := HashPubKey(in.PubKey)

	return bytes.Equal(lockingHash, pubKeyHash)
}
func (out *TXOutput) Lock(address string) error {
	pubKeyHash, err := PubKeyHashFromAddress(address)
	if err != nil {
		return err
	}
	out.PubKeyHash = pubKeyHash

	return nil
}

func (out *TXOutput) IsLockedWithKey(pubKeyHash []byte) bool {
	return bytes.Equal(out.PubKeyHash, pubKeyHash)
}

func NewTXOutput(value int, address string) (*TXOutput, error) {
	txo := &TXOutput{Value: value, PubKeyHash: nil}
	if err := txo.Lock(address); err != nil {
		return nil, err
	}

	return txo, nil
}

func PubKeyHashFromAddress(address string) ([]byte, error) {
	decoded := base58.Decode(address)

	const publicKeyHashLen = sha256.Size
	expectedLen := addressVersionLen + publicKeyHashLen + addressChecksumLen
	if len(decoded) != expectedLen {
		return nil, fmt.Errorf("invalid address length: got %d bytes, want %d", len(decoded), expectedLen)
	}

	if decoded[0] != version {
		return nil, fmt.Errorf("invalid address version: got %d, want %d", decoded[0], version)
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
	// TODO: Replace this transaction-level result with a UTXO struct containing
	// the transaction ID, output index, and exact TXOutput. Returning whole
	// transactions can duplicate a transaction when multiple outputs match and
	// can make callers accidentally count outputs that were already spent.
	// Update FindUTXO and FindSpendableOutputs to consume those exact UTXOs.
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

func NewUTXOTransaction(from, to string, amount int, bc *Blockchain) (*Transaction, error) {
	var inputs []TXInput
	var outputs []TXOutput

	wallets, err := NewWallets()
	if err != nil {
		return nil, err
	}
	wallet, err := wallets.GetWallet(from)
	if err != nil {
		return nil, err
	}
	pubKeyHash := wallet.GetPubKeyHash()
	acc, validOutputs, err := bc.FindSpendableOutputs(pubKeyHash, amount)
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
			input := TXInput{txID, out, nil, wallet.PublicKey}
			inputs = append(inputs, input)
		}
	}

	// Build list of outputs
	newtxo, err := NewTXOutput(amount, to)
	if err != nil {
		return nil, err
	}
	outputs = append(outputs, *newtxo)
	if acc > amount {
		newtxo, err = NewTXOutput(acc-amount, from)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, *newtxo)
	}

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

func (tx *Transaction) Hash() ([]byte, error) {
	var hash [32]byte

	txCopy := *tx
	txCopy.ID = []byte{}

	data, err := txCopy.Serialize()
	if err != nil {
		return nil, err
	}
	hash = sha256.Sum256(data)

	return hash[:], nil
}

func (tx *Transaction) TrimmedCopy() Transaction {
	// We want to sign public key hashes in unlocked outputs (sender), public key hashes in new locked outputs (receiver), and values of outputs
	var inputs []TXInput
	var outputs []TXOutput

	for _, vin := range tx.Vin {
		inputs = append(inputs, TXInput{vin.Txid, vin.Vout, nil, nil})
	}
	// Copy each output into a new slice so the trimmed transaction has
	// independent TXOutput structs. The PubKeyHash byte slices remain shared.
	for _, vout := range tx.Vout {
		outputs = append(outputs, TXOutput{vout.Value, vout.PubKeyHash})
	}

	txCopy := Transaction{tx.ID, inputs, outputs}

	return txCopy
}

func (tx *Transaction) Sign(privKey ecdsa.PrivateKey, prevTXs map[string]Transaction) error {
	if tx.IsCoinbase() {
		return nil
	}

	txCopy := tx.TrimmedCopy()

	for inID, vin := range txCopy.Vin {
		prevTx := prevTXs[hex.EncodeToString(vin.Txid)]
		// should already be nil but double check
		txCopy.Vin[inID].Signature = nil
		txCopy.Vin[inID].PubKey = prevTx.Vout[vin.Vout].PubKeyHash
		hash, err := txCopy.Hash()
		if err != nil {
			return err
		}

		txCopy.ID = hash
		txCopy.Vin[inID].PubKey = nil

		signature, err := ecdsa.SignASN1(rand.Reader, &privKey, txCopy.ID)
		if err != nil {
			return err
		}

		tx.Vin[inID].Signature = signature
	}
	return nil
}

// TODO: Harden verification against malformed transactions by checking that
// each previous transaction exists and that vin.Vout is within its Vout range
// before indexing it. Handle coinbase transactions separately before applying
// normal input ownership and signature checks.
func (tx *Transaction) Verify(prevTXs map[string]Transaction) bool {
	txCopy := tx.TrimmedCopy()

	for inID, vin := range tx.Vin {
		prevTx := prevTXs[hex.EncodeToString(vin.Txid)]

		// check that public key belongs to previous output
		if !slices.Equal(HashPubKey(vin.PubKey), prevTx.Vout[vin.Vout].PubKeyHash) {
			return false
		}
		txCopy.Vin[inID].Signature = nil
		txCopy.Vin[inID].PubKey = prevTx.Vout[vin.Vout].PubKeyHash
		data, err := txCopy.Hash()
		if err != nil {
			return false
		}
		txCopy.ID = data
		txCopy.Vin[inID].PubKey = nil

		rawPubKey, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), vin.PubKey)
		if err != nil {
			return false
		}

		if !ecdsa.VerifyASN1(rawPubKey, txCopy.ID, vin.Signature) {
			return false
		}
	}

	return true
}
