package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
	"golang.org/x/text/runes"
)

type Node struct {
	token   string
	agent   *Agent
	conn    *net.Conn
	active  bool
	session *ssh.Session
	stdin   io.WriteCloser
	stdout  io.Reader
	client  *ssh.Client
	lock    bool
}

type Agent struct {
	password string
	username string
	conn     *net.Conn
}

var (
	nodes                     map[string]Node
	agents                    map[string]Agent
	connectionBufferReadSize  int
	connectionBufferWriteSize int
	upgrader                  websocket.Upgrader
	agentTerminalWaitTimeout  = 5
)

func node(conn net.Conn) {
	var (
		size int32
		buf  []byte
	)
	err := binary.Read(conn, binary.LittleEndian, &size)
	if err != nil {
		fmt.Println("Token size read error:", err)
	}
	buf = make([]byte, size)
	_, err = conn.Read(buf)
	if err != nil {
		fmt.Println("Token read error:", err)
	}

	message := string(buf[:size])
	data := bytes.Split([]byte(message), []byte(":"))
	id := string(data[0])
	token := string(data[1])

	log.Println("New terminal socket, token:", token)

	agent, ok := agents[id]

	if !ok {
		conn.Close()
		return
	}

	nodes[token] = Node{
		agent:  &agent,
		token:  token,
		conn:   &conn,
		active: false,
		lock:   false,
	}
}

func agent(conn net.Conn) {
	var (
		size int32
		buf  []byte
	)
	err := binary.Read(conn, binary.LittleEndian, &size)
	if err != nil {
		fmt.Println("Message size read error:", err)
	}
	buf = make([]byte, size)
	_, err = conn.Read(buf)
	if err != nil {
		fmt.Println("Message read error:", err)
	}

	message := string(buf[:size])
	data := bytes.Split([]byte(message), []byte(":"))
	id := string(data[0])
	username := string(data[1])
	password := string(data[2])

	log.Println("New agent, id:", id)

	agents[id] = Agent{
		username: username,
		password: password,
		conn:     &conn,
	}

	//go func() {
	//	defer conn.Close()
	//	buf = make([]byte, 4)
	//	for {
	//		_, err := conn.Read(buf)
	//		if err != nil {
	//			log.Println("Broken agent connection, id:", id, "deleting...")
	//			break
	//		}
	//	}
	//	delete(agents, id)
	//}()
}

func setupResponse(w *http.ResponseWriter, req *http.Request) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

func terminal(w http.ResponseWriter, r *http.Request) {
	setupResponse(&w, r)

	var closeTerminal = make(chan int)

	vars := mux.Vars(r)
	token := vars["token"]
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("ERROR: gorilla WebSoket create error:", err)
		w.WriteHeader(500)
		return
	}

	node, ok := nodes[token]

	for i := 0; !ok && i < agentTerminalWaitTimeout; i++ {
		time.Sleep(1000000000)
		node, ok = nodes[token]
	}

	if !ok {
		log.Println("No node by token:", token)
		w.WriteHeader(500)
		conn.Close()
		return
	}

	stdout := nodes[token].stdout
	stdin := nodes[token].stdin

	go func() {
		size := connectionBufferReadSize
		var buf []byte
		buf = make([]byte, size+4)
		i := 0
		for {
			n, err := stdout.Read(buf[i:size])
			if err != nil {
				log.Println("SSH read error:", err)
				closeTerminal <- 1
				return
			}
			j := 0
			r, _ := utf8.DecodeLastRune(buf[:size])
			for r == utf8.RuneError {
				j++
				r, _ = utf8.DecodeLastRune(buf[:size-j])
			}
			end := n + i - j

			if !utf8.Valid(buf[:end]) {
				newBuf := runes.ReplaceIllFormed().Bytes(buf[:end])
				end = len(newBuf)
				copy(buf, newBuf)
			}

			err = conn.WriteMessage(websocket.TextMessage, buf[:end])
			if err != nil {
				log.Println("WebSocket write error:", err)
				closeTerminal <- 1
				return
			}
			i = 0
			if j > 0 {
				k := 0
				for ; k < j; k++ {
					buf[k] = buf[end+k]
				}
				i = k
			}
			//time.Sleep(10)
		}
	}()

	go func() {
		size := connectionBufferWriteSize
		var buf []byte
		buf = make([]byte, size+4)

		for {
			_, reader, err := conn.NextReader()
			if err != nil {
				log.Println("WebSocket NextReader error:", err)
				closeTerminal <- 1
				return
			}
			n, err := reader.Read(buf)
			for err != io.EOF {
				if err != nil {
					log.Println("WebSocket NextReader read error:", err)
					closeTerminal <- 1
					return
				}
				_, err = stdin.Write([]byte(string(buf[:n])))
				if err != nil {
					log.Println("SSH write error:", err)
					closeTerminal <- 1
					return
				}
				n, err = reader.Read(buf)
			}
			//time.Sleep(10)
		}
	}()

	<-closeTerminal
	node = nodes[token]
	err = node.client.Close()
	log.Println(err)
	err = (*node.conn).Close()
	log.Println(err)
	delete(nodes, token)
}

func terminalRegister(w http.ResponseWriter, r *http.Request) {
	setupResponse(&w, r)

	id := r.URL.Query().Get("id")
	if node, ok := nodes[id]; ok {
		if node.lock {
			w.Write([]byte(id))
			return
		}
		node.lock = true
		nodes[id] = node

		errorExit := func() {
			w.WriteHeader(500)
			if node.client != nil {
				node.client.Close()
			}
			(*node.conn).Close()
			delete(nodes, id)
		}

		cols, rows := r.URL.Query().Get("cols"), r.URL.Query().Get("rows")
		colsInt, err := strconv.Atoi(cols)
		if err != nil {
			log.Println("String convert error (cols):", err)
			errorExit()
			return
		}

		rowsInt, err := strconv.Atoi(rows)
		if err != nil {
			log.Println("String convert error (rows):", err)
			errorExit()
			return
		}

		if !node.active {
			sshConfig := &ssh.ClientConfig{
				User: (*node.agent).username,
				Auth: []ssh.AuthMethod{ssh.Password((*node.agent).password)},
			}
			sshConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()

			c, ch, rq, err := ssh.NewClientConn(*node.conn, "", sshConfig)
			if err != nil {
				log.Println("SSH:", err)
				errorExit()
				return
			}
			client := ssh.NewClient(c, ch, rq)
			node.client = client
			nodes[id] = node
			session, err := client.NewSession()
			if err != nil {
				log.Println("SSH can`t create new session:", err)
				errorExit()
				return
			}

			node.session = session
			node.stdin, err = session.StdinPipe()
			if err != nil {
				log.Println("SSH can`t get stdin:", err)
				errorExit()
				return
			}
			node.stdout, err = session.StdoutPipe()
			if err != nil {
				log.Println("SSH can`t get stdout:", err)
				errorExit()
				return
			}
			if err := node.session.RequestPty("xterm-256color", rowsInt, colsInt, ssh.TerminalModes{}); err != nil {
				log.Println("Request for pseudo terminal failed:", err)
				return
			}

			if err := node.session.Shell(); err != nil {
				log.Println("Failed to start shell:", err)
				errorExit()
				return
			}

			node.active = true
		} else {
			err := node.session.WindowChange(rowsInt, colsInt)
			if err != nil {
				log.Println("Failed to change terminal size:", err)
				errorExit()
				return
			}
		}

		w.Write([]byte(id))
		nodes[id] = node
	}
}

func createTermminal(w http.ResponseWriter, r *http.Request) {
	setupResponse(&w, r)

	vars := mux.Vars(r)
	id := vars["id"]

	errorFunc := func() {
		w.WriteHeader(500)
		w.Write([]byte("error"))
	}

	if agent, ok := agents[id]; ok {
		conn := *agent.conn
		_, err := conn.Write([]byte("get-token"))
		if err != nil {
			log.Println("Failed to send command to agent:", err)
			errorFunc()
			return
		}
		var (
			size int32
			buf  []byte
		)
		err = binary.Read(conn, binary.LittleEndian, &size)
		if err != nil {
			fmt.Println("Token size read error from agent:", err)
		}
		buf = make([]byte, size)
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Println("Token read error from agent:", err)
		}
		w.Write(buf[:n])
	} else {
		errorFunc()
	}
}

func agentsList(w http.ResponseWriter, r *http.Request) {
	setupResponse(&w, r)

	keys := make([]string, len(agents))
	i := len(agents) - 1
	for k := range agents {
		keys[i] = k
		i--
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(keys); err != nil {
		w.WriteHeader(500)
	}
}

func httpServer() {
	webServerAddr := os.Getenv("FRONTEND_PORT")
	if webServerAddr != "" {
		webServerAddr = ":" + webServerAddr
	} else {
		webServerAddr = fmt.Sprintf(":%d", 4000)
	}

	log.Println("Listening http: \t", webServerAddr)
	router := mux.NewRouter()
	router.HandleFunc("/terminal-config", terminalRegister).Methods("POST")
	router.HandleFunc("/terminal/{token}", terminal).Methods("GET")
	router.HandleFunc("/create-terminal/{id}", createTermminal).Methods("GET")
	router.HandleFunc("/agents-list", agentsList).Methods("GET")
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	s := http.StripPrefix("/", http.FileServer(http.Dir(dir+"/static/")))
	router.PathPrefix("/").Handler(s)
	http.Handle("/", router)
	http.ListenAndServe(webServerAddr, nil)
}

func main() {
	rand.Seed(time.Now().UnixNano())
	var (
		quit = make(chan int)
		err  error
	)

	readBuf := os.Getenv("READ_BUFFER_SIZE")
	if readBuf != "" {
		connectionBufferReadSize, err = strconv.Atoi(readBuf)
		if err != nil {
			log.Fatalln("READ_BUFFER_SIZE env convert to int error:", err)
		}
		if connectionBufferReadSize < 4 {
			connectionBufferReadSize = 4
		}
	} else {
		connectionBufferReadSize = 1024
	}

	writeBuf := os.Getenv("WRITE_BUFFER_SIZE")
	if writeBuf != "" {
		connectionBufferWriteSize, err = strconv.Atoi(writeBuf)
		if err != nil {
			log.Fatalln("WRITE_BUFFER_SIZE env convert to int error:", err)
		}
		if connectionBufferWriteSize < 4 {
			connectionBufferWriteSize = 4
		}
	} else {
		connectionBufferWriteSize = 1024
	}

	timeout := os.Getenv("AGENT_TERMINAL_WAIT_TIMEOUT")
	if timeout != "" {
		agentTerminalWaitTimeout, err = strconv.Atoi(timeout)
		if err != nil {
			log.Fatalln("AGENT_TERMINAL_WAIT_TIMEOUT env convert to int error:", err)
		}
	} else {
		agentTerminalWaitTimeout = 5
	}

	debugStateStr := os.Getenv("DEBUG")
	debugging := false
	if debugStateStr == "1" || debugStateStr == "true" {
		debugging = true
	}

	if debugging {
		log.Println("SSH port: ", os.Getenv("SSH_PORT"))
		log.Println("Frontend port: ", os.Getenv("FRONTEND_PORT"))
		log.Println("Agent port: ", os.Getenv("AGENT_PORT"))
		log.Println("Agent terminal wait timeout: ", agentTerminalWaitTimeout)
		log.Println("Write buffer size: ", connectionBufferWriteSize)
		log.Println("Read buffer size: ", connectionBufferReadSize)
	}

	upgrader = websocket.Upgrader{
		ReadBufferSize:  connectionBufferReadSize,
		WriteBufferSize: connectionBufferWriteSize,
	}

	go httpServer()
	go func() {
		nodes = make(map[string]Node)
		agents = make(map[string]Agent)

		sshAddr := os.Getenv("SSH_PORT")
		if sshAddr != "" {
			sshAddr = ":" + sshAddr
		} else {
			sshAddr = fmt.Sprintf(":%d", 8888)
		}

		listener, err := net.Listen("tcp", sshAddr)
		if err != nil {
			log.Fatalf("Failed to setup listener: %v", err)
		}

		log.Println("Listening ssh node: ", sshAddr)

		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Fatalf("ERROR: failed to accept listener: %v", err)
			}
			log.Printf("Accepted connection %v\n", conn)
			go node(conn)
		}
	}()

	go func() {
		agentAddr := os.Getenv("AGENT_PORT")
		if agentAddr != "" {
			agentAddr = ":" + agentAddr
		} else {
			agentAddr = fmt.Sprintf(":%d", 9999)
		}

		listener, err := net.Listen("tcp", agentAddr)
		if err != nil {
			log.Fatalf("Failed to setup listener: %v", err)
		}

		log.Println("Listening agent:\t", agentAddr)

		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Fatalf("ERROR: failed to accept listener: %v", err)
			}
			log.Printf("Accepted connection %v\n", conn)
			go agent(conn)
		}
	}()
	<-quit
}
