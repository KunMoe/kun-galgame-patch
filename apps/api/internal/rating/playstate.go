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

	// Done is read-only: catalog answers state=done with no completion when
	// another application reported a finish without saying how much.
	Done = "done"
)

func toCatalogState(flat string) (state, completion string, ok bool) {
	switch flat {
	case Wish, Doing, OnHold, Dropped:
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
