package service

import (
	"net/http/httptest"
	"testing"

	"kun-galgame-patch-api/pkg/problem"

	"github.com/gofiber/fiber/v3"
)

func TestChangeCursorRoundTrip(t *testing.T) {
	app := fiber.New()
	var got ChangeCursor
	var bad bool
	app.Get("/", func(c fiber.Ctx) error {
		var prob *problem.Problem
		got, prob = ParseChangeCursor(c)
		bad = prob != nil
		return nil
	})
	parse := func(cursor string) {
		t.Helper()
		got, bad = ChangeCursor{}, false
		if _, err := app.Test(httptest.NewRequest("GET", "/?cursor="+cursor, nil)); err != nil {
			t.Fatal(err)
		}
	}

	want := ChangeCursor{Txid: 7340021, Seq: 918}
	parse(*want.Encode())
	if bad || got != want {
		t.Fatalf("round trip = %+v (bad %v), want %+v", got, bad, want)
	}
	for _, cursor := range []string{"cur_", "918", "cur_" + "OTE4", "cur_LTEuMg"} {
		parse(cursor)
		if !bad {
			t.Errorf("cursor %q parsed as %+v, want INVALID_CURSOR", cursor, got)
		}
	}
}
