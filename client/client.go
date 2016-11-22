package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/4180122/distbayes/bclass"
	"github.com/arcaneiceman/GoVector/govec"
	"github.com/gonum/matrix/mat64"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"
)

var (
	cnum      int = 0
	name      string
	inputargs []string
	myaddr    *net.TCPAddr
	svaddr    *net.TCPAddr
	model     bclass.Model
	logger    *govec.GoLog
	x         *mat64.Dense
	y         *mat64.Dense
	l         *net.TCPListener
	gmodel    bclass.GlobalModel
	gempty    bclass.GlobalModel
)

type message struct {
	Id     int
	Name   string
	Type   string
	C      int
	D      int
	Model  bclass.Model
	GModel bclass.GlobalModel
}

func main() {
	//Parsing inputargs
	parseArgs()

	//Initialize stuff
	model = bclass.RegLSBasisC(x, y, 0.01, 2)
	requestJoin()

	//Initialize TCP Connection and listener
	l, _ = net.ListenTCP("tcp", myaddr)
	go listener()

	//Main function of this server
	for {
		parseUserInput()
	}
}

func listener() {
	for {
		conn, err := l.AcceptTCP()
		checkError(err)
		go connHandler(conn)
	}
}

func connHandler(conn *net.TCPConn) {
	p := make([]byte, 1048576)
	conn.Read(p)
	var msg message
	logger.UnpackReceive("Received message", p, &msg)
	switch msg.Type {
	case "test_request":
		// server is asking me to test
		go testModel(msg.Id, msg.Model)
	case "global_grant":
		// server is sending global model
		gmodel = msg.GModel
		//go testGlobal(msg.GModel)
	default:
		// respond to ping
	}
	conn.Close()
}

func parseUserInput() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter command: ")
	text, _ := reader.ReadString('\n')
	//Windows adds its own strange carriage return, swap the following lines to fix it
	//ident := text[0 : len(text)-2]
	ident := text[0 : len(text)-1]
	switch ident {
	case "read":
		x = readData(inputargs[3])
		y = readData(inputargs[4])
		fmt.Printf("Data updated.\n")
	case "train":
		model = bclass.RegLSBasisC(x, y, 0.01, 2)
		yt := model.Predict(x)
		c, d := bclass.TestResults(yt, y)
		fmt.Printf("Model accuracy is: %v.\n", float64(c)/float64(d))
	case "push":
		yt := model.Predict(x)
		c, d := bclass.TestResults(yt, y)
		requestCommit(c, d)
	case "pull":
		fmt.Printf("Requesting global model from server.\n")
		requestGlobal()
	case "test":
		yt := gmodel.Predict(x)
		c, d := bclass.TestResults(yt, y)
		fmt.Printf("Global model accuracy on local data is: %v.\n", float64(c)/float64(d))
	case "who":
		fmt.Println(name)
	default:
		fmt.Printf("Command not recognized: %v.\n\n", ident)
		fmt.Printf("Choose from the following commands\n")
		fmt.Printf("read  -- Read data from disk\n")
		fmt.Printf("train -- Train model from data (reports error)\n")
		fmt.Printf("push  -- Push trained model to server\n")
		fmt.Printf("pull  -- Obtain global model from server\n")
		fmt.Printf("test  -- Test local data on global model\n")
		fmt.Printf("who   -- Print node name\n\n")
	}
}

func requestJoin() {
	msg := message{cnum, name, myaddr.String(), 0, 0, model, gempty}
	tcpSend(msg)
	fmt.Printf("Asked server to join.\n")
}

func requestCommit(c, d int) {
	cnum++
	msg := message{cnum, name, "commit_request", c, d, model, gempty}
	tcpSend(msg)
	fmt.Printf("Pushed local model to server.\n")
}

func requestGlobal() {
	msg := message{cnum, name, "global_request", 0, 0, model, gempty}
	tcpSend(msg)
}

func testModel(id int, testmodel bclass.Model) {
	fmt.Printf("\nReceived test requset.\nEnter command: ")
	yt := testmodel.Predict(x)
	c, d := bclass.TestResults(yt, y)
	msg := message{id, name, "test_complete", c, d, testmodel, gempty}
	tcpSend(msg)
	fmt.Printf("\nCompleted test requset.\nEnter command: ")
}

//func testGlobal(g bclass.GlobalModel) {
//	gmodel = g
//	yt := model.Predict(x)
//	yg := gmodel.Predict(x)
//	ct, dt := bclass.TestResults(yt, y)
//	cg, dg := bclass.TestResults(yg, y)
//	fmt.Printf("\nModel accuracy: Local (%v), Global (%v).\nEnter command: ", float64(ct)/float64(dt), float64(cg)/float64(dg))
//}

func tcpSend(msg message) {
	conn, err := net.DialTCP("tcp", nil, svaddr)
	checkError(err)
	outbuf := logger.PrepareSend(msg.Type, msg)
	_, err = conn.Write(outbuf)
	checkError(err)
}

func readData(filename string) *mat64.Dense {
	dat, err := ioutil.ReadFile(filename)
	checkError(err)
	array := strings.Split(string(dat), "\n")
	r := len(array) - 1
	temp := strings.Split(array[0], ",")
	c := len(temp)
	vdat := make([]float64, c*r)
	for i := 0; i < r; i++ {
		temp = strings.Split(array[i], ",")
		for j := 0; j < c; j++ {
			vdat[i*c+j], _ = strconv.ParseFloat(temp[j], 64)
		}
	}
	return mat64.NewDense(r, c, vdat)
}

func parseArgs() {
	flag.Parse()
	inputargs = flag.Args()
	var err error
	if len(inputargs) < 2 {
		fmt.Println("Not enough inputs")
		return
	}
	name = inputargs[0]
	myaddr, err = net.ResolveTCPAddr("tcp", inputargs[1])
	checkError(err)
	svaddr, err = net.ResolveTCPAddr("tcp", inputargs[2])
	checkError(err)
	x = readData(inputargs[3])
	y = readData(inputargs[4])
	logger = govec.Initialize(inputargs[0], inputargs[5])
}

func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %s", err.Error())
		os.Exit(1)
	}
}
