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
	"gopkg.in/ini.v1"
	"golang.org/x/term"
	"os"
	"strings"
	"context"
	//"os/exec"
)

//go:embed index.html
var mainHTML string

//go:embed script.js
var script string

//go:embed WorkerUI.html
var WUI string

//go:embed Worker.js
var WUIscript string

var isRunning bool
var queue chan request
var mutex chan bool
var InitReqChan chan bool
var InitResChan chan *sql.Rows
var IDReqChan chan int
var IDResChan chan *sql.Rows
var SettingsV Settings
var fd int
var done chan bool
var restart bool

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
		Status string
		Date string
		PType string `json:"part-type"`
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
		Status string
		Date string
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
	BasicInfo struct {
		ID int
		RType string
		CreationDate string
		FName string
		LName string
	}//TODO: bound id in DB to id in worker UI
	Settings struct {
		Server struct {
			address string
			dbconnect bool
		}
		DB struct {
			username string
			password string
			protocol string
			address string
			dbname string
		}
	}
)

func (r requestRepair) AddToDB(db *sql.DB) {
	db.Exec("INSERT INTO Repairs values(NULL, '"+r.PType+"', '"+r.Model+"', '"+r.Problem+"')")
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

func init() {
	fd = int(os.Stdin.Fd())
	cfg, err := ini.Load("settings.ini")
    if err != nil {
        log.Fatal(err)
    }
	GetVal(cfg, "Server", "address", &(SettingsV.Server.address), false)
	var dbcRes string
	GetVal(cfg, "Server", "dbconnect", &dbcRes, false)
	
	switch strings.ToLower(dbcRes) {
		case "true", "y", "yes", "on":SettingsV.Server.dbconnect=true
		default:SettingsV.Server.dbconnect=false
	}
	if SettingsV.Server.dbconnect {
		GetVal(cfg, "DB", "username", &(SettingsV.DB.username), false)
		GetVal(cfg, "DB", "password", &(SettingsV.DB.password), true)
		GetVal(cfg, "DB", "protocol", &(SettingsV.DB.protocol), false)
		GetVal(cfg, "DB", "address", &(SettingsV.DB.address), false)
		GetVal(cfg, "DB", "dbname", &(SettingsV.DB.dbname), false)
	}
}

func GetVal(cfg *ini.File, block, key string, dst *string, hide bool) {
	k, err:=cfg.Section(block).GetKey(key)
    if err != nil || k.String()==""{
		if !hide {
			fmt.Print(block+"."+key+"=")
			fmt.Scan(dst)
		} else {
			fmt.Print(block+"."+key+"(hidden)=")
			buff, _ := term.ReadPassword(fd)
			*dst = string(buff)
			fmt.Println()
		}
	} else {
		*dst=k.String()
	}
}

func main() {
	/*defer func() {
		if restart {
			exec.Command("start cmd.exe @cmd /k \"./Server.exe\"").Run()
		}
	}()*/
	isRunning = true
	if SettingsV.Server.dbconnect {
		r:=fmt.Sprintf("%s:%s@%s(%s)/%s",SettingsV.DB.username,SettingsV.DB.password,SettingsV.DB.protocol,SettingsV.DB.address,SettingsV.DB.dbname)
		db, err := sql.Open("mysql", r)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()
		mutex = make(chan bool, 1)
		mutex <- true
		queue = make(chan request, 100)
		InitReqChan = make(chan bool, 1)
		InitResChan = make(chan *sql.Rows, 1)
		IDReqChan = make(chan int, 1)
		IDResChan = make(chan *sql.Rows, 1)
		done = make(chan bool, 2)
		go RequestInserter(db)
		go RequestGetter(db)
	}
	
	http.HandleFunc("/", MainHandler)
	
	server := http.Server{Addr: SettingsV.Server.address}
	defer server.Close()
	go func() {
		server.ListenAndServe()
		fmt.Println("Server is stopped.")
	}()
	fmt.Println("Server is running.")
	time.Sleep(1*time.Second)
	for isRunning {
		var cmd string
		fmt.Print("Server:\\>")
		fmt.Scan(&cmd)
		
		switch strings.ToLower(cmd) {
			case "stop":
				fmt.Println("Server shutdown.")
				server.Shutdown(context.Background())
				isRunning = false
				restart = false
			case "restart":
				fmt.Println("Server shutdown.")
				server.Shutdown(context.Background())
				isRunning = false
				restart = true
			default:
				fmt.Println("Unknown command.")
		}
	}
	if SettingsV.Server.dbconnect {
		done <- true
		<-done
		done <- true
		<-done
	}
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
				w.Header().Set("content-type", "application/javascript")
				io.WriteString(w, script)
			case "/favicon.ico":
				v:=r.URL.Query()["v"][0]
				switch v {
					case "1":http.ServeFile(w, r, "pictures/favicon.ico")
					case "2":http.ServeFile(w, r, "pictures/faviconwork.ico")
					default:http.ServeFile(w, r, "pictures/favicon.ico")
				}
			case "/worker":
				io.WriteString(w, WUI)
			case "/Worker.js":
				w.Header().Set("content-type", "application/javascript")
				jsoninfo := GetDBInitInfo()
				res := fmt.Sprintf(WUIscript, jsoninfo)
				io.WriteString(w, res)
			case "/GetByIndex":
				var id int//TODO:read
				res := GetDBInfoByID(id)
				io.WriteString(w, res)//TODO?:change
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
			if SettingsV.Server.dbconnect {
				SendToQueue(repReq)
			} else {
				fmt.Println("Server has received data: ", repReq)
				fmt.Print("Server:\\>")
			}
		} else if t.Type == "assembly" {
			var assReq requestAssembly
			json.Unmarshal(body, &assReq)
			if SettingsV.Server.dbconnect {
				SendToQueue(assReq)
			} else {
				fmt.Println("Server has received data: ", assReq)
				fmt.Print("Server:\\>")
			}
		}
	} else {
		w.WriteHeader(405)
	}
}

func RequestInserter(db *sql.DB) {
	for isRunning {
		var ok bool
		if len(queue) == cap(queue) {ok = true}
		select {
			case r := <-queue:
				if ok && len(mutex) == 0 {mutex<-true}
				r.AddToDB(db)
			case <-done:
		}
	}
	done <- true
}

func RequestGetter(db *sql.DB) {
	for isRunning {
		select {
			case <-InitReqChan:
				r, err := db.Query("SELECT FName, LName, Date, CASE WHEN RepairID IS NULL THEN 'Complectation' ELSE 'Repair' END AS REQUEST_TYPE FROM requests WHERE status != 'comlete'")
				if err != nil {
					fmt.Println(err)
				} else {
					InitResChan <- r
				}
			case id:=<-IDReqChan:
				r, err := db.Query("SELECT CASE WHEN RepairID IS NULL THEN 'Complectation' ELSE 'Repair' END AS REQUEST_TYPE FROM requests WHERE ID=?; SELECT * FROM Repairs_view WHERE ID=?; SELECT * FROM Complectations_view WHERE ID=?", id, id, id)
				if err != nil {
					fmt.Println(err)
				} else {
					IDResChan <- r
				}
			case <-done:
		}
	}
	done <- true
}

func GetDBInfoByID(id int) string {
	IDReqChan <- id
	resRows := <- IDResChan
	resRows.Next()
	var typ string
	resRows.Scan(&typ)
	var res string
	switch typ {
		case "Repair":
			var buff requestRepair
			resRows.NextResultSet()
			resRows.Next()
			resRows.Scan(&buff.FName,&buff.LName,&buff.Email,&buff.Phone,&buff.RType,&buff.DAdress,&buff.Status,&buff.Date,&buff.PType,&buff.Model,&buff.Problem)
			fmt.Println(buff)
			res=""//TODO:add
		case "Complectation":
			var buff requestAssembly
			resRows.NextResultSet()
			resRows.NextResultSet()
			resRows.Next()
			resRows.Scan(&buff.FName,&buff.LName,&buff.Email,&buff.Phone,&buff.RType,&buff.DAdress,&buff.Status,&buff.Date, &buff.Case, &buff.Motherboard, &buff.CPU, &buff.GPU, &buff.RAM, &buff.Storage, &buff.Notes)
			fmt.Println(buff)
			res=""//TODO:add
		default :
			fmt.Println("Is this Riekstiņš order?")
			res=""//TODO:add all fields with err msg
	}
	return res
}

func GetDBInitInfo() string {
	var resjson string
	var template string = `{'Piepr':'%s: %s, %s %s', 'Darb1':'Skatīt detaļas','Darb2':'Atsūtīt vēstuli','Darb3':'Amainīt statusu'},`
	var ress []BasicInfo
	if SettingsV.Server.dbconnect {
		InitReqChan <- true
		resRows := <- InitResChan
		for resRows.Next() {
			var res BasicInfo
			resRows.Scan(&res.FName,&res.LName,&res.CreationDate,&res.RType)
			ress = append(ress, res)
		}
	} else {
		ress = append(ress, BasicInfo{0, "Repair", fmt.Sprint(time.Now())[:10], "FName1", "LName1"})
		ress = append(ress, BasicInfo{0, "Repair", fmt.Sprint(time.Now())[:10], "FName2", "LName2"})
		ress = append(ress, BasicInfo{0, "Complectation", fmt.Sprint(time.Now())[:10], "FName3", "LName3"})
		ress = append(ress, BasicInfo{0, "Repair", fmt.Sprint(time.Now())[:10], "FName4", "LName4"})
		ress = append(ress, BasicInfo{0, "Complectation", fmt.Sprint(time.Now())[:10], "FName5", "LName5"})
		ress = append(ress, BasicInfo{0, "Complectation", fmt.Sprint(time.Now())[:10], "FName6", "LName6"})
	}
	for _, src := range ress {
		var tp string
		switch src.RType {
			case "Repair":tp="Detaļas Remonts"
			case "Complectation":tp="Datora Komplektēšana"
			default:tp="This is totaly Riekstiņš order!"
		}
		resjson+=fmt.Sprintf(template, tp, src.CreationDate, src.FName, src.LName)
	}
	resjson = resjson[:len(resjson)-1]
	return resjson
}
