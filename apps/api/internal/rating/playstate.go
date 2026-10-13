package rating

// The flat play states the form offers, the same vocabulary the forum uses.
// Catalog keeps them on two axes: a state, and for done how much was finished.
const (
	Wish         = "wish"
	Doing        = "doing"
	DoneOneRoute = "done_one_route"
	DoneMain     = "done_main"
	DoneAll      = "done_all"
	OnHold       = "on_hold"
	Dropped      = "dropped"

	// Finished with how much left unstated, which another application can
	// record. The form never offers it, but sends it back as it found it: read
	// as one route it was rewritten on every site by a save that only added a
	// rating.
	Done = "done"
)

func toCatalogState(flat string) (state, completion string, ok bool) {
	switch flat {
	case Wish, Doing, OnHold, Dropped, Done:
		return flat, "", true
	case DoneOneRoute:
		return "done", "one_route", true
	case DoneMain:
		return "done", "main", true
	case DoneAll:
		return "done", "all", true
	}
	return "", "", false
}

func fromCatalogState(state string, completion *string) string {
	switch state {
	case "wish", "doing", "on_hold", "dropped":
		return state
	case "done":
		if completion == nil {
			return Done
		}
		switch *completion {
		case "one_route":
			return DoneOneRoute
		case "main":
			return DoneMain
		case "all":
			return DoneAll
		}
		return Done
	}
	return ""
}
