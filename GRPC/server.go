package GRPC

import (
	BcGrpc "bcDB_algorand/Proto/blockchain"
	BCData "bcDB_algorand/blockchain/blockchain_data"
	BCTable "bcDB_algorand/blockchain/blockchain_table"
	"bcDB_algorand/cache"
	"bcDB_algorand/txpool"
	"google.golang.org/grpc"
	"log"
	"net"
)

type Server struct{}

// 全局变量
var localDataBlockChain *BCData.BlockChain
var localTableBlockChain *BCTable.BlockChain
var localTxPool *txpool.TxPool
var localCache *cache.Cache

func (s *Server) Init(LDbc *BCData.BlockChain, LTbc *BCTable.BlockChain, TxPool *txpool.TxPool, cache *cache.Cache) {
	localDataBlockChain = LDbc
	localTableBlockChain = LTbc
	localTxPool = TxPool
	localCache = cache
	go s.startService()
}

// 启动Grpc服务端
func (s *Server) startService() {
	// 初始化grpc对象
	grpcServer := grpc.NewServer()
	// 注册服务
	BcGrpc.RegisterBlockChainServiceServer(grpcServer, &Service{})
	// 创建监听
	listen, err := net.Listen("tcp", ":3301")
	if err != nil {
		log.Panic(err)
	}
	defer func(listen net.Listener) {
		err := listen.Close()
		if err != nil {
			log.Panic(err)
		}
	}(listen)

	// 绑定服务
	err = grpcServer.Serve(listen)
	if err != nil {
		log.Panic(err)
	}
}
