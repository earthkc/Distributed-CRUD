// FRONT END FOR
// Quotes, The Best Quotes

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// template initalization
var tpl *template.Template

// Current connection backend node used to transmit/receive data from
//var conn net.Conn

// Read/Write mutex for access to backendConns
var mutex sync.RWMutex

var port string

// Slice that holds the dialed TCP connections
var backendConns []*net.TCPConn

// Directs to the page for adding a new quote
func addQuote(w http.ResponseWriter, r *http.Request) {
	tpl.ExecuteTemplate(w, "newquote.html", nil)
}

func createQuoteConn(temp string) {
	mutex.RLock()
	for _, conn := range backendConns {
		encoder := json.NewEncoder(conn)
		action := "create"
		encoder.Encode(action)
		encoder.Encode(temp)
	}
	mutex.RUnlock()
}

// Sends the newly created quote to the backend
func createQuote() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodPost {
			temp := req.FormValue("quote")
			createQuoteConn(temp)
			http.Redirect(w, req, "/", http.StatusSeeOther)
		}
	}
}

func deleteQuoteConn(id string) {
	mutex.RLock()
	for _, conn := range backendConns {
		encoder := json.NewEncoder(conn)
		action := "delete"
		encoder.Encode(action)
		encoder.Encode(id)
	}
	mutex.RUnlock()
}

// Informs the backend which quote is to be deleted
func deleteQuote() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		id := req.FormValue("id")
		deleteQuoteConn(id)
		http.Redirect(w, req, "/", http.StatusSeeOther)
	}
}

// Loads the quote editing page
func editQuote(w http.ResponseWriter, req *http.Request) {
	id := req.FormValue("id")
	tpl.ExecuteTemplate(w, "editQuote.html", id)
}

func updateQuoteConn(id string, newQuote string) {
	mutex.RLock()
	for _, conn := range backendConns {
		encoder := json.NewEncoder(conn)
		action := "update"
		encoder.Encode(action)
		encoder.Encode(id)
		encoder.Encode(newQuote)

	}
	mutex.RUnlock()

}

// Sends the quote to be edited, and the edits to the backend
func updateQuote() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		id := req.FormValue("id")
		newQuote := req.FormValue("quote")
		updateQuoteConn(id, newQuote)
		http.Redirect(w, req, "/", http.StatusSeeOther)
	}
}

func indexConn() []string {
	var quotes = []string{}
	mutex.RLock()
	for _, conn := range backendConns {
		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)
		action := "index"
		encoder.Encode(action)
		decoder.Decode(&quotes)
		fmt.Println("Indexed")
		fmt.Println(len(quotes)) // error check
	}
	mutex.RUnlock()
	return quotes
}

// Retrieves data from the backend to display the index page

func index() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tpl.ExecuteTemplate(w, "index.html", indexConn())
	}
}

// Removes failed connection from connection array
// Reassigns current datastream connection to a functional node if necessary
// Attempts a new connection to the same address
func failedNodeProcedure(backendAddr *net.TCPAddr, currConn *net.TCPConn, killErrDetect chan bool) {

	// Removes failed connection from array of connnected nodes
	mutex.Lock()
	// Prevents frontend from crashing when there is only one backend node
	if len(backendConns) > 1 {
		var index int
		for i, v := range backendConns {
			if v == currConn {
				index = i
				break
			}
		}
		//mutex.Lock()
		backendConns = backendConns[:index+copy(backendConns[index:], backendConns[index+1:])]
		//mutex.Unlock()
	}
	mutex.Unlock()

	// close connection?

	/*
		// Reassigns working connection if necessary
		if currConn == conn {
			mutex.RLock()
			if len(backendConns) > 1 {
				conn = backendConns[rand.Intn(len(backendConns))]
			}
			mutex.RUnlock()
		}
	*/

	// Re-establishes a connection to the address
	for {
		newconn, err := net.DialTCP("tcp", nil, backendAddr)
		if err == nil {
			// Adds the new connection to the established connections slice
			mutex.Lock()
			backendConns = append(backendConns, newconn)
			mutex.Unlock()

			// Terminates the errorDetect thread that tracked the failed node
			killErrDetect <- true

			// Creates a new thread to detect error on this new connection
			go errDetect(backendAddr, newconn)
			//conn = newconn
			go httpHandler()
			break
		}
	}
	return
}

// Detects error on backend through the use of Ping-Ack
func errDetect(backendaddr *net.TCPAddr, currConn *net.TCPConn) {
	encoder := json.NewEncoder(currConn)
	decoder := json.NewDecoder(currConn)
	var ack string
	killErrDetect := make(chan bool)
	initiatedKillErrDetect := false
	for {
		select {
		case <-killErrDetect:
			return
		default:
			ack = ""
			time.Sleep(1 * time.Second)
			action := "PING"
			encoder.Encode(action)
			decoder.Decode(&ack)
			if ack != "ACK" {
				// Prints out error message
				fmt.Printf("Detected failure on ")
				if backendaddr.IP == nil {
					fmt.Printf("localhost")
				} else {
					fmt.Print(backendaddr.IP)
				}
				fmt.Printf(":%d at ", backendaddr.Port)
				fmt.Println(time.Now())

				// Attempts to reconnect on the same address/port
				if initiatedKillErrDetect == false {
					initiatedKillErrDetect = true
					go failedNodeProcedure(backendaddr, currConn, killErrDetect)
				}
			}
			time.Sleep(4 * time.Second)
		}
	}
}

// http Handler functions
func httpHandler() {

	fmt.Println("httpHandler ENTRY") // error printf

	mux := http.NewServeMux()
	mux.Handle("/", index())
	mux.HandleFunc("/addQuote", addQuote)
	mux.Handle("/createQuote", createQuote())
	mux.Handle("/deleteQuote", deleteQuote())
	mux.HandleFunc("/editQuote", editQuote)
	mux.Handle("/updateQuote", updateQuote())

	//http.HandleFunc("/", index())
	//http.HandleFunc("/addQuote", addQuote)
	//http.HandleFunc("/createQuote", createQuote())
	//http.HandleFunc("/deleteQuote", deleteQuote())
	//http.HandleFunc("/editQuote", editQuote)
	//http.HandleFunc("/updateQuote", updateQuote())

	// connects to provided or default HTTP port
	// Default: localhost:8080
	http.ListenAndServe(port, mux)
}

func connWatch() {
	for {
		time.Sleep(1 * time.Second)
		mutex.RLock()
		fmt.Printf("Length of conn slice: %d\n", len(backendConns))
		mutex.RUnlock()
		time.Sleep(1 * time.Second)
	}
}

// template initalization directory
func init() {
	tpl = template.Must(template.ParseGlob("templates/*"))
}

func main() {
	// HTTP listen flag
	var portnum int
	flag.IntVar(&portnum, "listen", 8080, "HTTP port")

	// TCP port flag
	var tcpport string
	flag.StringVar(&tcpport, "backend", ":8090", "backend TCP port")

	flag.Parse()

	// Converts HTTP flag into a string
	p := strconv.Itoa(portnum)
	port = ":" + p

	// Array that holds the backend TCP ports
	backendNodes := strings.Split(tcpport, ",")

	// Slice that holds the resolved backend TCP addresses
	var backendAddrs []*net.TCPAddr
	for _, v := range backendNodes {
		temp, _ := net.ResolveTCPAddr("tcp", v)
		backendAddrs = append(backendAddrs, temp)
	}

	// Channel receives a value when backend does not respond
	// Used to indicate when to reconnect
	//errorChan := make(chan bool)

	// Boolean that indicates the HTTP Handler functions are initiated
	//handeled := false

	// Channel indicates when the backend error alert thread should end
	//endErrorChan := make(chan bool)

	// Waitgroup to wait for the previous backend alert thread to end
	//var wg sync.WaitGroup

	// Slice that holds the dialed TCP connections
	//var backendConns []*net.TCPConn

	// Initial connections to TCP
	// Starts detection failure threads for each node
	// Returns error and ends program if one backend in the parameters is not avaliable
	for _, v := range backendAddrs {
		tempconn, err := net.DialTCP("tcp", nil, v)
		if err != nil {
			fmt.Println("A backend server could not be connected to. Ending program.")
			return
		}
		go errDetect(v, tempconn)
		mutex.Lock()
		backendConns = append(backendConns, tempconn)
		mutex.Unlock()
	}

	go connWatch()

	// *****************************
	// Initializes http Handler functions
	/*
		for _, v := range backendConns {
			conn = v
			go httpHandler()
		}
	*/
	go httpHandler()

	// Randomly selects one of the avaliable backend connections to be
	// used for current interactions
	/*
		rand.Seed(time.Now().UnixNano())
		mutex.RLock()
		conn = backendConns[rand.Intn(len(backendConns))]
		mutex.RUnlock()
	*/

	// Prevents main from ending
	badChan := make(chan int)

	<-badChan

}
