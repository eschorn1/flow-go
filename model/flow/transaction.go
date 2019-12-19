package flow

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/hash"
)

// TransactionBody includes the main contents of a transaction
type TransactionBody struct {
	// A reference to a previous block
	// A transaction is expired after specific number of blocks (defined by network) counting from this block
	// for example, if block reference is pointing to a block with height of X and network limit is 10,
	// a block with x+10 height is the last block that is allowed to include this transaction.
	// user can adjust this reference to older blocks if he/she wants to make tx expire faster
	ReferenceBlockHash model.Fingerprint
	// the script part of the transaction in Cadence Language
	Script []byte
	// a unique random number to differentiate two transactions with the same properties
	// it doesn't have to be sequential, and we might remove it in the future
	Nonce uint64
	// Max amount of computation which is allowed to be done during this transaction
	ComputeLimit uint64
	// Account that pays for this transaction fees
	PayerAccount Address
	// A ordered (ascending) list of addresses that scripts will touch their assets (including payer address)
	// Accounts listed here all have to provide signatures
	// Each account might provide multiple signatures (sum of weight should be at least 1)
	// If code touches accounts that is not listed here, tx fails
	ScriptAccounts []Address
	// List of account signatures including signatures of the payer account and the script accounts
	Signatures []AccountSignature
}

// Transaction is the smallest unit of task.
type Transaction struct {
	// Body of the transaction
	TransactionBody
	Status           TransactionStatus
	Events           []Event
	ComputationSpent uint64
	StartState       StateCommitment
	EndState         StateCommitment
}

// Fingerprint hashes the transaction body
// Fingerprint provides a meseaure to check the integrity of the content
func (tx *Transaction) Fingerprint() model.Fingerprint {
	return model.Fingerprint(hash.DefaultHasher.ComputeHash(encoding.DefaultEncoder.MustEncode(tx.TransactionBody)))
}

// ID returns an Identifier (unique id) for this transaction
// ID is generated by hashing the transaction body only
func (tx *Transaction) ID() model.Identifier {
	txBodyHash := hash.DefaultHasher.ComputeHash(encoding.DefaultEncoder.MustEncode(tx.TransactionBody))
	var id model.Identifier
	copy(id[:], txBodyHash)
	return id
}

func (tx *Transaction) String() string {
	return fmt.Sprintf("Transaction %v submitted by %v (block %v)",
		tx.Hash(), tx.PayerAccount.Hex(), tx.ReferenceBlockHash)
}

// Hash returns the canonical hash of this transaction.
func (tx *Transaction) Hash() crypto.Hash {
	return hash.DefaultHasher.ComputeHash(encoding.DefaultEncoder.MustEncode(tx))
}

// NewTransaction initialize a transaction
func NewTransaction(blockref []byte, scrip []byte, nonce uint64, cl uint64, payer Address, sa []Address) *Transaction {
	txBody := TransactionBody{ReferenceBlockHash: blockref,
		Script:         scrip,
		Nonce:          nonce,
		ComputeLimit:   cl,
		PayerAccount:   payer,
		ScriptAccounts: sa}
	return &Transaction{TransactionBody: txBody}
}

// AddSignature signs the transaction with the given account and private key, then adds the signature to the list
// of signatures.
func (tx *Transaction) AddSignature(account Address, sig crypto.Signature) {
	accountSig := AccountSignature{
		Account:   account,
		Signature: sig.Bytes(),
	}

	tx.Signatures = append(tx.Signatures, accountSig)
}

// TransactionStatus represents the status of a Transaction.
type TransactionStatus int

const (
	// TransactionStatusUnknown indicates that the transaction status is not known.
	TransactionStatusUnknown TransactionStatus = iota
	// TransactionPending is the status of a pending transaction.
	TransactionPending
	// TransactionFinalized is the status of a finalized transaction.
	TransactionFinalized
	// TransactionReverted is the status of a reverted transaction.
	TransactionReverted
	// TransactionSealed is the status of a sealed transaction.
	TransactionSealed
)

// String returns the string representation of a transaction status.
func (s TransactionStatus) String() string {
	return [...]string{"PENDING", "FINALIZED", "REVERTED", "SEALED"}[s]
}

// TransactionField represents a required transaction field.
type TransactionField int

const (
	TransactionFieldUnknown TransactionField = iota
	TransactionFieldScript
	TransactionFieldRefBlockHash
	TransactionFieldNonce
	TransactionFieldComputeLimit
	TransactionFieldPayerAccount
)

// String returns the string representation of a transaction field.
func (f TransactionField) String() string {
	return [...]string{"Unknown", "Script", "ReferenceBlockHash", "Nonce", "ComputeLimit", "PayerAccount"}[f]
}

// MissingFields checks if a transaction is missing any required fields and returns those that are missing.
func (tx *Transaction) MissingFields() []string {
	// Required fields are Script, ReferenceBlockHash, Nonce, ComputeLimit, PayerAccount
	missingFields := make([]string, 0)

	if len(tx.Script) == 0 {
		missingFields = append(missingFields, TransactionFieldScript.String())
	}

	// TODO: need to refactor tests to include ReferenceBlockHash field (i.e. b.GetLatestBlock().Hash() should do)
	// if len(tx.ReferenceBlockHash) == 0 {
	// 	missingFields = append(missingFields, TransactionFieldRefBlockHash.String())
	// }

	if tx.Nonce == 0 {
		missingFields = append(missingFields, TransactionFieldNonce.String())
	}

	if tx.ComputeLimit == 0 {
		missingFields = append(missingFields, TransactionFieldComputeLimit.String())
	}

	if tx.PayerAccount == ZeroAddress {
		missingFields = append(missingFields, TransactionFieldPayerAccount.String())
	}

	return missingFields
}
