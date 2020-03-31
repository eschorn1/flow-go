package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestGuaranteeInsertCheckRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		g := unittest.CollectionGuaranteeFixture()

		err := db.Update(InsertGuarantee(g))
		require.Nil(t, err)

		var exists bool
		err = db.View(CheckGuarantee(g.CollectionID, &exists))
		require.NoError(t, err)
		require.True(t, exists)

		var retrieved flow.CollectionGuarantee
		err = db.View(RetrieveGuarantee(g.CollectionID, &retrieved))
		require.NoError(t, err)

		assert.Equal(t, g, &retrieved)
	})
}

func TestIndexGuaranteedCollectionByBlockHashInsertRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		height := uint64(1)
		blockID := flow.Identifier{0x10}
		parentID := flow.Identifier{0x20}
		collID1 := flow.Identifier{0x01}
		collID2 := flow.Identifier{0x02}
		collections := []*flow.CollectionGuarantee{
			{CollectionID: collID1, Signatures: []crypto.Signature{{0x10}}},
			{CollectionID: collID2, Signatures: []crypto.Signature{{0x20}}},
		}
		expected := flow.GetIDs(collections)

		err := db.Update(func(tx *badger.Txn) error {
			for _, coll := range collections {
				if err := InsertGuarantee(coll)(tx); err != nil {
					return err
				}
			}
			if err := IndexGuaranteePayload(height, blockID, parentID, expected)(tx); err != nil {
				return err
			}
			return nil
		})
		require.Nil(t, err)

		var actual []flow.Identifier
		err = db.View(LookupGuaranteePayload(height, blockID, parentID, &actual))
		require.Nil(t, err)

		assert.Equal(t, []flow.Identifier{collID1, collID2}, actual)
	})
}

func TestIndexGuaranteedCollectionByBlockHashMultipleBlocks(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		height1 := uint64(1)
		height2 := uint64(2)
		blockID1 := flow.Identifier{0x10}
		blockID2 := flow.Identifier{0x20}
		parentID1 := flow.Identifier{0x30}
		parentID2 := flow.Identifier{0x40}
		collID1 := flow.Identifier{0x01}
		collID2 := flow.Identifier{0x02}
		collID3 := flow.Identifier{0x03}
		collID4 := flow.Identifier{0x04}
		set1 := []*flow.CollectionGuarantee{
			{CollectionID: collID1, Signatures: []crypto.Signature{{0x1}}},
		}
		set2 := []*flow.CollectionGuarantee{
			{CollectionID: collID2, Signatures: []crypto.Signature{{0x2}}},
			{CollectionID: collID3, Signatures: []crypto.Signature{{0x3}}},
			{CollectionID: collID4, Signatures: []crypto.Signature{{0x1}}},
		}
		ids1 := flow.GetIDs(set1)
		ids2 := flow.GetIDs(set2)

		// insert block 1
		err := db.Update(func(tx *badger.Txn) error {
			for _, coll := range set1 {
				if err := InsertGuarantee(coll)(tx); err != nil {
					return err
				}
			}
			if err := IndexGuaranteePayload(height1, blockID1, parentID1, ids1)(tx); err != nil {
				return err
			}
			return nil
		})
		require.Nil(t, err)

		// insert block 2
		err = db.Update(func(tx *badger.Txn) error {
			for _, coll := range set2 {
				if err := InsertGuarantee(coll)(tx); err != nil {
					return err
				}
			}
			if err := IndexGuaranteePayload(height2, blockID2, parentID2, ids2)(tx); err != nil {
				return err
			}
			return nil
		})
		require.Nil(t, err)

		t.Run("should retrieve collections for block", func(t *testing.T) {
			var actual1 []flow.Identifier
			err = db.View(LookupGuaranteePayload(height1, blockID1, parentID1, &actual1))
			assert.NoError(t, err)
			assert.ElementsMatch(t, []flow.Identifier{collID1}, actual1)

			// get block 2
			var actual2 []flow.Identifier
			err = db.View(LookupGuaranteePayload(height2, blockID2, parentID2, &actual2))
			assert.NoError(t, err)
			assert.Equal(t, []flow.Identifier{collID2, collID3, collID4}, actual2)
		})
	})
}

func TestFindDecendants(t *testing.T) {
	t.Run("should find all descendants", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			finalizedHeight := 3
			finalizedBlockID := flow.Identifier{byte(finalizedHeight)}

			err := db.Update(func(tx *badger.Txn) error {
				// add finalized block
				require.Nil(t, addIndex(tx, finalizedHeight, finalizedHeight, finalizedHeight-1))

				// add unfinalized blocks
				require.Nil(t, addIndex(tx, 4, 4, 3))
				require.Nil(t, addIndex(tx, 5, 5, 4))
				require.Nil(t, addIndex(tx, 6, 6, 5))
				require.Nil(t, addIndex(tx, 7, 7, 6))
				require.Nil(t, addIndex(tx, 8, 8, 7))
				require.Nil(t, addIndex(tx, 9, 9, 8))
				return nil
			})
			require.Nil(t, err)

			var descendants []flow.Identifier
			err = db.View(FindDescendants(uint64(finalizedHeight), finalizedBlockID, &descendants))
			require.Nil(t, err)

			assert.Equal(t, toFlowIDs(4, 5, 6, 7, 8, 9), descendants)
		})
	})

	t.Run("should exclude disconnected blocks", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			finalizedHeight := 3
			finalizedBlockID := flow.Identifier{byte(finalizedHeight)}

			err := db.Update(func(tx *badger.Txn) error {
				// add finalized block
				require.Nil(t, addIndex(tx, finalizedHeight, finalizedHeight, finalizedHeight-1))

				// add unfinalized blocks
				require.Nil(t, addIndex(tx, 4, 4, 3))
				require.Nil(t, addIndex(tx, 5, 5, 4))
				require.Nil(t, addIndex(tx, 6, 6, 10)) // block 6 connects to a unknown block
				require.Nil(t, addIndex(tx, 7, 7, 5))
				require.Nil(t, addIndex(tx, 8, 8, 7))
				require.Nil(t, addIndex(tx, 11, 11, 10)) // block 11 connects to a unknown block
				require.Nil(t, addIndex(tx, 12, 12, 11)) // block 12 connects to 11 which connects to a unknown block
				require.Nil(t, addIndex(tx, 13, 13, 8))
				return nil
			})
			require.Nil(t, err)

			var descendants []flow.Identifier
			err = db.View(FindDescendants(uint64(finalizedHeight), finalizedBlockID, &descendants))
			require.Nil(t, err)

			assert.Equal(t, toFlowIDs(4, 5, 7, 8, 13), descendants)
		})
	})

	t.Run("should include conflicting blocks", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			finalizedHeight := 3
			finalizedBlockID := flow.Identifier{byte(finalizedHeight)}

			err := db.Update(func(tx *badger.Txn) error {
				// add finalized block
				require.Nil(t, addIndex(tx, finalizedHeight, finalizedHeight, finalizedHeight-1))

				// add unfinalized blocks
				require.Nil(t, addIndex(tx, 4, 4, 3))
				require.Nil(t, addIndex(tx, 5, 5, 4))
				require.Nil(t, addIndex(tx, 5, 10, 4))  // a different block 5 that also connects to block 4
				require.Nil(t, addIndex(tx, 6, 11, 10)) // a different block 6 that connects to a different block 5
				require.Nil(t, addIndex(tx, 6, 6, 5))
				return nil
			})
			require.Nil(t, err)

			var descendants []flow.Identifier
			err = db.View(FindDescendants(uint64(finalizedHeight), finalizedBlockID, &descendants))
			require.Nil(t, err)

			assert.Equal(t, toFlowIDs(4, 5, 10, 6, 11), descendants)
		})
	})

	t.Run("should not include old finalized blocks", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			finalizedHeight := 3
			finalizedBlockID := flow.Identifier{byte(finalizedHeight)}

			err := db.Update(func(tx *badger.Txn) error {
				require.Nil(t, addIndex(tx, 2, 2, 3)) // add an old finalized block
				// add finalized block
				require.Nil(t, addIndex(tx, finalizedHeight, finalizedHeight, finalizedHeight-1))

				// add unfinalized blocks
				require.Nil(t, addIndex(tx, 4, 4, 3))
				require.Nil(t, addIndex(tx, 5, 5, 4))
				require.Nil(t, addIndex(tx, 6, 6, 5))
				return nil
			})
			require.Nil(t, err)

			var descendants []flow.Identifier
			err = db.View(FindDescendants(uint64(finalizedHeight), finalizedBlockID, &descendants))
			require.Nil(t, err)

			assert.Equal(t, toFlowIDs(4, 5, 6), descendants)
		})
	})

	t.Run("should not include the finalized block itself", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			finalizedHeight := 3
			finalizedBlockID := flow.Identifier{byte(finalizedHeight)}

			err := db.Update(func(tx *badger.Txn) error {
				// this test case verifies that when iterating through the keys,
				// keys as the same height as the finalized block should be ignored
				require.Nil(t, addIndex(tx, finalizedHeight, 1, finalizedHeight))

				// add finalized block
				require.Nil(t, addIndex(tx, finalizedHeight, finalizedHeight, finalizedHeight-1))

				// add unfinalized blocks
				require.Nil(t, addIndex(tx, 4, 4, 3))
				require.Nil(t, addIndex(tx, 5, 5, 4))
				require.Nil(t, addIndex(tx, 6, 6, 5))
				return nil
			})
			require.Nil(t, err)

			var descendants []flow.Identifier
			err = db.View(FindDescendants(uint64(finalizedHeight), finalizedBlockID, &descendants))
			require.Nil(t, err)

			assert.Equal(t, toFlowIDs(4, 5, 6), descendants)
		})
	})
}

// helper to create a list Identifier from numbers
func toFlowIDs(ns ...int) []flow.Identifier {
	ids := make([]flow.Identifier, len(ns))
	for i, n := range ns {
		ids[i] = flow.Identifier{byte(n)}
	}
	return ids
}

func addIndex(tx *badger.Txn, height int, blockID int, parentID int) error {
	return IndexGuaranteePayload(uint64(height), flow.Identifier{byte(blockID)}, flow.Identifier{byte(parentID)}, make([]flow.Identifier, 0))(tx)
}
