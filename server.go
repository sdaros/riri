// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"html/template"
	"log"
	"net/http"

	"github.com/boltdb/bolt"
)

var (
	addr = flag.String("addr", ":8080", "http service address")
)

func main() {
	flag.Parse()
	db, err := initDB("urlshare.db")
	if err != nil {
		log.Fatal("BoltDB: ", err)
	}
	defer db.Close()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		serveApp(w, r, db)
	})
	err = http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
func serveApp(w http.ResponseWriter, r *http.Request, db *bolt.DB) {
	log.Println(r.Method + " " + r.URL.String())

	tmpl, err := template.ParseFiles("index.html")
	if err != nil {
		http.Error(w, "Whoops! Our bad", http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case "GET":
		result, err := fetchFromDB(db, "1")
		if err != nil {
			log.Printf("Error fetching from DB: %v", err)
			http.Error(w, "Whoops! Our bad", http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, result["1"])
		return
	case "POST":
		urls := r.FormValue("urls")
		if urls == "" {
			http.Error(w, "That's not how you use this service :-)", http.StatusBadRequest)
			return
		}
		if err := updateDB(db, "1", urls); err != nil {
			log.Printf("Error updating DB: %v", err)
			http.Error(w, "Whoops! Our bad", http.StatusInternalServerError)
			return
		}
		log.Println("Sync changes to DB")

		result, err := fetchFromDB(db, "1")
		if err != nil {
			log.Printf("Error fetching from DB: %v", err)
			http.Error(w, "Whoops! Our bad", http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, result["1"])
		return
	default:
		http.Error(w, "That's not how you use this service :-)", http.StatusBadRequest)
		return
	}
}
func initDB(pathToDB string) (*bolt.DB, error) {
	db, err := bolt.Open(pathToDB, 0600, nil)
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("urls"))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}
func updateDB(db *bolt.DB, key, value string) error {
	err := db.Update(func(tx *bolt.Tx) error {
		urls := tx.Bucket([]byte("urls"))
		if err := urls.Put([]byte(key), []byte(value)); err != nil {
			return err
		}
		return nil
	})
	return err
}
func fetchFromDB(db *bolt.DB, key string) (map[string]string, error) {
	result := make(map[string]string)
	err := db.View(func(tx *bolt.Tx) error {
		urls := tx.Bucket([]byte("urls"))
		url := urls.Get([]byte(key))
		result[key] = string(url[:])
		return nil
	})
	return result, err
}
