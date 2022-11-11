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
	"strconv"
	"os/exec"
	"bufio"
	"runtime"
	"net/smtp"
)

//go:embed index.html
var mainHTML string

//go:embed script.js
var script string

//go:embed WorkerUI.html
var WUI string

//go:embed Worker.js
var WUIscript string

//go:embed login.html
var loginhtml string

//go:embed login.js
var loginjs string

//go:embed admin.html
var admhtml string

//go:embed admin.js
var admjs string

var isRunning bool
var queue chan request
var mutex chan bool
var InitReqChan chan bool
var InitResChan chan *sql.Rows
var IDReqChan chan RInfo
var IDResChan chan *sql.Rows
var IDTReqChan chan int
var IDTResChan chan *sql.Rows
var StatReqChan chan StatusChange
var StatResChan chan string
var UserReqChan chan UserData
var UserResChan chan *sql.Rows
var EmailReqChan chan int
var EmailResChan chan *sql.Rows
var QReqChan chan string
var QResChan chan RowErr
var SettingsV Settings
var fd int
var done chan bool = make(chan bool, 4)
var restart bool
var sep string
var auth smtp.Auth
var UserDB []UserData

var SessionMutex chan bool = make(chan bool, 1)
var SessionDB []Session
var SessID = 1

var consoleIn chan string = make(chan string, 1)
var consoleOut chan string = make(chan string, 1)
var adminFormIn chan string = make(chan string, 1)
var adminFormOut chan string = make(chan string, 1)
var AdmReqChan chan string = make(chan string, 1)
var AdmResChan chan string = make(chan string, 1)

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
	}
	Settings struct {
		Server struct {
			address string
			dbconnect bool
			mailconnect bool
		}
		DB struct {
			username string
			password string
			protocol string
			address string
			dbname string
		}
		Mail struct {
			email string
			password string
		}
	}
	RInfo struct {
		id int
		T string
	}
	StatusChange struct {
		ID int `json:"id"`
		ByWhom int
		NewStatus string `json:"status"`
		Comment string `json:"comment"`
	}
	MsgInfo struct {
		To string `json:"email"`
		Body string `json:"msg"`
	}
	
	Session struct {
		ID int
		userID int
		isAdmin bool
		openedDate time.Time
		expirationDate time.Time
	}
	UserData struct {
		Login string `json:"login"`
		Password string `json:"password"`
	}
	RowErr struct {
		r *sql.Rows
		err error
	}
)

func createSession(userID int, isAdm bool) int {
	now := time.Now()
	sess := Session{SessID, userID, isAdm, now, now.Add(24*time.Hour)}
	SessID++
	<-SessionMutex
	SessionDB = append(SessionDB,sess)
	SessionMutex<-true
	return sess.ID
}

func DeleteSession(sessID int) {
	<-SessionMutex
	i := 0
	var s Session
	for i, s = range SessionDB {
		if s.ID == sessID {
			break
		}
	}
	
	SessionDB = append(SessionDB[:i], SessionDB[i+1:]...)
	SessionMutex <- true
}

func checkSession(req *http.Request) (bool, int) {
	SID, err := strconv.Atoi(req.Header.Get("Session-ID"))
	if err != nil {
		return false, SID
	}
	for _, s := range SessionDB {
		if s.ID == SID {
			return true, SID
		}
	}
	return false, SID
}

func checkSession2(req *http.Request) (bool, int) {
	SID, err := strconv.Atoi(req.URL.Query()["sid"][0])
	if err != nil {
		return false, SID
	}
	for _, s := range SessionDB {
		if s.ID == SID {
			return true, SID
		}
	}
	return false, SID
}

func checkUser(u UserData) (bool, int, bool) {
	if SettingsV.Server.dbconnect {
		UserReqChan <- u
		rows:=<-UserResChan
		if rows.Next() {
			var uid int
			var pass string
			rows.Scan(&uid, &pass)
			return pass==u.Password, uid, strings.HasPrefix(u.Login, "[ADM]")
		} else {
			return false, 0, false
		}
	} else {
		for i, us := range UserDB {
			if us.Login == u.Login && us.Password == u.Password {
				return true, i, strings.HasPrefix(u.Login, "[ADM]")
			}
		}
		return false, 0, false
	}
}

func SIDIsAdmin(id int) bool {
	for _, s := range SessionDB {
		if s.ID == id {
			return s.isAdmin
		}
	}
	return false
}

func (r requestRepair) AddToDB(db *sql.DB) {
	db.Exec("INSERT INTO Repairs VALUES (NULL, '"+r.PType+"', '"+r.Model+"', '"+r.Problem+"')")
	date:=fmt.Sprint(time.Now())[:10]
	db.Exec("INSERT INTO Requests VALUES (NULL, '"+r.FName+"', '"+r.LName+"', '"+r.Email+"', '"+r.Phone+"', '"+r.RType+"', '"+r.DAdress+"', LAST_INSERT_ID(), -1, 'pending', '', '"+date+"')")
	templ:=`name: %s %s, email: %s, phoneNum: %s, deliveryType: %s, adress: %s, date: %s, componentType: %s, model: %s, problemDescription: %s`
	q:=fmt.Sprintf("INSERT INTO Audit VALUES (NULL,0,'Client make new repair request with data: "+templ+"')",r.FName,r.LName,r.Email,r.Phone,r.RType,r.DAdress,date,r.PType,r.Model,r.Problem)
	db.Exec(q)
}

func (r requestAssembly) AddToDB(db *sql.DB) {
	db.Exec("INSERT INTO Complectations VALUES (NULL, '"+r.Case+"', '"+r.Motherboard+"', '"+r.CPU+"', '"+r.GPU+"', '"+r.RAM+"', '"+r.Storage+"', '"+r.Notes+"')")
	date:=fmt.Sprint(time.Now())[:10]
	db.Exec("INSERT INTO Requests VALUES (NULL, '"+r.FName+"', '"+r.LName+"', '"+r.Email+"', '"+r.Phone+"', '"+r.RType+"', '"+r.DAdress+"', -1, LAST_INSERT_ID(), 'pending', '', '"+date+"')")
	templ:=`name: %s %s, email: %s, phoneNum: %s, deliveryType: %s, adress: %s, date: %s, case: %s, motherboard: %s, cpu: %s, videocard: %s, ram: %s, memory: %s, notes: %s`
	q:=fmt.Sprintf("INSERT INTO Audit VALUES (NULL,0,'Client make new complectation request with data: "+templ+"')",r.FName,r.LName,r.Email,r.Phone,r.RType,r.DAdress,date,r.Case,r.Motherboard,r.CPU,r.GPU,r.RAM,r.Storage,r.Notes)
	db.Exec(q)
}

func SendToQueue[T request](r T) {
	<-mutex
	queue<-request(r)
	if len(queue) < cap(queue) && len(mutex) == 0 {mutex<-true}
}

func init() {
	if strings.Contains(runtime.GOOS,"windows") {
		sep = "\\"
	} else {
		sep = "/"
	}
	UserDB=append(UserDB, UserData{"[ADM]admin", "somepass"})
	UserDB=append(UserDB, UserData{"worker", "123456"})
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
	
	var mcRes string
	GetVal(cfg, "Server", "mailconnect", &mcRes, false)
	switch strings.ToLower(mcRes) {
		case "true", "y", "yes", "on":SettingsV.Server.mailconnect=true
		default:SettingsV.Server.mailconnect=false
	}
	if SettingsV.Server.dbconnect {
		GetVal(cfg, "DB", "username", &(SettingsV.DB.username), false)
		GetVal(cfg, "DB", "password", &(SettingsV.DB.password), true)
		GetVal(cfg, "DB", "protocol", &(SettingsV.DB.protocol), false)
		GetVal(cfg, "DB", "address", &(SettingsV.DB.address), false)
		GetVal(cfg, "DB", "dbname", &(SettingsV.DB.dbname), false)
		
	}
	if SettingsV.Server.mailconnect {
		GetVal(cfg, "Mail", "email", &(SettingsV.Mail.email), true)
		GetVal(cfg, "Mail", "password", &(SettingsV.Mail.password), true)
		auth = smtp.PlainAuth("", SettingsV.Mail.email, SettingsV.Mail.password, "smtp.gmail.com")
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
	root, _ := os.Getwd()
	SessionMutex <- true
	defer func() {
		if restart {
			if strings.Contains(runtime.GOOS,"windows") {
				exec.Command("cmd.exe", "/c", "start", root+"\\os\\restart.bat", root).Run()
			}
		} 
	}()
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
		IDReqChan = make(chan RInfo, 1)
		IDResChan = make(chan *sql.Rows, 1)
		IDTReqChan = make(chan int, 1)
		IDTResChan = make(chan *sql.Rows, 1)
		StatReqChan = make(chan StatusChange, 1)
		StatResChan = make(chan string, 1)
		UserReqChan = make(chan UserData, 1)
		UserResChan = make(chan *sql.Rows, 1)
		EmailReqChan = make(chan int, 1)
		EmailResChan = make(chan *sql.Rows, 1)
		QReqChan = make(chan string, 1)
		QResChan = make(chan RowErr, 1)
		
		go RequestProcesser(db)
		go RequestGetter(db)
	}
	go SessionDBWorker()
	http.HandleFunc("/", MainHandler)
	
	server := http.Server{Addr: SettingsV.Server.address}
	defer server.Close()
	go func() {
		server.ListenAndServe()
	}()
	
	go consoleScanner()
	go ADMScanner()
	fmt.Println("Server is running.")
	time.Sleep(1*time.Second)
	consoleOut<-""
	for isRunning {
		var fullcmd string
		var out chan string
		select {
			case fullcmd=<-consoleIn:
				out = consoleOut
			case fullcmd=<-adminFormIn:
				out = adminFormOut
		}
		
		cmdarr:=strings.Split(fullcmd, " ")
		if len(cmdarr)<=0 {
			out<-"Empty command!"
			continue
		}
		cmd:=cmdarr[0]
		
		switch strings.ToLower(cmd) {
			case "stop":
				isRunning = false
				restart = false
				out<-"Server shutdown."
				time.Sleep(1*time.Second)
				server.Shutdown(context.Background())
			case "restart":
				if strings.Contains(runtime.GOOS,"windows"){
					out<-"Server shutdown."
					time.Sleep(1*time.Second)
					server.Shutdown(context.Background())
					isRunning = false
					restart = true
				} else {
					out<-"This command is not supported by Your os."
				}
				
			case "upload":
				if len(cmdarr)<=2 {
					out<-"Not enough arguments."
					continue
				}
				var location string
				var path string
				var i int = 1
				if strings.HasPrefix(cmdarr[i], `"`) {
					for {
						if i >= len(cmdarr) {
							break
						}
						if strings.HasSuffix(cmdarr[i], `"`) {
							break
						}
						
						i++
					}
					if i >= len(cmdarr) {
						out<-"Error in arguments."
						continue
					}
					location = strings.Join(cmdarr[1:i+1], " ")
					location = location[1:len(location)-1]
					i++
				} else {
					location = cmdarr[i]
					i++
				}
				if i >= len(cmdarr) {
					out<-"Not enough arguments."
					continue
				}
				oldi:=i
				if strings.HasPrefix(cmdarr[i], `"`) {
					for {
						if i >= len(cmdarr) {
							break
						}
						if strings.HasSuffix(cmdarr[i], `"`) {
							break
						}
						i++
					}
					if i >= len(cmdarr) {
						out<-"Error in arguments."
						continue
					}
					path = strings.Join(cmdarr[oldi:i+1], " ")
					path = path[1:len(path)-1]
				} else {
					path = cmdarr[i]
				}
				if strings.ToLower(location) == "root" {
					location = ""
				}
				src, err := os.Open(path)
				if err != nil {
					out<-fmt.Sprint(err)
					continue
				}
				
				if len(location) != 0 {
					err = os.MkdirAll(location, 0750)
					if err != nil {
						out<-fmt.Sprint(err)
						src.Close()
						continue
					}
				}
				
				stat, _ := src.Stat()
				dst, err := os.Create(root+sep+location+sep+stat.Name())
				if err != nil {
					out<-fmt.Sprint(err)
					src.Close()
					continue
				}
				var buff []byte
				
				buff = make([]byte, stat.Size())
				src.Read(buff)
				dstW:=bufio.NewWriter(dst)
				dstW.Write(buff)
				dstW.Flush()
				dst.Close()
				src.Close()
				out <- "Command is complete!"
			default:
				out<-"Unknown command: " + cmd
		}
	}
	time.Sleep(1*time.Second)
	fmt.Println("Server will stop soon.")
	if SettingsV.Server.dbconnect {
		<-done
		<-done
	}
	<-done
	<-done
	fmt.Println("Server is stopped.")
}

func MainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Access-Control-Allow-Methods", "GET, POST")
	w.Header().Add("Access-Control-Allow-Headers", "content-type, session-id")
	w.Header().Add("Access-Control-Allow-Origin","*")
	
	if r.Method == "GET" {
		switch r.URL.Path {
			case "/":
				io.WriteString(w, mainHTML)
			case "/script.js":
				w.Header().Set("content-type", "application/javascript")
				io.WriteString(w, script)
			case "/favicon.ico":
				varr:=r.URL.Query()["v"]
				var v string
				if len(varr) <= 0 {
					var toScan struct {
						V int `json:"v"`
					}
					body, err := io.ReadAll(r.Body)
					if err != nil {
						w.WriteHeader(405)
						fmt.Println(err)
					}
					r.Body.Close()
					json.Unmarshal(body, &toScan)
					v=strconv.Itoa(toScan.V)
				} else {
					v=varr[0]
				}
				switch v {
					case "1":http.ServeFile(w, r, "pictures/favicon.ico")
					case "2":http.ServeFile(w, r, "pictures/faviconwork.ico")
					default:http.ServeFile(w, r, "pictures/favicon.ico")
				}
			case "/worker":
				ok, sid:=checkSession(r)
				if !ok {
					io.WriteString(w, loginhtml)
				} else {
					if SIDIsAdmin(sid) {
						toSend := fmt.Sprintf(admhtml, sid, sid)
						io.WriteString(w, toSend)
					} else {
						toSend := fmt.Sprintf(WUI, sid)
						io.WriteString(w, toSend)
					}
				}
			case "/Worker.js":
				ok, sid:=checkSession2(r)
				if !ok {
					io.WriteString(w, "You need to login.")
				} else {
					w.Header().Set("content-type", "application/javascript")
					jsoninfo := GetDBInitInfo()
					res := fmt.Sprintf(WUIscript, jsoninfo, sid)
					io.WriteString(w, res)
				}
			case "/admin.js":
				ok, sid:=checkSession2(r)
				if !ok {
					io.WriteString(w, "You need to login.")
				} else {
					w.Header().Set("content-type", "application/javascript")
					if SIDIsAdmin(sid) {
						res := fmt.Sprintf(admjs, sid)
						io.WriteString(w, res)
					} else {
						io.WriteString(w, "You are not admin.")
					}
				}
			case "/GetByIndex":
				ok, _:=checkSession(r)
				if !ok {
					io.WriteString(w, "You need to login.")
				} else {
					var id int
					ids :=r.URL.Query()["id"]
					if len(ids) <= 0 {
						var toScan struct {
							ID int `json:"id"`
						}
						body, err := io.ReadAll(r.Body)
						if err != nil {
							w.WriteHeader(405)
							fmt.Println(err)
						}
						r.Body.Close()
						json.Unmarshal(body, &toScan)
						id=toScan.ID
					} else {
						id, _=strconv.Atoi(ids[0])
					}
					res := GetDBInfoByID(id)
					io.WriteString(w, res)
				}
			case "/login.js":
				io.WriteString(w, loginjs)
			case "/email":
				ok, _:=checkSession(r)
				if !ok {
					io.WriteString(w, "You need to login.")
				} else {
					var id int
					ids :=r.URL.Query()["id"]
					if len(ids) <= 0 {
						var toScan struct {
							ID int `json:"id"`
						}
						body, err := io.ReadAll(r.Body)
						if err != nil {
							w.WriteHeader(405)
							fmt.Println(err)
						}
						r.Body.Close()
						json.Unmarshal(body, &toScan)
						id=toScan.ID
					} else {
						id, _=strconv.Atoi(ids[0])
					}
					
					res := GetEmailByID(id)
					io.WriteString(w, res)
				}
			default:
				w.WriteHeader(405)
		}
		
	} else if r.Method == "POST" {
		switch r.URL.Path {
			case "/":
				body, err := io.ReadAll(r.Body)
				if err != nil {
					w.WriteHeader(405)
					fmt.Println(err)
					fmt.Print("Server:\\>")
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
			case "/auth":
				data, err := io.ReadAll(r.Body)
				r.Body.Close()
				if err != nil {
					io.WriteString(w, "unknown")
					return
				}
				var v UserData
				json.Unmarshal(data, &v)
				if v.Login=="" {
					io.WriteString(w, "unknown")
					return
				}
				ok,uid,adm := checkUser(v)
				if !ok {
					io.WriteString(w, "unknown")
				} else {
					sid:=createSession(uid,adm)
					io.WriteString(w, fmt.Sprint(sid))
				}
			case "/exit":
				ok, sid:=checkSession2(r)
				if ok {
					DeleteSession(sid)
				}
			case "/setStatus":
				ok, sid:=checkSession(r)
				if !ok {
					io.WriteString(w, "You need to login.")
				} else {
					body, err := io.ReadAll(r.Body)
					if err != nil {
						w.WriteHeader(405)
						fmt.Println(err)
						fmt.Print("Server:\\>")
					}
					r.Body.Close()
					var stat StatusChange
					json.Unmarshal(body, &stat)
					stat.ByWhom = GetUID(sid)
					if SettingsV.Server.dbconnect {
						StatReqChan <- stat
						io.WriteString(w, <-StatResChan)
					} else {
						fmt.Printf("Server received status change for id: %d, change status to: %s with comment: %s\n", stat.ID, stat.NewStatus, stat.Comment)
						fmt.Print("Server:\\>")
						io.WriteString(w, "success")
					}
				}
			case "/sendMsg":
				ok, _:=checkSession(r)
				if !ok {
					io.WriteString(w, "You need to login.")
				} else {
					body, err := io.ReadAll(r.Body)
					if err != nil {
						w.WriteHeader(405)
						fmt.Println(err)
						fmt.Print("Server:\\>")
					}
					r.Body.Close()
					var msgI MsgInfo
					json.Unmarshal(body, &msgI)
					if SettingsV.Server.mailconnect {
						from:=SettingsV.Mail.email
						msg := "From: "+from+"\n" + "To: " + msgI.To + "\n" + "Subject: About Your request\n\n" + msgI.Body
						
						err = smtp.SendMail("smtp.gmail.com:587", auth, from, []string{msgI.To}, []byte(msg))
						if err != nil {
							fmt.Println("Received Error while sending massage: ", err)
							fmt.Print("Server:\\>")
							io.WriteString(w, "error")
						} else {
							io.WriteString(w, "success")
						}
					} else {
						fmt.Printf("Server received massage: \"%s\" to send to: %s\n", msgI.Body, msgI.To)
						io.WriteString(w, "success")
						fmt.Print("Server:\\>")
					}
				}
			case "/consoleCMD":
				ok, sid:=checkSession(r)
				if !ok {
					io.WriteString(w, "You need to login.")
				} else {
					if !SIDIsAdmin(sid) {
						io.WriteString(w, "You are not admin.")
					} else {
						cmd, err := io.ReadAll(r.Body)
						if err != nil {
							io.WriteString(w, fmt.Sprint(err))
							return
						}
						r.Body.Close()
						AdmReqChan<-string(cmd)
						res:=<-AdmResChan
						io.WriteString(w, res)
					}
				}
			case "/DBquery":
				ok, sid:=checkSession(r)
				if !ok {
					io.WriteString(w, "You need to login.")
				} else {
					if !SIDIsAdmin(sid) {
						io.WriteString(w, "You are not admin.")
					} else {
						if !SettingsV.Server.dbconnect {
							io.WriteString(w, "DB is not connected.")
							return
						}
						sql, err := io.ReadAll(r.Body)
						if err != nil {
							io.WriteString(w, fmt.Sprint(err))
							return
						}
						r.Body.Close()
						res:=ProcessSQL(string(sql))
						io.WriteString(w, res)
					}
				}
		}
	} else {
		w.WriteHeader(405)
	}
}

func GetUID(SID int) int {
	for _, s := range SessionDB {
		if s.ID == SID {
			return s.userID
		}
	}
	return -1
}

func consoleScanner() {
	<-consoleOut
	stdin:=bufio.NewReader(os.Stdin)
	stdin.Discard(stdin.Buffered())
	for isRunning {
		fmt.Print("Server:\\>")
		str, err := stdin.ReadString('\n')
		if !isRunning {break}
		if err != nil {
			fmt.Println(err)
			continue
		}
		str = strings.TrimSpace(str)
		consoleIn<-str
		res:=<-consoleOut
		fmt.Println(res)
		if !isRunning {break}
	}
}

func ADMScanner() {
	for isRunning {
		select {
			case str:=<-AdmReqChan:
				str = strings.TrimSpace(str)
				adminFormIn<-str
				res:=<-adminFormOut
				AdmResChan<-res
			default:
		}
	}
	done<-true
}

func (stat StatusChange) SetStatus(db *sql.DB) string {
	q:=fmt.Sprintf("UPDATE requests SET Status = '%s', LastComment = '%s' WHERE id = %d;", stat.NewStatus, stat.Comment, stat.ID)
	_, err := db.Exec(q)
	if err != nil {
		return "error"
	} else {
		q=fmt.Sprintf("INSERT INTO audit (WorkerID, Description) VALUES (%d, 'Has updated status of client with id=%d to %s with comment \"%s\"')", stat.ByWhom,stat.ID, stat.NewStatus, stat.Comment)
		db.Exec(q)
		return "success"
	}
}

func SessionDBWorker() {
	for isRunning {
		time.Sleep(10*time.Second)
		for _, s := range SessionDB {
			if time.Now().After(s.expirationDate) {
				DeleteSession(s.ID)
			}
		}
	}
	done <- true
}

func RequestProcesser(db *sql.DB) {
	for len(queue)!=0 || isRunning {
		var ok bool
		if len(queue) == cap(queue) {ok = true}
		select {
			case r := <-queue:
				if ok && len(mutex) == 0 {mutex<-true}
				r.AddToDB(db)
			case stat := <- StatReqChan:
				StatResChan <- stat.SetStatus(db)
			default:
		}
	}
	done <- true
}

func RequestGetter(db *sql.DB) {
	for len(UserReqChan)!=0 || len(InitReqChan)!=0 || len(IDTReqChan)!=0 || len(IDReqChan)!=0 || isRunning {
		time.Sleep(10*time.Nanosecond)
		select {
			case <-InitReqChan:
				r, err := db.Query("SELECT ID, FName, LName, Date, CASE WHEN RepairID=-1 THEN 'Complectation' ELSE 'Repair' END AS REQUEST_TYPE FROM requests WHERE status != 'Ready'")
				if err != nil {
					fmt.Println(err)
					fmt.Println("Server:\\>")
					InitResChan <- nil
				} else {
					InitResChan <- r
				}
			case id:=<-IDTReqChan:
				q:=fmt.Sprintf("SELECT CASE WHEN RepairID=-1 THEN 'Complectation' ELSE 'Repair' END AS REQUEST_TYPE FROM requests WHERE ID=%d", id)
				r, err := db.Query(q)
				if err != nil {
					fmt.Println(err)
					fmt.Println("Server:\\>")
					IDTResChan <- nil
				} else {
					IDTResChan <- r
				}
			case inf:=<-IDReqChan:
				var q string
				switch inf.T {
					case "Complectation": q=fmt.Sprintf("SELECT FName, LName, Email, PhoneNumber, ReceiptType, DeliveryAddress, Status, Date, CaseT, Motherboard, CPU, GPU, RAM, Storage, TermsRequests FROM Complectations_view WHERE ID=%d", inf.id)
					case "Repair": q=fmt.Sprintf("SELECT FName, LName, Email, PhoneNumber, ReceiptType, DeliveryAddress, Status, Date, ComponentType, Model, ProblemDescription FROM Repairs_view WHERE ID=%d", inf.id)
					default: q=""
				}
				r, err := db.Query(q)
				if err != nil {
					fmt.Println(err)
					fmt.Println("Server:\\>")
					IDResChan <- nil
				} else {
					IDResChan <- r
				}
			case user:=<-UserReqChan:
				q:=fmt.Sprintf("SELECT AID, password FROM Accounts WHERE login='%s'", user.Login)
				r, err := db.Query(q)
				if err != nil {
					fmt.Println(err)
					fmt.Println("Server:\\>")
					UserResChan <- nil
				} else {
					UserResChan <- r
				}
			case id:=<-EmailReqChan:
				q:=fmt.Sprintf("SELECT Email FROM requests WHERE id=%d",id)
				r, err := db.Query(q)
				if err != nil {
					fmt.Println(err)
					fmt.Println("Server:\\>")
					EmailResChan <- nil
				} else {
					EmailResChan <- r
				}
			case q:=<-QReqChan:
				r, err := db.Query(q)
				db.Exec("INSERT INTO Audit VALUES (NULL,1,'Used query: "+q+"')")
				QResChan<-RowErr{r,err}
			default:
		}
	}
	done <- true
}

func ProcessSQL(q string) string {
	QReqChan <- q
	re := <-QResChan
	err:=re.err
	r:=re.r
	var res string
	if err != nil {
		res = fmt.Sprint(err)
	} else {
		if !r.Next() {
			res = "Query completed successfully."
		} else {
			res=`<table>`
			columns, _ :=r.Columns()
			res+=`<tr>`
			for _, c := range columns {
				res+=`<th>`+c+`</th>`
			}
			res+=`</tr>`
			for {
				var ress []string = make([]string, len(columns))
				r.Scan((StrToPtr(ress))...)
				res+=`<tr>`
				for i, _ := range columns {
					res+=`<td>`+ress[i]+`</td>`
				}
				res+=`</tr>`
				if !r.Next() {break}
			}
			res+=`</table>`
		}
	}
	return res
}

func StrToPtr(s []string) []any {
	var p []any = make([]any, len(s))
	for i:=range s {
		p[i]=&(s[i])
	}
	return p
}

func GetEmailByID(id int) string {
	var res string
	if SettingsV.Server.dbconnect {
		EmailReqChan <- id
		resRows := <- EmailResChan
		resRows.Next()
		resRows.Scan(&res)
	} else {
		switch id {
			case 0:res="Example1@gmail.com"
			case 1:res="Example2@gmail.com"
			case 3:res="Example4@gmail.com"
			case 2:res="Example3@gmail.com"
			case 4:res="Example5@gmail.com"
			case 5:res="Example6@gmail.com"
			case 6:res="Devil@gmail.com"
			case 7:res="Devil@gmail.com"
			default:res="Error@Error.404"
		}
	}
	return res
}

func GetDBInfoByID(id int) string {
	var res string
	if SettingsV.Server.dbconnect {
		var T string
		IDTReqChan <- id
		resRows := <- IDTResChan
		resRows.Next()
		resRows.Scan(&T)
		IDReqChan <- RInfo{id,T}
		resRows = <- IDResChan
		resRows.Next()
		switch T {
			case "Repair":
				var buff requestRepair
				resRows.Scan(&buff.FName,&buff.LName,&buff.Email,&buff.Phone,&buff.RType,&buff.DAdress,&buff.Status,&buff.Date,&buff.PType,&buff.Model,&buff.Problem)
				var template string = `{"name": "%s %s","email": "%s","reqType": "%s","componentType": "%s","model": "%s","tel": "%s","deliv": "%s","status": "%s","problem": "%s","date": "%s"}`
				var delivery string
				switch buff.RType {
					case "home-delivery": delivery=buff.DAdress
					case "parcel-delivery": delivery="Pakomātā (uz "+buff.DAdress+")"
					case "store-delivery": delivery="Veikalā"
					default: delivery="Veikalā"
				}
				res=fmt.Sprintf(template,buff.FName,buff.LName,buff.Email,T,buff.PType,buff.Model,buff.Phone,delivery,buff.Status,buff.Problem,buff.Date)
			case "Complectation":
				var buff requestAssembly
				resRows.Scan(&buff.FName,&buff.LName,&buff.Email,&buff.Phone,&buff.RType,&buff.DAdress,&buff.Status,&buff.Date, &buff.Case, &buff.Motherboard, &buff.CPU, &buff.GPU, &buff.RAM, &buff.Storage, &buff.Notes)
				var template string = `{"name": "%s %s","email": "%s","reqType": "%s","case": "%s","motherboard": "%s","cpu": "%s","videocard": "%s","ram": "%s","memory": "%s","tel": "%s","deliv": "%s","status": "%s","notes": "%s","date": "%s"}`
				var delivery string
				switch buff.RType {
					case "home-delivery": delivery=buff.DAdress
					case "parcel-delivery": delivery="Pakomātā (uz "+buff.DAdress+")"
					case "store-delivery": delivery="Veikalā"
					default: delivery="Veikalā"
				}
				res=fmt.Sprintf(template,buff.FName,buff.LName,buff.Email,T,buff.Case, buff.Motherboard, buff.CPU, buff.GPU, buff.RAM, buff.Storage,buff.Phone,delivery,buff.Status,buff.Notes,buff.Date)
			default:
				fmt.Println("Is this Riekstiņš order?")
				fmt.Print("Server:\\>")
				res=`{"name": "Error","email": "Error","reqType": "Error","componentType": "Error","model": "Error","tel": "Error","deliv": "Error","status": "Error","problem": "Error"}`
		}
	} else {
		date:=fmt.Sprint(time.Now())[:10]
		templateRep := `{"name": "%s %s","email": "%s","reqType": "%s","componentType": "%s","model": "%s","tel": "%s","deliv": "%s","status": "%s","problem": "%s","date": "%s"}`
		templateComp := `{"name": "%s %s","email": "%s","reqType": "%s","case": "%s","motherboard": "%s","cpu": "%s","videocard": "%s","ram": "%s","memory": "%s","tel": "%s","deliv": "%s","status": "%s","notes": "%s","date": "%s"}`
		switch id {
			case 0:res=fmt.Sprintf(templateRep,"FName1","LName1","Example1@gmail.com","Repair","CT1","Mod1","+371 00000000","Number1 Street, Town1","pending","Some description 1",date)
			case 1:res=fmt.Sprintf(templateRep,"FName2","LName2","Example2@gmail.com","Repair","CT2","Mod2","+371 11111111","Pakomātā (uz Number2 Street, Town2)","processing","Some description 2",date)
			case 3:res=fmt.Sprintf(templateRep,"FName4","LName4","Example4@gmail.com","Repair","CT3","Mod3","+371 22222222","Veikalā","canceled","Some description 3",date)
			case 2:res=fmt.Sprintf(templateComp,"FName3","LName3","Example3@gmail.com","Complectation","case1","mother1","cpu1","gpu1","ram1","mem1","+371 33333333","Veikalā","pending","Some description 1",date)
			case 4:res=fmt.Sprintf(templateComp,"FName5","LName5","Example5@gmail.com","Complectation","case2","mother2","cpu2","gpu2","ram2","mem2","+371 44444444","Number5 Street, Town9","delivering","Some description 2",date)
			case 5:res=fmt.Sprintf(templateComp,"FName6","LName6","Example6@gmail.com","Complectation","case3","mother3","cpu3","gpu3","ram3","mem3","+371 55555555","Veikalā","waiting","Some description 3",date)
			case 6:res=fmt.Sprintf(templateRep,"Aigars","Riekstiņš","Devil@gmail.com","Repair","compType","Model","+666666","Class No9","pending","","2020-09-01")
			case 7:res=fmt.Sprintf(templateComp,"Aigars","Riekstiņš","Devil@gmail.com","Complectation","CaseName","MoboName","cpuName","gpuName","ramName","memoryName","+666666","Veikalā","pending","","2020-09-01")
			default:res=fmt.Sprintf(templateRep,"Error","Error","Error@Error.404","Repair","Error","Error","+Error","Error street","error","Error!","Error-Error-Error")
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
		resRows := <- InitResChan
		for resRows.Next() {
			var res BasicInfo
			resRows.Scan(&res.ID,&res.FName,&res.LName,&res.CreationDate,&res.RType)
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
			case "Repair":tp="Detaļas Remonts"
			case "Complectation":tp="Datora Komplektēšana"
			default:tp="This is totaly Riekstiņš order!"
		}
		resjson+=fmt.Sprintf(template, tp, src.CreationDate, src.FName, src.LName, src.ID)
	}
	if len(ress)>0 {
		resjson = resjson[:len(resjson)-1]
	}
	return resjson
}
