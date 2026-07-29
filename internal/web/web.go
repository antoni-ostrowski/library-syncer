package web

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/antoni-ostrowski/library-syncer/internal/events"
	"github.com/antoni-ostrowski/library-syncer/internal/web/views"
)

type State struct {
	mu            sync.RWMutex
	TrackersState map[string]string
}

func StartHttpServer(eventsCh chan events.Event) {
	state := State{TrackersState: make(map[string]string)}
	go func() {
		for e := range eventsCh {
			switch e.Tracker {
			case string(events.SyncStarted):
				fmt.Printf("%v tracker state update: %v\n", e.Tracker, e.Type)
				state.mu.Lock()
				if v, ok := state.TrackersState[e.Tracker]; ok {
					state.TrackersState[e.Tracker] = v
				}
				state.mu.Unlock()

			}

		}

	}()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		views.Base(views.Index()).Render(r.Context(), w)
		fmt.Fprintf(w, "hello")
	})

	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatalln("server error: ", err)
	}
}
