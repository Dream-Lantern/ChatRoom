package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

// map 的 value值
type clientC struct {
	Ch   chan string
	Name string
}

// 用户登录， 退出， 发送信息的  channel
var login chan clientC = make(chan clientC)

var message chan string = make(chan string)

var logout chan clientC = make(chan clientC)

// 存储 人员信息的 map . key值是 用户名称（进程ID）
var clientInfo map[string]clientC

// 通过 无缓冲 channel完成 通信及同步
func manager() {

	clientInfo = make(map[string]clientC)

	for {
		select {
		case cli := <-login:
			//clientInfo[cli] = true
			clientInfo[cli.Name] = cli
		case cli := <-logout:
			delete(clientInfo, cli.Name)
		case msg := <-message:
			// 遍历 存在的 客户端
			for _, cli := range clientInfo {
				cli.Ch <- msg //向 客户端 发送 消息
			}
		}
	}
}

// 回写给 客户端
func ReadFromClient(conn net.Conn, cli clientC) {
	for msg := range cli.Ch {
		conn.Write([]byte(msg + "\n"))
	}
}

// 查询人数
func showMember(conn net.Conn) {
	fmt.Fprintf(conn, "聊天室人数:%d\n", len(clientInfo))
	for _, mem := range clientInfo {
		fmt.Fprintf(conn, mem.Name)
		fmt.Fprintf(conn, "\n")
	}
}

// 悄悄话
//TO|1234|helllo
func secretComm(myName string, commText string) {
	commName := strings.Split(commText, "|")[1]
	// 利用 map完成 指定 通话
	clientInfo[commName].Ch <- commName + "悄悄对你说: " + strings.Split(commText, "|")[2]
}

// communication
func HandleConn(conn net.Conn) {
	defer conn.Close()

	myName := conn.RemoteAddr().String()
	myName = strings.Split(myName, ":")[1]

	myCli := clientC{make(chan string), myName}

	// 回写给 客户端
	go ReadFromClient(conn, myCli)

	// login
	login <- myCli
	message <- "心悦会员" + myName + "登录"

	// 分析状态的 管道
	//myFire := make(chan int)
	myTalk := make(chan int)
	myQuit := make(chan int)

	// 定时器
	myTicker := time.NewTicker(time.Second * 20)

	// communication 匿名函数
	go func() {
		myScanner := bufio.NewScanner(conn)

		for myScanner.Scan() {
			commText := myScanner.Text()
			if commText == "who" {
				showMember(conn)
			} else if len(commText) > 2 && commText[:2] == "TO" {
				secretComm(myName, commText)
			} else {
				message <- myName + ": " + commText
			}
			myTalk <- 1
		}
		// false 说明客户端主动退出， 没有继续通话
		myQuit <- 1
	}()

	// select 监听 客户端状态
	for {
		select {
		case <-myTicker.C:
			// 超时 被踢
			conn.Write([]byte("超时被T\n"))
			logout <- myCli
			message <- "心悦会员" + myName + "长时间不活跃，已强制踢除"
			return
		case <-myQuit:
			// 主动 退出
			logout <- myCli
			message <- "心悦会员" + myName + "退出"
		case <-myTalk:
			// 正常 通话
			myTicker.Stop()
			myTicker = time.NewTicker(time.Second * 20)
		}
	}
}

func main() {

	lnFd, errln := net.Listen("tcp", ":9000")
	if errln != nil {
		log.Println(errln)
		return
	}
	defer lnFd.Close()

	// 通过 select 处理 channel 通信
	go manager()

	for {
		conn, errConn := lnFd.Accept()
		if errConn != nil {
			log.Println(errConn)
			continue
		}
		// communication
		go HandleConn(conn)
	}
}
