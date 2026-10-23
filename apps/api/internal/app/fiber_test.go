package app

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

// go test -race does not see this class of bug: fasthttp recycles the buffer
// through sync.Pool, which is a happens-before edge, so only the values can tell.
func TestRequestValuesSurviveTheHandler(t *testing.T) {
	app := newFiber()
	var kept [][3]string
	app.Get("/", func(c fiber.Ctx) error {
		kept = append(kept, [3]string{c.Query("q"), c.Cookies("k"), c.Get("X-V")})
		return c.SendStatus(fiber.StatusNoContent)
	})

	for i := range 3 {
		v := strconv.Itoa(i) + "-value-padded"
		req := httptest.NewRequest(http.MethodGet, "/?q=q"+v, nil)
		req.AddCookie(&http.Cookie{Name: "k", Value: "k" + v})
		req.Header.Set("X-V", "h"+v)
		if _, err := app.Test(req); err != nil {
			t.Fatal(err)
		}
	}
	for i, got := range kept {
		v := strconv.Itoa(i) + "-value-padded"
		if want := [3]string{"q" + v, "k" + v, "h" + v}; got != want {
			t.Errorf("request %d kept %q, want %q", i, got, want)
		}
	}
}

type peerConn struct {
	net.Conn
	peer net.Addr
}

func (c peerConn) RemoteAddr() net.Addr { return c.peer }

func clientIP(t *testing.T, peer string, header string) string {
	t.Helper()
	app := newFiber()
	app.Get("/", func(c fiber.Ctx) error { return c.SendString(c.IP()) })
	server, client := net.Pipe()
	go func() {
		_ = (&fasthttp.Server{Handler: app.Handler()}).ServeConn(
			peerConn{Conn: server, peer: &net.TCPAddr{IP: net.ParseIP(peer), Port: 4000}})
	}()
	defer client.Close()
	req := "GET / HTTP/1.1\r\nHost: moyu\r\nConnection: close\r\n"
	if header != "" {
		req += "CF-Connecting-IP: " + header + "\r\n"
	}
	if _, err := client.Write([]byte(req + "\r\n")); err != nil {
		t.Fatal(err)
	}
	res, err := http.ReadResponse(bufio.NewReader(client), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return string(body)
}

func TestClientIPComesFromCloudflareThroughAPrivateProxy(t *testing.T) {
	for _, tc := range []struct{ peer, header, want string }{
		{"10.0.1.7", "203.0.113.9", "203.0.113.9"},
		{"10.0.1.7", "2001:db8::1", "2001:db8::1"},
		{"10.0.1.7", "", "10.0.1.7"},
		{"10.0.1.7", "not-an-ip", "10.0.1.7"},
		{"198.51.100.4", "203.0.113.9", "198.51.100.4"},
	} {
		if got := clientIP(t, tc.peer, tc.header); got != tc.want {
			t.Errorf("peer %s, CF-Connecting-IP %q: c.IP() = %q, want %q", tc.peer, tc.header, got, tc.want)
		}
	}
}
