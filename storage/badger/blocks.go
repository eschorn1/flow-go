// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// Blocks implements a simple read-only block storage around a badger DB.
type Blocks struct {
	db *badger.DB
}

func NewBlocks(db *badger.DB) *Blocks {
	b := &Blocks{
		db: db,
	}
	return b
}

func (b *Blocks) ByHash(hash crypto.Hash) (*flow.Block, error) {

	var block *flow.Block
	err := b.db.View(func(tx *badger.Txn) error {

		var err error
		block, err = b.retrieveBlock(tx, hash)
		if err != nil {
			if err == storage.NotFoundErr {
				return err
			}
			return errors.Wrap(err, "could not retrieve block")
		}

		return nil
	})

	return block, err
}

func (b *Blocks) ByNumber(number uint64) (*flow.Block, error) {

	var block *flow.Block
	err := b.db.View(func(tx *badger.Txn) error {

		// get the hash
		var hash crypto.Hash
		err := operation.RetrieveHash(number, &hash)(tx)
		if err != nil {
			if err == storage.NotFoundErr {
				return err
			}
			return errors.Wrap(err, "could not retrieve hash")
		}

		block, err = b.retrieveBlock(tx, hash)
		if err != nil {
			if err == storage.NotFoundErr {
				return err
			}
			return errors.Wrap(err, "could not retrieve block")
		}

		return nil
	})

	return block, err
}

func (b *Blocks) retrieveBlock(tx *badger.Txn, hash crypto.Hash) (*flow.Block, error) {

	// get the header
	var header flow.Header
	err := operation.RetrieveHeader(hash, &header)(tx)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve header")
	}

	// get the new identities
	var identities flow.IdentityList
	err = operation.RetrieveIdentities(hash, &identities)(tx)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve identities")
	}

	// get the guaranteed collections
	var collections []*flow.GuaranteedCollection
	err = operation.RetrieveGuaranteedCollections(hash, &collections)(tx)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve collections")
	}

	// create the block
	block := &flow.Block{
		Header:                header,
		NewIdentities:         identities,
		GuaranteedCollections: collections,
	}

	return block, nil
}

func (b *Blocks) Save(block *flow.Block) error {

	err := b.db.Update(func(tx *badger.Txn) error {

		err := operation.PersistHeader(&block.Header)(tx)
		if err != nil {
			return errors.Wrap(err, "cannot save block")
		}
		err = operation.PersistIdentities(block.Hash(), block.NewIdentities)(tx)
		if err != nil {
			return errors.Wrap(err, "cannot save block")
		}
		err = operation.PersistGuaranteedCollections(block.Hash(), block.GuaranteedCollections)(tx)
		if err != nil {
			return errors.Wrap(err, "cannot save block")
		}
		err = operation.PersistHash(block.Number, block.Hash())(tx)
		if err != nil {
			return errors.Wrap(err, "cannot save block")
		}

		return nil
	})
	return err

}
