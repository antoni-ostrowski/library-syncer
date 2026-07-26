package web

import (
	"log"
	"net/http"

	"github.com/antoni-ostrowski/library-syncer/internal/web/views"
)

func SetupServer() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		views.Base(views.Index()).Render(r.Context(), w)
	})

	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatalln("server error: ", err)
	}

}
