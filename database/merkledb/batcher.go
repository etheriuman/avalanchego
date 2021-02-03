package merkledb

import (
	"github.com/ava-labs/avalanchego/database"
)

type batchValue struct {
	key    []byte
	value  []byte
	delete bool
}

// Batch is a write-only database that commits changes to its host database
// when Write is called. A batch cannot be used concurrently.
type Batch struct {
	data []batchValue
	tree *Tree
}

func NewBatch(t *Tree) database.Batch {
	return &Batch{
		data: nil,
		tree: t,
	}
}

// ValueSize retrieves the amount of data queued up for writing.
func (b *Batch) ValueSize() int {
	return len(b.data)
}

// Write flushes any accumulated data to disk.
// TODO Optimize
// If the size of the batch is smaller than the trie, which it will be in all realistic cases,
// then it makes sense to do some pre-processing such that the batched operations don't modify the same part of the trie multiple times.
func (b *Batch) Write() error {
	var err error

	for _, d := range b.data {
		if d.delete {
			err = b.tree.del(d.key)
		} else {
			err = b.tree.put(d.key, d.value)
		}

		if err != nil {
			return err
		}
	}
	return b.tree.persistence.Commit(nil)
}

// Reset resets the batch for reuse.
func (b *Batch) Reset() {
	b.data = nil
}

// Replay replays the batch contents.
func (b *Batch) Replay(w database.KeyValueWriter) error {
	for _, val := range b.data {
		if val.delete {
			if err := w.Delete(val.key); err != nil {
				return err
			}
		} else if err := w.Put(val.key, val.value); err != nil {
			return err
		}
	}
	return nil
}

// Put inserts the given value into the key-value data store.
func (b *Batch) Put(key []byte, value []byte) error {
	b.data = append(b.data, batchValue{
		key:    key,
		value:  value,
		delete: false,
	})
	return nil
}

// Delete removes the key from the key-value data store.
func (b *Batch) Delete(key []byte) error {
	b.data = append(b.data, batchValue{
		key:    key,
		value:  nil,
		delete: true,
	})
	return nil
}

// Inner returns a Batch writing to the inner database, if one exists. If
// this batch is already writing to the base DB, then itself should be
// returned.
func (b *Batch) Inner() database.Batch {
	return b
}