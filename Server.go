package main

import (
	"net/http"
	"log"
	"io"
	_ "embed"
	"database/sql"
	"encoding/json"
	"time"
	"fmt"
)

//go:embed index.html
var mainHTML string

var isRunning bool//TODO: make contollable by admin
var queue chan request
var mutex chan bool

type (
	rType struct {
		Type string `json:"request-type"`
	}
	requestRepair struct {
		FName string `json:"fname"`
		LName string `json:"lname"`
		Email string `json:"email"`
		Phone string `json:"phone"`
		RType string `json:"receive-type"`
		DAdress string `json:"delivery-address"`
		//TODO: add repair specific fields
	}
	requestAssembly struct {
		FName string `json:"fname"`
		LName string `json:"lname"`
		Email string `json:"email"`
		Phone string `json:"phone"`
		RType string `json:"receive-type"`
		DAdress string `json:"delivery-address"`
		//TODO: add assembly specific fields
	}
	request interface {
		AddToDB(db *sql.DB)
	}
)

func (r requestRepair) AddToDB(db *sql.DB) {
	db.Exec("INSERT INTO Requests values(0, '"+r.FName+"', '"+r.LName+"', '"+r.Email+"', '"+r.Phone+"', '"+r.RType+"', '"+r.DAdress+"', 0, 0, 'statuss', '', '"+fmt.Sprint(time.Now())[:10]+"')")//placeholder
	//ReceiptType:byte/string???
	//TODO: check if adding time works
	//TODO: make actual sql request
}

func (r requestAssembly) AddToDB(db *sql.DB) {
	db.Exec("INSERT INTO Requests values(0, '"+r.FName+"', '"+r.LName+"', '"+r.Email+"', '"+r.Phone+"', '"+r.RType+"', '"+r.DAdress+"', 0, 0, 'statuss', '', '"+fmt.Sprint(time.Now())[:10]+"')")//placeholder
	//ReceiptType:byte/string???
	//TODO: check if adding time works
	//TODO: make actual sql request
}

func SendToQueue[T request](r T) {
	<-mutex
	queue<-request(r)
	if len(queue) < cap(queue) && len(mutex) == 0 {mutex<-true}
}

func main() {
	db, err := sql.Open("driver-name", "database=test1")//TODO: choose MySQLDriver; https://github.com/golang/go/wiki/SQLDrivers
	if err != nil {
		log.Fatal(err)
	}
	mutex = make(chan bool, 1)
	mutex <- true
	queue = make(chan request, 100)
	isRunning = true
	go RequestInserter(db)
	
	http.HandleFunc("/", MainHandler)
	
	log.Fatal(http.ListenAndServe(":8080", nil))//TODO: change host
}

func MainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Access-Control-Allow-Methods", "GET, POST")
	w.Header().Add("Access-Control-Allow-Headers", "content-type")
	w.Header().Add("Access-Control-Allow-Origin","*")
	/*
	 * 	method: 'POST', // *GET, POST, PUT, DELETE, etc.
		mode: 'cors', // no-cors, *cors, same-origin
		cache: 'no-cache', // *default, no-cache, reload, force-cache, only-if-cached
		credentials: 'same-origin', // include, *same-origin, omit
		headers: {
		  'Content-Type': 'application/json'
		},
		redirect: 'follow', // manual, *follow, error
		referrerPolicy: 'no-referrer', // no-referrer, *client
		body: ...
	 *///TODO: add some of those into js request (mode is mandatory)
	
	if r.Method == "GET" {
		io.WriteString(w, mainHTML)
	} else if r.Method == "POST" {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(405)
			log.Println(err)
		}
		r.Body.Close()
		var t rType
		json.Unmarshal(body, &t)
		if t.Type == "repair" {
			var repReq requestRepair
			json.Unmarshal(body, &repReq)
			SendToQueue(repReq)
		} else if t.Type == "assembly" {
			var assReq requestAssembly
			json.Unmarshal(body, &assReq)
			SendToQueue(assReq)
		}
	} else {
		w.WriteHeader(405)
	}
}

func RequestInserter(db *sql.DB) {
	for isRunning {
		var ok bool
		if len(queue) == cap(queue) {ok = true}
		r := <-queue
		if ok && len(mutex) == 0 {mutex<-true}
		r.AddToDB(db)
	}
}
