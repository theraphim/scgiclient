/*
Copyright 2013 Mathieu Lonjaret.
*/

// Package scgiclient implements the client side of the
// Simple Common Gateway Interface protocol, as described
// at http://python.ca/scgi/protocol.txt
package scgiclient

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type SCGITransport struct{}

func dialurl(u *url.URL) (net.Conn, error) {
	if u.Host == "" {
		fi, err := os.Stat(u.Path)
		if err == nil && fi.Mode()&os.ModeSocket != 0 {
			return net.Dial("unix", u.Path)
		}
		return nil, fmt.Errorf("%+v", u)
	}
	return net.Dial("tcp", u.Host)
}

func (s SCGITransport) RoundTrip(r *http.Request) (*http.Response, error) {
	conn, err := dialurl(r.URL)
	if err != nil {
		return nil, err
	}
	encoded, err := Encode(r)
	if err != nil {
		conn.Close()
		return nil, err
	}

	if _, err := conn.Write(encoded); err != nil {
		conn.Close()
		return nil, err
	}
	return Receive(conn, r)
}

func Encode(r *http.Request) ([]byte, error) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	return append(netstring(defaultHeader(len(body))), body...), nil
}

type combireader struct {
	*bufio.Reader
	conn net.Conn
}

func (cr combireader) Close() error {
	return cr.conn.Close()
}

func Receive(conn net.Conn, req *http.Request) (*http.Response, error) {
	r := bufio.NewReader(conn)
	resp := &http.Response{
		Request: req,
	}
	tp := textproto.NewReader(r)
	mimeHeader, err := tp.ReadMIMEHeader()
	if err != nil {
		conn.Close()
		return nil, err
	}
	status, ok := mimeHeader["Status"]
	if !ok {
		conn.Close()
		return nil, fmt.Errorf("ffuuu")
	}
	resp.Status = status[0]
	f := strings.SplitN(resp.Status, " ", 2)
	resp.StatusCode, err = strconv.Atoi(f[0])
	if err != nil {
		conn.Close()
		return nil, err
	}
	resp.Header = http.Header(mimeHeader)
	resp.Body = combireader{
		Reader: r,
		conn:   conn,
	}
	return resp, nil
}

func defaultHeader(bodyLen int) []byte {
	dh := append([]byte{}, header("CONTENT_LENGTH", strconv.Itoa(bodyLen))...)
	for _, kv := range defaultHeaderFields {
		dh = append(dh, header(kv.key, kv.value)...)
	}
	return dh
}

func header(name, value string) []byte {
	h := append([]byte(name), 0)
	h = append(h, []byte(value)...)
	return append(h, 0)
}

type headerField struct {
	key   string
	value string
}

// not using a map, because header fields need to be in order
var defaultHeaderFields = []headerField{
	{key: "SCGI", value: "1"},
	{key: "REQUEST_METHOD", value: "POST"},
	{key: "SERVER_PROTOCOL", value: "HTTP/1.1"},
}

const (
	comma = byte(',')
	colon = byte(':')
)

func netstring(s []byte) []byte {
	le := []byte(strconv.Itoa(len(s)))
	ns := append(le, colon)
	ns = append(ns, s...)
	ns = append(ns, comma)
	return ns
}
