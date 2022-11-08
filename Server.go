package main

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/term"
	"gopkg.in/ini.v1"
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
var IDReqChan chan RInfo
var IDResChan chan *sql.Rows
var IDTReqChan chan int
var IDTResChan chan *sql.Rows
var SettingsV Settings
var fd int
var done chan bool
var restart bool

type (
	rType struct {
		Type string `json:"request-type"`
	}
	requestRepair struct {
		FName   string `json:"fname"`
		LName   string `json:"lname"`
		Email   string `json:"email"`
		Phone   string `json:"phone"`
		RType   string `json:"receive-type"`
		DAdress string `json:"delivery-address"`
		Status  string
		Date    string
		PType   string `json:"part-type"`
		Model   string `json:"model"`
		Problem string `json:"repair-description"`
	}
	requestAssembly struct {
		FName       string `json:"fname"`
		LName       string `json:"lname"`
		Email       string `json:"email"`
		Phone       string `json:"phone"`
		RType       string `json:"receive-type"`
		DAdress     string `json:"delivery-address"`
		Status      string
		Date        string
		Case        string `json:"case"`
		Motherboard string `json:"motherboard"`
		CPU         string `json:"cpu"`
		GPU         string `json:"gpu"`
		RAM         string `json:"ram"`
		Storage     string `json:"storage"`
		Notes       string `json:"notes"`
	}
	request interface {
		AddToDB(db *sql.DB)
	}
	BasicInfo struct {
		ID           int
		RType        string
		CreationDate string
		FName        string
		LName        string
	} //TODO: bound id in DB to id in worker UI
	Settings struct {
		Server struct {
			address   string
			dbconnect bool
		}
		DB struct {
			username string
			password string
			protocol string
			address  string
			dbname   string
		}
	}
	RInfo struct {
		id int
		T  string
	}
)

func (r requestRepair) AddToDB(db *sql.DB) {
	db.Exec("INSERT INTO Repairs values(NULL, '" + r.PType + "', '" + r.Model + "', '" + r.Problem + "')")
	db.Exec("INSERT INTO Requests values(NULL, '" + r.FName + "', '" + r.LName + "', '" + r.Email + "', '" + r.Phone + "', '" + r.RType + "', '" + r.DAdress + "', LAST_INSERT_ID(), NULL, 'pending', '', '" + fmt.Sprint(time.Now())[:10] + "')")
}

func (r requestAssembly) AddToDB(db *sql.DB) {
	db.Exec("INSERT INTO Complectations values(NULL, '" + r.Case + "', '" + r.Motherboard + "', '" + r.CPU + "', '" + r.GPU + "', '" + r.RAM + "', '" + r.Storage + "', '" + r.Notes + "')")
	db.Exec("INSERT INTO Requests values(NULL, '" + r.FName + "', '" + r.LName + "', '" + r.Email + "', '" + r.Phone + "', '" + r.RType + "', '" + r.DAdress + "', NULL, LAST_INSERT_ID(), 'pending', '', '" + fmt.Sprint(time.Now())[:10] + "')")
}

func SendToQueue[T request](r T) {
	<-mutex
	queue <- request(r)
	if len(queue) < cap(queue) && len(mutex) == 0 {
		mutex <- true
	}
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
	case "true", "y", "yes", "on":
		SettingsV.Server.dbconnect = true
	default:
		SettingsV.Server.dbconnect = false
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
	k, err := cfg.Section(block).GetKey(key)
	if err != nil || k.String() == "" {
		if !hide {
			fmt.Print(block + "." + key + "=")
			fmt.Scan(dst)
		} else {
			fmt.Print(block + "." + key + "(hidden)=")
			buff, _ := term.ReadPassword(fd)
			*dst = string(buff)
			fmt.Println()
		}
	} else {
		*dst = k.String()
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
		r := fmt.Sprintf("%s:%s@%s(%s)/%s", SettingsV.DB.username, SettingsV.DB.password, SettingsV.DB.protocol, SettingsV.DB.address, SettingsV.DB.dbname)
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
		IDReqChan = make(chan RInfo, 1)
		IDResChan = make(chan *sql.Rows, 1)
		IDTReqChan = make(chan int, 1)
		IDTResChan = make(chan *sql.Rows, 1)
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
	time.Sleep(1 * time.Second)
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
	w.Header().Add("Access-Control-Allow-Origin", "*")

	if r.Method == "GET" {
		switch r.URL.Path {
		case "/":
			io.WriteString(w, mainHTML)
		case "/script.js":
			w.Header().Set("content-type", "application/javascript")
			io.WriteString(w, script)
		case "/favicon.ico":
			v := r.URL.Query()["v"][0]
			switch v {
			case "1":
				http.ServeFile(w, r, "pictures/favicon.ico")
			case "2":
				http.ServeFile(w, r, "pictures/faviconwork.ico")
			default:
				http.ServeFile(w, r, "pictures/favicon.ico")
			}
		case "/worker":
			io.WriteString(w, WUI)
		case "/Worker.js":
			w.Header().Set("content-type", "application/javascript")
			jsoninfo := GetDBInitInfo()
			res := fmt.Sprintf(WUIscript, jsoninfo)
			io.WriteString(w, res)
		case "/GetByIndex":
			id, _ := strconv.Atoi(r.URL.Query()["id"][0])
			res := GetDBInfoByID(id)
			io.WriteString(w, res)
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
		if len(queue) == cap(queue) {
			ok = true
		}
		select {
		case r := <-queue:
			if ok && len(mutex) == 0 {
				mutex <- true
			}
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
			r, err := db.Query("SELECT ID, FName, LName, Date, CASE WHEN RepairID IS NULL THEN 'Complectation' ELSE 'Repair' END AS REQUEST_TYPE FROM requests WHERE status != 'comlete'")
			if err != nil {
				fmt.Println(err)
				InitResChan <- nil
			} else {
				InitResChan <- r
			}
		case id := <-IDTReqChan:
			q := fmt.Sprintf("SELECT CASE WHEN RepairID IS NULL THEN 'Complectation' ELSE 'Repair' END AS REQUEST_TYPE FROM requests WHERE ID=%d", id)
			r, err := db.Query(q)
			if err != nil {
				fmt.Println(err)
				IDTResChan <- nil
			} else {
				IDTResChan <- r
			}
		case inf := <-IDReqChan:
			var q string
			switch inf.T {
			case "Complectation":
				q = fmt.Sprintf("SELECT FName, LName, Email, PhoneNumber, ReceiptType, DeliveryAddress, Status, Date, CaseT, Motherboard, CPU, GPU, RAM, Storage, TermsRequests FROM Complectations_view WHERE ID=%d", inf.id)
			case "Repair":
				q = fmt.Sprintf("SELECT FName, LName, Email, PhoneNumber, ReceiptType, DeliveryAddress, Status, Date, ComponentType, Model, ProblemDescription FROM Repairs_view WHERE ID=%d", inf.id)
			default:
				q = ""
			}
			r, err := db.Query(q)
			if err != nil {
				fmt.Println(err)
				IDResChan <- nil
			} else {
				IDResChan <- r
			}
		case <-done:
		}
	}
	done <- true
}

func GetDBInfoByID(id int) string {
	var res string
	if SettingsV.Server.dbconnect {
		var T string
		IDTReqChan <- id
		resRows := <-IDTResChan
		resRows.Next()
		resRows.Scan(&T)
		IDReqChan <- RInfo{id, T}
		resRows = <-IDResChan
		resRows.Next()
		switch T {
		case "Repair":
			var buff requestRepair
			resRows.Scan(&buff.FName, &buff.LName, &buff.Email, &buff.Phone, &buff.RType, &buff.DAdress, &buff.Status, &buff.Date, &buff.PType, &buff.Model, &buff.Problem)
			var template string = `{"name": "%s %s","email": "%s","reqType": "%s","componentType": "%s","model": "%s","tel": "%s","deliv": "%s","status": "%s","problem": "%s","date": "%s"}`
			var delivery string
			switch buff.RType {
			case "home-delivery":
				delivery = buff.DAdress
			case "parcel-delivery":
				delivery = "Pakomātā (uz " + buff.DAdress + ")"
			case "store-delivery":
				delivery = "Veikalā"
			default:
				delivery = "Veikalā"
			}
			res = fmt.Sprintf(template, buff.FName, buff.LName, buff.Email, T, buff.PType, buff.Model, buff.Phone, delivery, buff.Status, buff.Problem, &buff.Date)
		case "Complectation":
			var buff requestAssembly
			resRows.Scan(&buff.FName, &buff.LName, &buff.Email, &buff.Phone, &buff.RType, &buff.DAdress, &buff.Status, &buff.Date, &buff.Case, &buff.Motherboard, &buff.CPU, &buff.GPU, &buff.RAM, &buff.Storage, &buff.Notes)
			var template string = `{"name": "%s %s","email": "%s","reqType": "%s","case": "%s","motherboard": "%s","cpu": "%s","videocard": "%s","ram": "%s","memory": "%s","tel": "%s","deliv": "%s","status": "%s","notes": "%s","date": "%s"}`
			var delivery string
			switch buff.RType {
			case "home-delivery":
				delivery = buff.DAdress
			case "parcel-delivery":
				delivery = "Pakomātā (uz " + buff.DAdress + ")"
			case "store-delivery":
				delivery = "Veikalā"
			default:
				delivery = "Veikalā"
			}
			res = fmt.Sprintf(template, buff.FName, buff.LName, buff.Email, T, buff.Case, buff.Motherboard, buff.CPU, buff.GPU, buff.RAM, buff.Storage, buff.Phone, delivery, buff.Status, buff.Notes, &buff.Date)
		default:
			fmt.Println("Is this Riekstiņš order?")
			res = `{"name": "Error","email": "Error","reqType": "Error","componentType": "Error","model": "Error","tel": "Error","deliv": "Error","status": "Error","problem": "Error"}`
		}
	} else {
		date := fmt.Sprint(time.Now())
		templateRep := `{"name": "%s %s","email": "%s","reqType": "%s","componentType": "%s","model": "%s","tel": "%s","deliv": "%s","status": "%s","problem": "%s","date": "%s"}`
		templateComp := `{"name": "%s %s","email": "%s","reqType": "%s","case": "%s","motherboard": "%s","cpu": "%s","videocard": "%s","ram": "%s","memory": "%s","tel": "%s","deliv": "%s","status": "%s","notes": "%s","date": "%s"}`
		switch id {
		case 0:
			res = fmt.Sprintf(templateRep, "FName1", "LName1", "Example1@gmail.com", "Repair", "CT1", "Mod1", "+371 00000000", "Number1 Street, Town1", "pending", "Some description 1", date)
		case 1:
			res = fmt.Sprintf(templateRep, "FName2", "LName2", "Example2@gmail.com", "Repair", "CT2", "Mod2", "+371 11111111", "Pakomātā (uz Number2 Street, Town2)", "processing", "Some description 2", date)
		case 3:
			res = fmt.Sprintf(templateRep, "FName4", "LName4", "Example4@gmail.com", "Repair", "CT3", "Mod3", "+371 22222222", "Veikalā", "canceled", "Some description 3", date)
		case 2:
			res = fmt.Sprintf(templateComp, "FName3", "LName3", "Example3@gmail.com", "Complectation", "case1", "mother1", "cpu1", "gpu1", "ram1", "mem1", "+371 33333333", "Veikalā", "pending", "Some description 1", date)
		case 4:
			res = fmt.Sprintf(templateComp, "FName5", "LName5", "Example5@gmail.com", "Complectation", "case2", "mother2", "cpu2", "gpu2", "ram2", "mem2", "+371 44444444", "Number5 Street, Town9", "delivering", "Some description 2", date)
		case 5:
			res = fmt.Sprintf(templateComp, "FName6", "LName6", "Example6@gmail.com", "Complectation", "case3", "mother3", "cpu3", "gpu3", "ram3", "mem3", "+371 55555555", "Veikalā", "waiting", "Some description 3", date)
		case 6:
			res = fmt.Sprintf(templateRep, "Aigars", "Riekstiņš", "Devil@gmail.com", "Repair", "compType", "Model", "+666666", "Class No9", "pending", "", "2020-09-01")
		case 7:
			res = fmt.Sprintf(templateComp, "Aigars", "Riekstiņš", "Devil@gmail.com", "Complectation", "CaseName", "MoboName", "cpuName", "gpuName", "ramName", "memoryName", "+666666", "Veikalā", "pending", "", "2020-09-01")
		default:
			res = fmt.Sprintf(templateRep, "Error", "Error", "Error", "Error", "Error", "Error", "Error", "Error", "Error", "Error", "Error")
		}
	}
	return res
}

func GetDBInitInfo() string {
	var resjson string
	var template string = `{'Piepr':'%s: %s, %s %s', 'Darb1':'Skatīt detaļas','Darb2':'Atsūtīt vēstuli','Darb3':'Samainīt statusu','Id':'%d'},`
	var ress []BasicInfo
	if SettingsV.Server.dbconnect {
		InitReqChan <- true
		resRows := <-InitResChan
		for resRows.Next() {
			var res BasicInfo
			resRows.Scan(&res.ID, &res.FName, &res.LName, &res.CreationDate, &res.RType)
			ress = append(ress, res)
		}
	} else {
		ress = append(ress, BasicInfo{0, "Repair", fmt.Sprint(time.Now())[:10], "FName1", "LName1"})
		ress = append(ress, BasicInfo{1, "Repair", fmt.Sprint(time.Now())[:10], "FName2", "LName2"})
		ress = append(ress, BasicInfo{2, "Complectation", fmt.Sprint(time.Now())[:10], "FName3", "LName3"})
		ress = append(ress, BasicInfo{3, "Repair", fmt.Sprint(time.Now())[:10], "FName4", "LName4"})
		ress = append(ress, BasicInfo{4, "Complectation", fmt.Sprint(time.Now())[:10], "FName5", "LName5"})
		ress = append(ress, BasicInfo{5, "Complectation", fmt.Sprint(time.Now())[:10], "FName6", "LName6"})
		ress = append(ress, BasicInfo{6, "Repair", "2020-09-01", "Aigars", "Riekstiņš"})
		ress = append(ress, BasicInfo{7, "Complectation", "2020-09-01", "Aigars", "Riekstiņš"})
	}
	for _, src := range ress {
		var tp string
		switch src.RType {
		case "Repair":
			tp = "Detaļas Remonts"
		case "Complectation":
			tp = "Datora Komplektēšana"
		default:
			tp = "This is totaly Riekstiņš order!"
		}
		resjson += fmt.Sprintf(template, tp, src.CreationDate, src.FName, src.LName, src.ID)
	}
	resjson = resjson[:len(resjson)-1]
	return resjson
}
