package main

import (
	"flag"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/boltdb/bolt"
)

var addr = flag.String("addr", ":8080", "http service address")

type Mapping struct {
	FromIri string
	ToIri   string
}
type apiHandlerV1 struct {
	path    string
	baseIri string
	create  func(string, string, *bolt.DB) error
	db      *bolt.DB
}
type adminHandlerV1 struct {
	path  string
	fetch func(string, *bolt.DB) ([]*Mapping, error)
	db    *bolt.DB
}
type appHandlerV1 struct {
	path    string
	baseIri string
	fetch   func(string, *bolt.DB) ([]*Mapping, error)
	db      *bolt.DB
}

func main() {
	flag.Parse()
	db, err := initDB("bolt.db")
	if err != nil {
		log.Fatal("BoltDB: ", err)
	}
	defer db.Close()
	mux := http.NewServeMux()
	baseIriV1 := "https://riri.cip.li"

	apiV1 := &apiHandlerV1{path: "/api/v1/mappings", baseIri: baseIriV1, create: createV1, db: db}
	mux.Handle(apiV1.path, apiV1)

	adminV1 := &adminHandlerV1{path: "/admin", fetch: fetchV1, db: db}
	mux.Handle(adminV1.path, adminV1)

	appV1 := &appHandlerV1{path: "/_/", baseIri: baseIriV1, fetch: fetchV1, db: db}
	mux.Handle(appV1.path, appV1)

	mux.HandleFunc("/assets/", func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method + " " + r.URL.String())
		base := filepath.Base(r.URL.Path)
		http.ServeFile(w, r, filepath.Join("assets", base))
	})

	server := &http.Server{Addr: *addr, Handler: mux}
	err = server.ListenAndServe()
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
func (h *apiHandlerV1) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method + " " + r.URL.String())

	switch r.Method {
	case "PATCH":
		fromIri := r.FormValue("fromIri")
		toIri := r.FormValue("toIri")
		if toIri == "" {
			http.Error(w, "That's not how you use this service :-)", http.StatusBadRequest)
			return
		}
		// Create new mapping if fromIri is empty
		if fromIri == "" {
			if err := h.create(h.baseIri, toIri, h.db); err != nil {
				log.Printf("Error adding new mapping to DB: %v", err)
				http.Error(w, "Whoops! Our bad", http.StatusInternalServerError)
				return
			}
			return
		}
		if err := updateMapping(h.db, fromIri, toIri); err != nil {
			log.Printf("Error updating DB: %v", err)
			http.Error(w, "Whoops! Our bad", http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "That's not how you use this service :-)", http.StatusBadRequest)
		return
	}
}
func (h *adminHandlerV1) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method + " " + r.URL.String())

	tmpl, err := template.ParseFiles("index.html")
	if err != nil {
		log.Printf("Error making template: %v", err)
		http.Error(w, "Whoops! Our bad", http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case "GET":
		result, err := h.fetch("", h.db)
		if err != nil {
			log.Printf("Error fetching from DB: %v", err)
			http.Error(w, "Whoops! Our bad", http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, result)
		return
	default:
		http.Error(w, "That's not how you use this service :-)", http.StatusBadRequest)
		return
	}
}
func (h *appHandlerV1) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method + " " + r.URL.String())

	switch r.Method {
	case "GET":
		result, err := h.fetch(h.baseIri+r.URL.String(), h.db)
		if err != nil {
			log.Printf("Error fetching from DB: %v", err)
			http.Error(w, "Whoops! Our bad", http.StatusInternalServerError)
			return
		}
		if len(result) == 0 {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, result[0].ToIri, http.StatusTemporaryRedirect)
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
		_, err := tx.CreateBucketIfNotExists([]byte("riris"))
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
func fetchV1(key string, db *bolt.DB) ([]*Mapping, error) {
	var result []*Mapping
	err := db.View(func(tx *bolt.Tx) error {
		riris := tx.Bucket([]byte("riris"))
		if key != "" {
			v := riris.Get([]byte(key))
			if len(v) > 0 {
				result = append(result, &Mapping{key, string(v[:])})
			}
			return nil
		}
		c := riris.Cursor()
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			result = append(result, &Mapping{string(k[:]), string(v[:])})
		}
		return nil
	})
	return result, err
}
func createV1(baseIri, value string, db *bolt.DB) error {
	err := db.Update(func(tx *bolt.Tx) error {
		riris := tx.Bucket([]byte("riris"))
		id, _ := riris.NextSequence()
		// Example of a key:  https://riri.cip.li/_/1
		key := baseIri + "/_/" + strconv.FormatUint(id, 16)
		if err := riris.Put([]byte(key), []byte(value)); err != nil {
			return err
		}
		return nil
	})
	return err
}
func updateMapping(db *bolt.DB, key, value string) error {
	err := db.Update(func(tx *bolt.Tx) error {
		riris := tx.Bucket([]byte("riris"))
		if err := riris.Put([]byte(key), []byte(value)); err != nil {
			return err
		}
		return nil
	})
	return err
}
