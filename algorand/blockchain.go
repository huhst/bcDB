package algorand

import (
	"bcDB_algorand/algorand/common"
	"sync"
)

var Mu sync.RWMutex

type Blockchain struct {
	Last    *Block
	Genesis *Block
	Blocks  map[uint64]map[common.Hash]*Block
}

func NewDataBlockchain() *Blockchain {
	bc := &Blockchain{
		Blocks: make(map[uint64]map[common.Hash]*Block),
	}
	bc.init()
	return bc
}
func NewTableBlockchain() *Blockchain {
	bc := &Blockchain{
		Blocks: make(map[uint64]map[common.Hash]*Block),
	}
	bc.init()
	return bc
}

func (bc *Blockchain) init() {
	emptyHash := common.Sha256([]byte{})
	// create genesis
	bc.Genesis = &Block{
		Round:      0,
		Seed:       emptyHash.Bytes(),
		ParentHash: emptyHash,
		Author:     common.HashToAddr(emptyHash),
	}
	bc.Add(bc.Genesis)
}

func (bc *Blockchain) Get(hash common.Hash, round uint64) *Block {
	Mu.RLock()
	defer Mu.RUnlock()
	blocks := bc.Blocks[round]
	if blocks != nil {
		return blocks[hash]
	}
	return nil
}

func (bc *Blockchain) GetByRound(round uint64) *Block {
	Mu.RLock()
	defer Mu.RUnlock()
	last := bc.Last
	for round > 0 {
		if last.Round == round {
			return last
		}
		last = bc.Get(last.ParentHash, round-1)
		round--
	}
	return last
}

func (bc *Blockchain) Add(blk *Block) {
	Mu.Lock()
	defer Mu.Unlock()
	blocks := bc.Blocks[blk.Round]
	if blocks == nil {
		blocks = make(map[common.Hash]*Block)
	}
	blocks[blk.Hash()] = blk
	bc.Blocks[blk.Round] = blocks
	if bc.Last == nil || blk.Round > bc.Last.Round {
		bc.Last = blk
	}
}
