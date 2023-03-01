package blockqueue

import (
	"sync"
)

// 使用队列保存未同步的区块

var LocalDataBlockQueue *Queue
var LocalTableBlockQueue *Queue

// 与algorand交互的队列

var DataBlock2Algorand *Queue
var TableBlock2Algorand *Queue
var Algorand2DataBlock *Queue
var Algorand2TableBlock *Queue

// 定义节点
type node struct {
	data interface{}
	next *node
}

// Queue 定义队列的结构
type Queue struct {
	Head       *node // 头节点
	Rear       *node // 尾节点
	Size       int   //大小
	sync.Mutex       // 锁
}

// Init 队列初始化
func Init() *Queue {
	q := new(Queue)
	q.Head = nil
	q.Rear = nil
	q.Size = 0
	return q
}

// Put 尾插法
func (q *Queue) Put(element interface{}) {
	//log.Println("Queue put begin!")
	n := new(node)
	n.data = element
	q.Lock()
	defer q.Unlock()

	if q.Rear == nil {
		q.Head = n
		q.Rear = n
	} else {
		q.Rear.next = n
		q.Rear = n
	}
	q.Size++
	//log.Println("Queue put end!", q.Size)
}

//// PutHead 头插法，在队列头部插入一个元素
//func (q *Queue) PutHead(element interface{}) {
//	n := new(node)
//	n.data = element
//	q.Lock()
//	defer q.Unlock()
//	if q.Head == nil {
//		q.Head = n
//		q.Rear = n
//	} else {
//		n.next = q.Head
//		q.Head = n
//	}
//	q.Size++
//}

// Get 获取并删除队列头部的元素
func (q *Queue) Get() interface{} {
	//log.Println("Queue get begin!")
	if q.Head == nil {
		return nil
	}
	n := q.Head
	q.Lock()
	defer q.Unlock()
	// 代表队列中仅一个元素
	if n.next == nil {
		q.Head = nil
		q.Rear = nil

	} else {
		q.Head = n.next
	}
	q.Size--
	//log.Println("Queue get end!", q.Size)
	return n.data
}
