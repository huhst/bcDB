package serverExec

import (
	"alg_bcDB/server"
	"alg_bcDB/serverExec/service"
	"alg_bcDB/util"
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
)

//监听网络地址端口
//const (
//	HOST = "192.168.241.128"
//	PORT = "8888"
//)

var RPCs *server.Server
var UID string

//定义接口
type ExecCommandService interface {
	Cmd(ctx context.Context, cmd *service.CommandRequest) (*service.CommandReply, error)
	//服务器流
	StreamServer(req *service.StreamReq, res service.Server_StreamServerServer) error
	//客户端流
	StreamClient(res service.Server_StreamClientServer) error
	//双向流
	StreamTwo(res service.Server_StreamTwoServer) error
	MustEmbedUnimplementedServerServer()
}

//继承
type Exec struct {
	service.UnimplementedServerServer
}

func NewExecCommandService() ExecCommandService {
	return &Exec{}
}

//实现接口
func (exec *Exec) Cmd(ctx context.Context, cmd *service.CommandRequest) (*service.CommandReply, error) {

	//s := &server.Server{}

	var responses *service.CommandReply
	//连接服务器验证密码
	if cmd.ServerPassword != "" {
		if cmd.ServerPassword == "123" {
			responses = &service.CommandReply{
				ServerConnect: "True",
			}
		} else {
			responses = &service.CommandReply{
				ServerConnect: "False",
			}
		}
	}

	//登录验证
	if cmd.LoginAccount != "" {
		isLoginSuccess := RPCs.SignInAccount(cmd.LoginAccount, cmd.LoginPassword)
		if isLoginSuccess {
			UID = cmd.LoginAccount + "-QAQ-" + cmd.LoginPassword
			tabelNames := RPCs.MyTable(UID) //将共享表表名传过去
			responses = &service.CommandReply{
				Login:      "True",
				TabelNames: tabelNames,
			}
		} else {
			responses = &service.CommandReply{
				Login: "False",
			}
		}
	}

	//注册验证
	if cmd.RegisterAccount != "" {
		isRegisterSuccess := RPCs.RegisterAccount(cmd.RegisterAccount, cmd.RegisterPassword)
		if isRegisterSuccess {
			responses = &service.CommandReply{
				Register: "True",
			}
		} else {
			responses = &service.CommandReply{
				Register: "False",
			}
		}
	}

	//获取用户信息
	if cmd.ViewMyinfo != "" {
		isViewMyinfoSuccess := RPCs.CheckAccount(cmd.Uid)
		if isViewMyinfoSuccess != "" {
			responses = &service.CommandReply{
				Uaddress: isViewMyinfoSuccess,
			}
		} else {
			responses = &service.CommandReply{
				Uaddress: "False",
			}
		}
	}

	//注销用户
	if cmd.LogoutIng != "" {
		isLogoutSuccess := RPCs.SignOutAccount(cmd.Uid)
		if isLogoutSuccess {
			responses = &service.CommandReply{
				LogoutOk: "True",
			}
		} else {
			responses = &service.CommandReply{
				LogoutOk: "False",
			}
		}
	}

	//创建表
	if cmd.TabelName != "" && cmd.PermissionTable != "" {
		permissionTable := strings.Split(cmd.PermissionTable, ",")
		isCreatetabelSuccess := RPCs.Table(cmd.Uid, cmd.TabelName, permissionTable)
		time.Sleep(time.Second * 5)
		if isCreatetabelSuccess {
			tabelNames := RPCs.MyTable(cmd.Uid) //将共享表表名传过去
			responses = &service.CommandReply{
				TabelOk:    "True",
				TabelNames: tabelNames,
			}
		} else {
			responses = &service.CommandReply{
				TabelOk: "False",
			}
		}
	}

	//加入数据
	if cmd.Key != "" && cmd.Value != "" && cmd.TabelName != "" {
		isPutSuccess := RPCs.Put(cmd.Uid, cmd.Key, cmd.Value, cmd.TabelName)
		if isPutSuccess {
			responses = &service.CommandReply{
				PutOk: "True",
			}
		} else {
			responses = &service.CommandReply{
				PutOk: "False",
			}
		}
		fmt.Println("回复：", responses)
	}

	//获取数据
	if cmd.Key != "" && cmd.TabelName != "" && cmd.Value == "" {
		fmt.Printf("cmd.Uid: %v\n", cmd.Uid)
		getValue := RPCs.Get(cmd.Uid, cmd.Key, cmd.TabelName)
		if getValue != "" {
			responses = &service.CommandReply{
				GetOk: "True",
				Value: getValue,
			}
		} else {
			responses = &service.CommandReply{
				GetOk: "False",
			}
		}
	}

	//responses := &service.CommandReply{
	//	ServerConnect: "The command is: " + cmd.ClientIp + " " + cmd.ServerPassword + ", tips from golang server",
	//	Login:         cmd.LoginAccount + " " + cmd.LoginPassword,
	//	Register:      cmd.RegisterAccount + " " + cmd.RegisterPassword,
	//	SearchResult:  cmd.SearchContent,
	//}
	fmt.Println(cmd)
	//获取 cmd.Command 命令，对其进行处理
	return responses, nil
}

//服务器流
func (exec *Exec) StreamServer(req *service.StreamReq, res service.Server_StreamServerServer) error {
	//打印客户端请求
	fmt.Println(req.Data)
	i := 0
	for {
		i++
		if i > 2 {
			break
		}
		//给客户端发送消息
		err := res.Send(&service.StreamRes{
			Data: fmt.Sprintf("%v", time.Now().Unix()),
		})
		if err != nil {
			fmt.Println(err)
		}
		time.Sleep(time.Second)
	}
	return nil //返回则终止请求
}

//客户端流
func (exec *Exec) StreamClient(res service.Server_StreamClientServer) error {
	i := 0
	for {
		if i > 2 {
			break
		}
		r, err := res.Recv()
		if err != nil {
			fmt.Println(err)
			break
		} else {
			fmt.Println(r.Data)
		}
		i++
	}
	//发送信息给客户端
	res.SendMsg(&service.StreamRes{
		Data: "客户流截至",
	})
	return nil //返回则终止请求
}

//双向流
func (exec *Exec) StreamTwo(res service.Server_StreamTwoServer) error {
	wg := sync.WaitGroup{}
	wg.Add(2)

	//接受客户端消息
	go func() {
		defer wg.Done()
		i := 0
		for {
			if i > 20 {
				break
			}
			RecData, err := res.Recv()
			if err != nil {
				fmt.Println(err)
			}
			fmt.Println("收到客户端的消息：", RecData.Data)
			i++
		}

	}()

	//给客户端发送消息
	go func() {
		defer wg.Done()
		i := 0
		for {
			if i > 20 {
				break
			}
			err := res.Send(&service.StreamRes{Data: "我是服务器 " + strconv.Itoa(i)})
			if err != nil {
				fmt.Println(err)
			}
			time.Sleep(time.Second)
			i++
		}

	}()
	wg.Wait()
	return nil
}

func (exec *Exec) MustEmbedUnimplementedServerServer() {}

func ServerStart() {
	//localhost := string(GetOutboundIP())
	localhost := util.GetLocalIp1()
	l, err := net.Listen("tcp", localhost+":8888") //开启监听
	//fmt.Println(l.Addr().String())

	if err != nil {
		log.Panic(err)
	}
	fmt.Println("Listen on " + localhost + ":8888")

	grpcService := grpc.NewServer()

	var ExecCommandService ExecCommandService //声明接口

	ExecCommandService = NewExecCommandService()
	service.RegisterServerServer(grpcService, ExecCommandService) //注册服务

	fmt.Println("开启GRPC")
	err = grpcService.Serve(l) //开启服务
	if err != nil {
		println(err)
	}
}
