package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/context"
	mgo "gopkg.in/mgo.v2"
)

type Note struct {
	Text string    `json:"text" bson:"text"`
	User string    `json:"user" bson:"user"`
	When time.Time `json:"when" bson:"when"`
}

type Adapter func(http.Handler) http.Handler

func main() {
	// connect to the database
	db, err := mgo.Dial("mongodb://localhost:27017")
	if err != nil {
		log.Fatal("cannot dial mongo", err)
	}
	defer db.Close() // clean up when we’re done
	// Adapt our handle function using withDB
	h := Adapt(http.HandlerFunc(handle), withDB(db))
	// add the handler
	http.Handle("/notes", context.ClearHandler(h))
	// start the server
	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatal(err)
	}
}

func handle(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getNotes(w, r)
	case "POST":
		createNote(w, r)
	default:
		http.Error(w, "Not supported", http.StatusMethodNotAllowed)
	}
}

func createNote(w http.ResponseWriter, r *http.Request) {
	// decode the request body
	log.Println("createNote")
	var note Note
	err := r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	note.Text = r.Form["text"][0]
	note.User = r.Form["user"][0]
	note.When = time.Now()

	log.Println("Note", note)

	// Hook up to Db
	db := context.Get(r, "database").(*mgo.Session)
	// insert it into the database
	if err := db.DB("test").C("gonotes").Insert(&note); err != nil {
		log.Println("Failed insert")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Println("Insert OK")
}

func getNotes(w http.ResponseWriter, r *http.Request) {
	log.Println("getNotes")

	// Hook up to Db
	db := context.Get(r, "database").(*mgo.Session)
	// Read notes and return
	var notes []*Note
	err := db.DB("test").C("gonotes").Find(nil).Sort("-when").All(&notes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	err = json.NewEncoder(w).Encode(notes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func Adapt(h http.Handler, adapters ...Adapter) http.Handler {
	for _, adapter := range adapters {
		h = adapter(h)
	}
	return h
}

func withDB(db *mgo.Session) Adapter {
	// return the Adapter
	return func(h http.Handler) http.Handler {
		// the adapter (when called) should return a new handler
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// copy the database session
			dbsession := db.Copy()
			defer dbsession.Close() // clean up
			// save it in the mux context
			context.Set(r, "database", dbsession)
			// pass execution to the original handler
			h.ServeHTTP(w, r)
		})
	}
}
