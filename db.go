//    Title: db.go
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
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/tpjg/goriakpbc"
	"log"
	"strings"
)

type Database struct {
	Application *App
	Name        string
	Params      map[string]string
	Buckets     map[string]*riak.Bucket
	Dsn         string
	Create      string
	Connection  *sql.DB
}

func (db *Database) GetAllObjs(table string) ([]interface{}, error) {
	data := make([]interface{}, 0)
	if db.Name == "riak" {
		if _, exists := db.Buckets[table]; !exists {
			return nil, errors.New("Bucket does not exist.")
		}
		keys, err := db.Buckets[table].ListKeys()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			obj, err := db.GetObj(table, string(key))
			if err != nil {
				return nil, err
			}
			data = append(data, obj)
		}
	} else {
		query := fmt.Sprintf("SELECT * FROM %s", table)
		rows, err := db.Connection.Query(query)
		if err != nil {
			return nil, err
		}
		cols, err := rows.Columns()
		if err != nil {
			return nil, err
		}
		blobs := make([][]byte, len(cols))
		dest := make([]interface{}, len(cols))
		for i := range cols {
			dest[i] = &blobs[i]
		}
		for rows.Next() {
			if err := rows.Scan(dest...); err != nil {
				return nil, err
			}
			for i, blob := range blobs {
				var value interface{}
				col := cols[i]
				if col == "hash" {
					continue
				} else if err := json.Unmarshal(blob, &value); err != nil {
					return nil, err
				}
				data = append(data, value)
			}
		}
	}
	return data, nil
}

func (db *Database) GetObj(table string, key string) (interface{}, error) {
	var data interface{}
	if db.Name == "riak" {
		if _, exists := db.Buckets[table]; !exists {
			return nil, errors.New("Bucket does not exist.")
		}
		if exists, _ := db.Buckets[table].Exists(key); !exists {
			return nil, errors.New("Object does not exist.")
		}
		obj, err := db.Buckets[table].Get(key)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(obj.Data, &data); err != nil {
			return nil, err
		}
	} else {
		query := ""
		blob := make([]byte, 0)
		if db.Name == "postgres" {
			query = fmt.Sprintf("SELECT data FROM %s WHERE hash = $1", table)
		} else {
			query = fmt.Sprintf("SELECT data FROM %s WHERE hash = ?", table)
		}
		if err := db.Connection.QueryRow(query, key).Scan(&blob); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(blob, &data); err != nil {
			return nil, err
		}
	}
	return data, nil
}

func (db *Database) DeleteObj(table string, key string) error {
	if db.Name == "riak" {
		if _, exists := db.Buckets[table]; !exists {
			return errors.New("Bucket does not exist.")
		}
		if err := db.Buckets[table].Delete(key); err != nil {
			return err
		}
	} else {
		query := ""
		if db.Name == "postgres" {
			query = fmt.Sprintf("DELETE FROM %s WHERE hash = $1", table)
		} else {
			query = fmt.Sprintf("DELETE FROM %s WHERE hash = ?", table)
		}
		if _, err := db.Connection.Exec(query, key); err != nil {
			return err
		}
	}
	return nil
}

func (db *Database) InsertObj(table string, key string, data interface{}) error {
	blob, err := json.Marshal(&data)
	if err != nil {
		return err
	}
	if db.Name == "riak" {
		if _, exists := db.Buckets[table]; !exists {
			return errors.New("Bucket does not exist.")
		}
		obj := db.Buckets[table].NewObject(key)
		obj.ContentType = "application/json"
		obj.Data = blob
		if err = obj.Store(); err != nil {
			return err
		}
	} else {
		query := ""
		if db.Name == "postgres" {
			query = fmt.Sprintf("INSERT INTO %s (hash, data) VALUES ($1, $2)", table)
		} else {
			query = fmt.Sprintf("INSERT INTO %s (hash, data) VALUES (?, ?)", table)
		}
		if _, err := db.Connection.Exec(query, key, blob); err != nil {
			return err
		}
	}
	return nil
}

func (db *Database) Start() {
	usersTableExists := false
	if db.Name == "riak" {
		if err := riak.ConnectClient(db.Dsn); err != nil {
			log.Fatal("Cannot connect, is Riak running?")
		}
		tableList := strings.Split(db.Params["tables"], ",")
		for _, table := range tableList {
			bname := strings.TrimSpace(table)
			if bname == "users" {
				usersTableExists = true
			}
			db.Buckets[bname], _ = riak.NewBucket(bname)
		}
		if usersTableExists == false {
			db.Buckets["users"], _ = riak.NewBucket("users")
		}
	} else {
		dbconn, err := sql.Open(db.Name, db.Dsn)
		if err != nil {
			log.Fatal(err)
		}
		db.Connection = dbconn
		if _, exists := db.Params["tables"]; !exists {
			return
		}
		tableList := strings.Split(db.Params["tables"], ",")
		for _, table := range tableList {
			tname := strings.TrimSpace(table)
			if tname == "users" {
				usersTableExists = true
			}
			statement := fmt.Sprintf(db.Create, tname)
			if _, err := db.Connection.Exec(statement); err != nil {
				log.Fatal(err)
			}
		}
		if usersTableExists == false {
			statement := fmt.Sprintf(db.Create, "users")
			if _, err := db.Connection.Exec(statement); err != nil {
				log.Fatal(err)
			}
		}
	}
}
