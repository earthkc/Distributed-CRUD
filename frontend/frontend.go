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
	"sync"
	"time"
)

// template initalization
var tpl *template.Template

var conn net.Conn

// Directs to the page for adding a new quote
func addQuote(w http.ResponseWriter, r *http.Request) {
	tpl.ExecuteTemplate(w, "newquote.html", nil)
}

// Sends the newly created quote to the backend
func createQuote() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodPost {
			temp := req.FormValue("quote")
			encoder := json.NewEncoder(conn)
			action := "create"
			encoder.Encode(action)
			encoder.Encode(temp)
			http.Redirect(w, req, "/", http.StatusSeeOther)
		}
	}
}

// Informs the backend which quote is to be deleted
func deleteQuote() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		id := req.FormValue("id")
		encoder := json.NewEncoder(conn)
		action := "delete"
		encoder.Encode(action)
		encoder.Encode(id)
		http.Redirect(w, req, "/", http.StatusSeeOther)
	}
}

// Loads the quote editing page
func editQuote(w http.ResponseWriter, req *http.Request) {
	id := req.FormValue("id")
	tpl.ExecuteTemplate(w, "editQuote.html", id)
}

// Sends the quote to be edited, and the edits to the backend
func updateQuote() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		id := req.FormValue("id")
		newQuote := req.FormValue("quote")
		encoder := json.NewEncoder(conn)
		action := "update"
		encoder.Encode(action)
		encoder.Encode(id)
		encoder.Encode(newQuote)
		http.Redirect(w, req, "/", http.StatusSeeOther)
	}
}

// Retrieves data from the backend to display the index page
func index() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)
		action := "index"
		encoder.Encode(action)
		var quotes = []string{}
		decoder.Decode(&quotes)
		tpl.ExecuteTemplate(w, "index.html", quotes)
	}
}

// Detects error on backend through the use of Ping-Ack
func errDetect(backendaddr *net.TCPAddr, errorChan chan bool, endErrorChan chan bool, wg *sync.WaitGroup) {
	wg.Add(1)
	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)
	var ack string
	for {
		select {
		case <-endErrorChan:
			wg.Done()
			return
		default:
			ack = ""
			time.Sleep(1 * time.Second)
			action := "PING"
			encoder.Encode(action)
			decoder.Decode(&ack)
			if ack != "ACK" {
				fmt.Printf("Detected failure on ")
				if backendaddr.IP == nil {
					fmt.Printf("localhost")
				} else {
					fmt.Print(backendaddr.IP)
				}
				fmt.Printf(":%d at ", backendaddr.Port)
				fmt.Println(time.Now())
				errorChan <- true
			}
			time.Sleep(4 * time.Second)
		}
	}
}

// http Handler functions
func httpHandler(port string) {
	http.HandleFunc("/", index())
	http.HandleFunc("/addQuote", addQuote)
	http.HandleFunc("/createQuote", createQuote())
	http.HandleFunc("/deleteQuote", deleteQuote())
	http.HandleFunc("/editQuote", editQuote)
	http.HandleFunc("/updateQuote", updateQuote())
	// connects to provided or default HTTP port
	// Default: localhost:8080
	http.ListenAndServe(port, nil)
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

	p := strconv.Itoa(portnum)
	port := ":" + p

	backendaddr, _ := net.ResolveTCPAddr("tcp", tcpport)

	errorChan := make(chan bool)

	handeled := false

	endErrorChan := make(chan bool)

	var wg sync.WaitGroup

	// connecting to backend with TCP
	for {
		var err error
		conn, err = net.DialTCP("tcp", nil, backendaddr)
		if err == nil {
			if handeled == true {
				endErrorChan <- true
			}
			wg.Wait()
			go errDetect(backendaddr, errorChan, endErrorChan, &wg)

			if handeled == false {
				go httpHandler(port)
				handeled = true
			}

		}
		<-errorChan

	}

}
