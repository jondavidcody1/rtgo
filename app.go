//    Title: app.go
//    Author: Jon Cody
//
//    This program is free software: you can redistribute it and/or modify
//    it under the terms of the GNU General Public License as published by
//    the Free Software Foundation, either version 3 of the License, or
//    (at your option) any later version.
//
//    This program is distributed in the hope that it will be useful,
//    but WITHOUT ANY WARRANTY; without even the implied warranty of
//    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//    GNU General Public License for more details.
//
//    You should have received a copy of the GNU General Public License
//    along with this program.  If not, see <http://www.gnu.org/licenses/>.

package rtgo

import (
    "crypto/rand"
    "crypto/sha1"
    "crypto/sha256"
    "encoding/json"
    "fmt"
    "github.com/chuckpreslar/emission"
    "github.com/gorilla/securecookie"
    "github.com/satori/go.uuid"
    "github.com/tpjg/goriakpbc"
    "html/template"
    "io/ioutil"
    "log"
    "net/http"
    "regexp"
    "strconv"
    "strings"
)

type App struct {
    Port        string
    Proxy       string
    Sslkey      string
    Sslcrt      string
    Cookiename  string
    Hashkey     string
    Blockkey    string
    Scook       *securecookie.SecureCookie
    Templates   *template.Template
    Emitter     *emission.Emitter
    Handlers    map[string]func(w http.ResponseWriter, r *http.Request)
    Database    map[string]map[string]string
    Routes      map[string]map[string]string
    ConnManager map[string]*Conn
    RoomManager map[string]*Room
    DB          *Database
}

func (a *App) ReadCookieHandler(w http.ResponseWriter, r *http.Request, cookname string) map[string]string {
    cookie, err := r.Cookie(cookname)
    if err != nil {
        return nil
    }
    cookvalue := make(map[string]string)
    if err := a.Scook.Decode(cookname, cookie.Value, &cookvalue); err != nil {
        return nil
    }
    return cookvalue
}

func (a *App) SetCookieHandler(w http.ResponseWriter, r *http.Request, cookname string, cookvalue map[string]string) {
    encoded, err := a.Scook.Encode(cookname, cookvalue)
    if err != nil {
        return
    }
    cookie := &http.Cookie{
        Name:  cookname,
        Value: encoded,
        Path:  "/",
    }
    http.SetCookie(w, cookie)
    return
}

func (a *App) RegisterHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "Invalid request method.", 405)
        return
    }
    username := r.FormValue("username")
    email := r.FormValue("email")
    password := r.FormValue("password")
    if _, err := a.DB.GetObj("users", username); err == nil {
        w.WriteHeader(500)
        return
    }
    randombytes := make([]byte, 16)
    if _, err := rand.Read(randombytes); err != nil {
        w.WriteHeader(500)
        return
    }
    salt := fmt.Sprintf("%x", sha1.Sum(randombytes))
    hashstring := []byte(fmt.Sprintf("%s%s%s%s", username, email, password, salt))
    passhash := fmt.Sprintf("%x", sha256.Sum256(hashstring))
    obj := map[string]string{
        "username":  username,
        "passhash":  passhash,
        "email":     email,
        "salt":      salt,
        "privilege": "user",
    }
    if err := a.DB.InsertObj("users", username, obj); err != nil {
        w.WriteHeader(500)
        return
    }
    a.SetCookieHandler(w, r, a.Cookiename, map[string]string{
        "username":  username,
        "privilege": "user",
    })
    w.WriteHeader(200)
}

func (a *App) LoginHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "Invalid request method.", 405)
        return
    }
    username := r.FormValue("username")
    password := r.FormValue("password")
    initial, err := a.DB.GetObj("users", username)
    if err != nil {
        w.WriteHeader(500)
        return
    }
    result := initial.(map[string]string)
    passhash := sha256.Sum256([]byte(fmt.Sprintf("%s%s%s%s", username, result["email"], password, result["salt"])))
    if fmt.Sprintf("%x", passhash) == result["passhash"] {
        a.SetCookieHandler(w, r, a.Cookiename, map[string]string{
            "username":  username,
            "privilege": result["privilege"],
        })
        w.WriteHeader(200)
        return
    }
    w.WriteHeader(500)
}

func (a *App) BaseHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != "GET" {
        http.Error(w, "Method not allowed", 405)
        return
    }
    a.SetCookieHandler(w, r, a.Cookiename, map[string]string{
        "username":  "guest",
        "privilege": "user",
    })
    a.Templates.ExecuteTemplate(w, "base", nil)
}

func (a *App) StaticHandler(w http.ResponseWriter, r *http.Request) {
    http.ServeFile(w, r, r.URL.Path[1:])
}

func (a *App) SocketHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != "GET" {
        http.Error(w, "Method not allowed", 405)
        return
    }
    c, err := a.NewConnection(w, r)
    if err != nil {
        log.Println(err)
        return
    }
    go c.WritePump()
    c.Join("root")
    c.ReadPump()
}

func (a *App) FindRoute(path string) map[string]string {
    route := make(map[string]string)
    if _, ok := a.Routes[path]; ok {
        return a.Routes[path]
    }
    for key, _ := range a.Routes {
        if !strings.HasPrefix(key, "^") {
            continue
        }
        reg, err := regexp.Compile(key)
        if err != nil {
            continue
        }
        match := reg.FindStringSubmatch(path)
        if match == nil || len(match) == 0 {
            continue
        }
        for k, val := range a.Routes[key] {
            if !strings.HasPrefix(val, "$") {
                route[k] = val
                continue
            }
            index, err := strconv.Atoi(string(val[1]))
            if err != nil {
                continue
            }
            route[k] = match[index]
        }
    }
    return route
}

func (a *App) NewConnection(w http.ResponseWriter, r *http.Request) (*Conn, error) {
    cookie := a.ReadCookieHandler(w, r, a.Cookiename)
    socket, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return nil, err
    }
    c := &Conn{
        Application: a,
        Socket:      socket,
        Id:          uuid.NewV4().String(),
        Send:        make(chan []byte, 256),
        Rooms:       make(map[string]*Room),
        Privilege:   cookie["privilege"],
    }
    a.ConnManager[c.Id] = c
    return c, nil
}

func (a *App) NewRoom(name string) *Room {
    r := &Room{
        Application: a,
        Name:        name,
        Members:     make(map[string]*Conn),
        Stopchan:    make(chan bool),
        Joinchan:    make(chan *Conn),
        Leavechan:   make(chan *Conn),
        Send:        make(chan *RoomMessage),
    }
    go r.Start()
    a.RoomManager[name] = r
    return r
}

func (a *App) NewDatabase(name string, params map[string]string) *Database {
    dsn := ""
    create := ""
    switch name {
    case "riak":
        dsn = fmt.Sprintf("%s:%s", params["host"], params["port"])
        create = ""
    case "postgres":
        dsn = fmt.Sprintf("dbname=%s user=%s password=%s host=%s sslmode=%s fallback_application_name=%s connect_timeout=%s sslcert=%s sslkey=%s sslrootcert=%s", params["dbname"], params["user"], params["password"], params["host"], params["sslmode"], params["fallback_application_name"], params["connect_timeout"], params["sslcert"], params["sslkey"], params["sslrootcert"])
        create = "CREATE TABLE IF NOT EXISTS %s (hash VARCHAR(255) NOT NULL UNIQUE PRIMARY KEY, data BYTEA)"
    case "mysql":
        dsn = fmt.Sprintf("%s:%s@%s/%s?allowAllFiles=%s&allowCleartextPasswords=%s&allowOldPasswords=%s&charset=%s&collation=%s&clientFoundRows=%s&loc=%s&parseTime=%s&strict=%s&timeout=%s&tls=%s", params["user"], params["password"], params["host"], params["dbname"], params["allowAllFiles"], params["allowCleartextPasswords"], params["allowOldPasswords"], params["charset"], params["collation"], params["clientFoundRows"], params["loc"], params["parseTime"], params["strict"], params["timeout"], params["tls"])
        create = "CREATE TABLE IF NOT EXISTS %s (hash VARCHAR(255) NOT NULL UNIQUE PRIMARY KEY, data LONGBLOB)"
    case "sqlite3":
        dsn = fmt.Sprintf("%s", params["file"])
        create = "CREATE TABLE IF NOT EXISTS %s (hash VARCHAR(255) NOT NULL UNIQUE PRIMARY KEY, data BLOB)"
    }
    db := &Database{
        Application: a,
        Name:        name,
        Buckets:     make(map[string]*riak.Bucket),
        Params:      params,
        Dsn:         dsn,
        Create:      create,
    }
    db.Start()
    a.DB = db
    return db
}

func (a *App) Parse(filepath string) {
    var (
        hashkey  []byte
        blockkey []byte
    )
    file, err := ioutil.ReadFile(filepath)
    if err != nil {
        log.Fatal("Could not parse config.json: ", err)
    }
    if err := json.Unmarshal(file, a); err != nil {
        log.Fatal("Error parsing config.json: ", err)
    }
    if a.Hashkey == "" {
        hashkey = securecookie.GenerateRandomKey(16)
    } else {
        hashkey = []byte(a.Hashkey)
    }
    if a.Blockkey == "" {
        blockkey = securecookie.GenerateRandomKey(16)
    } else {
        blockkey = []byte(a.Blockkey)
    }
    a.Scook = securecookie.New(hashkey, blockkey)
    a.Templates = template.Must(template.ParseGlob("./static/views/*"))
}

func (a *App) AddHandler(route string, handler func(w http.ResponseWriter, r *http.Request)) {
    if _, ok := a.Handlers[route]; !ok {
        a.Handlers[route] = handler
    }
}

func (a *App) Start() {
    for dbase, params := range a.Database {
        a.NewDatabase(dbase, params)
        break
    }
    http.HandleFunc("/", a.BaseHandler)
    http.HandleFunc("/login", a.LoginHandler)
    http.HandleFunc("/register", a.RegisterHandler)
    http.HandleFunc("/ws", a.SocketHandler)
    http.HandleFunc("/static/", a.StaticHandler)
    for route, handler := range a.Handlers {
        http.HandleFunc(route, handler)
    }
    if a.Sslcrt != "" && a.Sslkey != "" {
        log.Fatal(http.ListenAndServeTLS(":"+a.Port, a.Sslcrt, a.Sslkey, nil))
    } else {
        log.Fatal(http.ListenAndServe(":"+a.Port, nil))
    }
}

func NewApp() *App {
    app := &App{
        Emitter:     emission.NewEmitter(),
        Handlers:    make(map[string]func(w http.ResponseWriter, r *http.Request)),
        ConnManager: make(map[string]*Conn),
        RoomManager: make(map[string]*Room),
    }
    return app
}
