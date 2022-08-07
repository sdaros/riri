package main

import (
	"flag"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"

	"github.com/boltdb/bolt"
)

var addr = flag.String("addr", ":8080", "http service address")

type Mapping struct {
	From string
	To   *url.URL
}
type App struct {
	db *bolt.DB
}

func main() {
	flag.Parse()
	db, err := initDB("bolt.db")
	if err != nil {
		log.Fatal("BoltDB: ", err)
	}
	defer db.Close()

	app := &App{db}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/mappings", app.apiHandler)
	mux.HandleFunc("/admin", app.adminHandler)
	mux.HandleFunc("/assets/", app.assetsHandler)
	mux.HandleFunc("/", app.rootHandler)
	server := &http.Server{Addr: *addr, Handler: mux}
	err = server.ListenAndServe()
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func (a *App) adminHandler(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method + " " + r.URL.String())

	tmpl, err := template.ParseFiles("index.html")
	if err != nil {
		log.Printf("Error making template: %v", err)
		http.Error(w, "Whoops! Our bad", http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case "GET":
		result, err := fetch("", a.db)
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

func (a *App) rootHandler(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method + " " + r.URL.String())

	switch r.Method {
	case "GET":
		result, err := fetch(filepath.Base(r.URL.EscapedPath()), a.db)
		if err != nil {
			log.Printf("Error fetching from DB: %v", err)
			http.Error(w, "Whoops! Our bad", http.StatusInternalServerError)
			return
		}
		if len(result) == 0 {
			http.NotFound(w, r)
			return
		}
		// Append Query Values from request to URL Query from result
		queryValuesFromRequest := r.URL.Query()
		queryValuesFromResult := result[0].To.Query()
		for k, v := range queryValuesFromRequest {
			for i := 0; i < len(v); i++ {
				queryValuesFromResult.Add(k, v[i])
			}
		}
		result[0].To.RawQuery = queryValuesFromResult.Encode()
		http.Redirect(w, r, result[0].To.String(), http.StatusTemporaryRedirect)
	default:
		http.Error(w, "That's not how you use this service :-)", http.StatusBadRequest)
		return
	}
}

func (a *App) apiHandler(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method + " " + r.URL.String())

	switch r.Method {
	case "PATCH":
		toIriUnescaped, err := url.QueryUnescape(r.FormValue("toIri"))
		if err != nil {
			log.Printf("Error trying to unescape `to IRI` field: %v", err)
			http.Error(w, "That's not how you use this service :-)", http.StatusBadRequest)
			return
		}
		toIri, err := url.Parse(toIriUnescaped)
		if err != nil {
			log.Printf("Error trying to parse `to IRI` field: %v", err)
			http.Error(w, "That's not how you use this service :-)", http.StatusBadRequest)
			return
		}
		if toIri.String() == "" {
			http.Error(w, "That's not how you use this service :-)", http.StatusBadRequest)
			return
		}
		// Create new mapping if fromIri is empty
		if r.FormValue("fromIri") == "" {
			if err := create(toIri, a.db); err != nil {
				log.Printf("Error adding new mapping to DB: %v", err)
				http.Error(w, "Whoops! Our bad", http.StatusInternalServerError)
				return
			}
			return
		}
		if err := updateMapping(a.db, r.FormValue("fromIri"), toIri); err != nil {
			log.Printf("Error updating DB: %v", err)
			http.Error(w, "Whoops! Our bad", http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "That's not how you use this service :-)", http.StatusBadRequest)
		return
	}
}

func (a *App) assetsHandler(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method + " " + r.URL.String())
	base := filepath.Base(r.URL.Path)
	http.ServeFile(w, r, filepath.Join("assets", base))
}

func fetch(key string, db *bolt.DB) ([]*Mapping, error) {
	var result []*Mapping
	err := db.View(func(tx *bolt.Tx) error {
		riris := tx.Bucket([]byte("riris"))
		if key != "" {
			v := riris.Get([]byte(key))
			if len(v) > 0 {
				toUrl, err := url.Parse(string(v[:]))
				if err != nil {
					return err
				}
				result = append(result, &Mapping{key, toUrl})
			}
			return nil
		}
		c := riris.Cursor()
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			toUrl, err := url.Parse(string(v[:]))
			if err != nil {
				return err
			}
			result = append(result, &Mapping{string(k[:]), toUrl})
		}
		return nil
	})
	return result, err
}
func create(value *url.URL, db *bolt.DB) error {
	err := db.Update(func(tx *bolt.Tx) error {
		riris := tx.Bucket([]byte("riris"))
		id, _ := riris.NextSequence()
		key := strconv.FormatUint(id, 10)
		if err := riris.Put([]byte(key), []byte(value.String())); err != nil {
			return err
		}
		return nil
	})
	return err
}
func updateMapping(db *bolt.DB, key string, value *url.URL) error {
	err := db.Update(func(tx *bolt.Tx) error {
		riris := tx.Bucket([]byte("riris"))
		if err := riris.Put([]byte(key), []byte(value.String())); err != nil {
			return err
		}
		return nil
	})
	return err
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
