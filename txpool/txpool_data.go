package txpool

import (
	"bcDB_algorand/algorand"
	"bcDB_algorand/blockchain/blockchain_data"
	"bcDB_algorand/blockqueue"
	"bcDB_algorand/cache"
	"errors"
	"fmt"
	"sync"
	"time"
)

type txNode struct {
	tx   blockchain_data.Transaction
	pre  *txNode
	next *txNode
}

type txQueue struct {
	curSize int
	maxSize int
	txMap   map[string]*txNode // map[txID]*txNode
	head    *txNode
	tail    *txNode
}

func (q *txQueue) init() {
	q.curSize = 0
	q.maxSize = 1000000

	q.txMap = make(map[string]*txNode)
	p1 := new(txNode)
	p2 := new(txNode)
	q.head = p1
	q.tail = p2
	p1.next = p2
	p2.pre = p1
}

func (q *txQueue) in(transaction blockchain_data.Transaction) {
	p := &txNode{
		tx:   transaction,
		pre:  nil,
		next: nil,
	}

	// 找到合适的位置, 自己的时间戳是要大于前面节点的时间戳的
	tsp := p.tx.TimeStamp
	p1 := q.tail.pre
	for p1.pre != nil && p1.tx.TimeStamp > tsp { // p1 不是头节点，并且p1的时间靠后
		p1 = p1.pre
	} // result p1 时间靠前，或者是头节点

	p.next = p1.next
	p1.next.pre = p
	p.pre = p1
	p1.next = p

	q.txMap[string(p.tx.TxID)] = p
	q.curSize++
}

func (q *txQueue) out(transaction blockchain_data.Transaction) {

	p := q.txMap[string(transaction.TxID)]
	p.pre.next = p.next
	p.next.pre = p.pre

	delete(q.txMap, string(transaction.TxID))
	q.curSize--
}

// 记账节点的动作。 定时在等待队列中拿取打包

type TxPoolData struct {
	sync.Mutex //锁

	txQueue txQueue // 交易队列

	packNumber   int     // 打包数量
	count        int     // 交易的数量
	countPointer *txNode // 指向交易的指针

	chain *blockchain_data.BlockChain // 数据链
	cache *cache.Cache                // 缓存
}

func (tpl *TxPoolData) init(chain *blockchain_data.BlockChain, cache *cache.Cache) {
	tpl.Lock()
	defer tpl.Unlock()

	tpl.txQueue.init()

	tpl.packNumber = 100
	tpl.count = 0
	tpl.countPointer = tpl.txQueue.head
	tpl.chain = chain
	tpl.cache = cache

}

// 交易的打包
func (tpl *TxPoolData) bookKeeperRun() {
	tpl.Lock()
	defer tpl.Unlock()

	tspNow := time.Now().Unix()
	tspStand := tspNow - 2 //时间戳2s前

	// 检查等待队列的交易的时间戳，统计时间合法的交易。
	// 从head开始计数时间戳符合要求的节点数量
	tpl.count = 0
	tpl.countPointer = tpl.txQueue.head.next
	for tpl.countPointer.next != nil && tpl.countPointer.tx.TimeStamp < tspStand {
		tpl.countPointer = tpl.countPointer.next
		tpl.count++
	} // result: cp-> 尾节点，或者第一个时间戳没有达到延迟要求的节点

	// 如果数量满足打包的要求，将这些交易批量打包。
	for tpl.count >= tpl.packNumber {

		var txs []*blockchain_data.Transaction
		p := tpl.txQueue.head.next // start: p.第一个要被打包的元素
		for i := 0; i < tpl.packNumber; i++ {
			txs = append(txs, &p.tx)
			delete(tpl.txQueue.txMap, string(p.tx.TxID))
			tpl.txQueue.curSize--

			p = p.next
		} // result: p指向一个不被打包的元素。或者刚好打包完，指向尾节点。

		// 区块的打包
		block := blockchain_data.NewBlock()
		block.InitBlock(txs, tpl.chain.TailHash, tpl.chain.LastID)
		tpl.chain.LastID++
		//tpl.OrdinaryRun(block)
		// 进行algorand过程
		// 1. 将区块传入同步队列
		blockqueue.DataBlock2Algorand.Put(block)
		// 2. 将算法输出的区块的内容与传入的区块进行合并,需要判断队列是否有数据
		for {
			time.AfterFunc(10*time.Second, func() {
				if blockqueue.Algorand2DataBlock.Size != 0 {
					get := blockqueue.Algorand2DataBlock.Get()
					algblock := get.(algorand.Block)

					// 区块的统一
					block.Author = algblock.Author
					block.AuthorVRF = algblock.AuthorVRF
					block.AuthorProof = algblock.AuthorProof
					block.Seed = algblock.Seed
					block.Proof = algblock.Proof
					block.Signature = algblock.Signature

					// 区块上链
					tpl.chain.AddBlockToChain(block)
					fmt.Println("生成一个新的数据区块", time.Now().String())

					// 更新本地缓存
					tpl.cache.UpdateByDataBlock(block)

					// TODO 分发区块
					//GRPC.DataBlockDistribute(&block)
					blockqueue.LocalDataBlockQueue.Put(block)

					tpl.txQueue.head.next = p
					p.pre = tpl.txQueue.head

					tpl.count -= tpl.packNumber
				}
			})
		}

	}
}

// OrdinaryRun 普通节点在接受到记账节点，发过来的区块时。根据区块里面的交易，删除自己交易池里面对应的交易。
func (tpl *TxPoolData) OrdinaryRun(block blockchain_data.Block) {
	tpl.Lock()
	defer tpl.Unlock()

	for _, tx := range block.Transactions {
		tpl.txQueue.out(*tx)
	}
}

// TxIn 被校验过的交易进入交易池
// 对时间戳进行检查，找到合适的位置并插入到队列里面。
// 记账节点： 接受其他节点和自己的交易进入交易池
// 普通节点： 接受记账节点传过来的交易然后进入交易池
func (tpl *TxPoolData) TxIn(transaction blockchain_data.Transaction) error {
	//tpl.Lock()
	//defer tpl.Unlock()

	if tpl.txQueue.curSize == tpl.txQueue.maxSize {
		return errors.New("FULL")
	}

	tpl.txQueue.in(transaction)

	return nil
}
