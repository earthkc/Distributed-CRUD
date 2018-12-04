// BACK END SERVER FOR
// Quotes, The Best Quotes

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"strconv"
	"sync"
)

// Global Slice that Holds the strings of quotes
var quotes = []string{}

// Read/Write Mutex
var mutex sync.RWMutex

// Adds a new user inputted quote into the global slice
func createQuote(a string) {
	mutex.Lock()
	quotes = append(quotes, a)
	mutex.Unlock()
}

// Deletes the selected quote
// *** Could be better implemented if quotes were stored with an identifier. Problems
// may arise if two quotes are exactly the same ***
func deleteQuote(id string) {
	index := 0
	mutex.Lock()
	for i, v := range quotes {
		if v == id {
			index = i
			break
		}
	}
	quotes = quotes[:index+copy(quotes[index:], quotes[index+1:])]
	mutex.Unlock()
}

// Edits/updates a quote with changes the user inputs
func updateQuote(id string, newQuote string) {
	index := 0
	mutex.Lock()
	for i, v := range quotes {
		if v == id {
			index = i
			break
		}
	}
	quotes[index] = newQuote
	mutex.Unlock()
}

// Handles each TCP connection
func connHandler(conn net.Conn) {
	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	// Reads actions from the front end
	for {
		var action string
		decoder.Decode(&action)
		switch action {
		case "PING":
			encoder.Encode("ACK")
		case "index":
			mutex.RLock()
			encoder.Encode(quotes)
			mutex.RUnlock()
		case "create":
			var newQuote string
			decoder.Decode(&newQuote)
			createQuote(newQuote)
		case "delete":
			var id string
			decoder.Decode(&id)
			deleteQuote(id)
		case "update":
			var id string
			var newQuote string
			decoder.Decode(&id)
			decoder.Decode(&newQuote)
			updateQuote(id, newQuote)
		}
	}
}

func raftConsensus() {
	type State int
	//var quorum int
	const (
		Follower  State = 0
		Candidate State = 1
		Leader    State = 2
	)
	//currState := Follower

}

func main() {

	// Preliminary hardcoded data
	quotes = append(quotes, "If a book about failures doesn’t sell, is it a success? - Jerry Seinfeld")
	quotes = append(quotes, "I like long walks, especially when they are taken by people who annoy me. - Fred Allen")
	quotes = append(quotes, "The most powerful force in the universe is compound interest. - Albert Einstein")
	quotes = append(quotes, "I love mankind. It's people I can't stand. - Charles M. Schulz")
	quotes = append(quotes, "Laziness is nothing more than the habit of resting before you get tired. - Jules Renard")
	quotes = append(quotes, "Go to Heaven for the climate, Hell for the company. - Mark Twain")
	quotes = append(quotes, "I can resist everything except temptation. - Oscar Wilde")
	quotes = append(quotes, "A committee is a group that keeps minutes and loses hours. - Milton Berle")

	// front end listen flag
	var portnum int
	flag.IntVar(&portnum, "listen", 8090, "front end listening port")
	flag.Parse()
	p := strconv.Itoa(portnum)
	port := ":" + p

	// Listens on this port
	ln, err := net.Listen("tcp", port)
	if err != nil {
		fmt.Println("Error listening on TCP port")
	}

	for {
		// Establishes a TCP connection to the frontend
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Error accepting connection")
		}
		fmt.Println("Connection accepted.")
		go connHandler(conn)
	}
	//defer conn.Close()
}
