package web

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/antoni-ostrowski/library-syncer/internal/db"
	"github.com/antoni-ostrowski/library-syncer/internal/parser"
	"github.com/antoni-ostrowski/library-syncer/internal/web/views"
)

func StartHttpServer(db *db.DbService) {

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		trackers, err := db.ListTrackers(r.Context())
		if err != nil {
			views.Error(err.Error()).Render(r.Context(), w)
		}
		views.Base(views.Index(views.IndexModel{TrackerListModel: views.TrackerListModel{Trackers: trackers}})).Render(r.Context(), w)
	})

	http.HandleFunc("/tracker-list", func(w http.ResponseWriter, r *http.Request) {
		trackers, err := db.ListTrackers(r.Context())
		if err != nil {
			views.Error(err.Error()).Render(r.Context(), w)
		}

		views.TrackerList(views.TrackerListModel{Trackers: trackers}).Render(r.Context(), w)
	})

	http.HandleFunc("/tracker", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid form", http.StatusBadRequest)
				return
			}
			readRanges := strings.Fields(r.FormValue("readRanges"))
			mapping := parser.TrackerMapping{
				Era:            r.FormValue("mapping_era"),
				Name:           r.FormValue("mapping_name"),
				Notes:          r.FormValue("mapping_notes"),
				FileDate:       r.FormValue("mapping_fileDate"),
				Type:           r.FormValue("mapping_type"),
				AvailableLen:   r.FormValue("mapping_availableLen"),
				Quality:        r.FormValue("mapping_quality"),
				Links:          r.FormValue("mapping_links"),
				FirstPreview:   r.FormValue("mapping_firstPreview"),
				LeakDate:       r.FormValue("mapping_leakDate"),
				OGFileLeakDate: r.FormValue("mapping_ogFileLeakDate"),
			}
			newTracker := parser.NewTracker(r.FormValue("artist"), r.FormValue("id"), readRanges, mapping, "idle")
			if err := db.UpsertTracker(r.Context(), newTracker); err != nil {
				fmt.Printf("upsert tracker failed: %v\n", err)
				http.Error(w, "failed to save tracker", http.StatusInternalServerError)
				return
			}
			views.Tracker(newTracker).Render(r.Context(), w)
		case http.MethodDelete:
			id := r.URL.Query().Get("id")
			if id == "" {
				fmt.Printf("no tracker id provided\n")
				http.Error(w, "failed to delete tracker: no id provided", http.StatusBadRequest)
			}
			if err := db.DeleteTracker(r.Context(), id); err != nil {
				fmt.Printf("tracker deletion failed: %v\n", err)
				http.Error(w, "failed to delete tracker", http.StatusInternalServerError)
			}
		}
	})

	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatalln("server error: ", err)
	}
}
