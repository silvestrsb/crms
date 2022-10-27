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
	_ "github.com/go-sql-driver/mysql"
)

//go:embed index.html
var mainHTML string

//go:embed script.js
var script string

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
		PTypre string `json:"part-type"`
		Model string `json:"model"`
		Problem string `json:"repair-description"`
	}
	requestAssembly struct {
		FName string `json:"fname"`
		LName string `json:"lname"`
		Email string `json:"email"`
		Phone string `json:"phone"`
		RType string `json:"receive-type"`
		DAdress string `json:"delivery-address"`
		Case string `json:"case"`
		Motherboard string `json:"motherboard"`
		CPU string `json:"cpu"`
		GPU string `json:"gpu"`
		RAM string `json:"ram"`
		Storage string `json:"storage"`
		Notes string `json:"notes"`
	}
	request interface {
		AddToDB(db *sql.DB)
	}
)

func (r requestRepair) AddToDB(db *sql.DB) {
	db.Exec("INSERT INTO Repairs values(NULL, '"+r.PTypre+"', '"+r.Model+"', '"+r.Problem+"')")
	db.Exec("INSERT INTO Requests values(NULL, '"+r.FName+"', '"+r.LName+"', '"+r.Email+"', '"+r.Phone+"', '"+r.RType+"', '"+r.DAdress+"', LAST_INSERT_ID(), NULL, 'pending', '', '"+fmt.Sprint(time.Now())[:10]+"')")
}

func (r requestAssembly) AddToDB(db *sql.DB) {
	db.Exec("INSERT INTO Complectations values(NULL, '"+r.Case+"', '"+r.Motherboard+"', '"+r.CPU+"', '"+r.GPU+"', '"+r.RAM+"', '"+r.Storage+"', '"+r.Notes+"')")
	db.Exec("INSERT INTO Requests values(NULL, '"+r.FName+"', '"+r.LName+"', '"+r.Email+"', '"+r.Phone+"', '"+r.RType+"', '"+r.DAdress+"', NULL, LAST_INSERT_ID(), 'pending', '', '"+fmt.Sprint(time.Now())[:10]+"')")
}

func SendToQueue[T request](r T) {
	<-mutex
	queue<-request(r)
	if len(queue) < cap(queue) && len(mutex) == 0 {mutex<-true}
}

func main() {
	db, err := sql.Open("mysql", "root:Just_password@tcp(localhost:3306)/server")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	mutex = make(chan bool, 1)
	mutex <- true
	queue = make(chan request, 100)
	isRunning = true
	go RequestInserter(db)
	
	http.HandleFunc("/", MainHandler)
	
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func MainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Access-Control-Allow-Methods", "GET, POST")
	w.Header().Add("Access-Control-Allow-Headers", "content-type")
	w.Header().Add("Access-Control-Allow-Origin","*")
	
	if r.Method == "GET" {
		switch r.URL.Path {
			case "/":
				io.WriteString(w, mainHTML)
			case "/script.js":
				io.WriteString(w, script)
			case "/favicon.ico":
				http.ServeFile(w, r, "pictures/favicon.ico")
			default:
				w.WriteHeader(405)
		}
		
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
