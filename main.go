package scgiclient

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"strconv"
	"strings"
)

func Send(addr string, r io.Reader) (*Response, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	req := NewRequest(addr, body)
	req.conn = conn
	if _, err = req.Send(); err != nil {
		return nil, err
	}
	resp, err := req.Receive()
	if err != nil {
		return nil, err
	}
	err = conn.Close()
	return resp, err
}

type Request struct {
	Addr string
	Header []byte
	Body   []byte
	conn net.Conn
}

func NewRequest(addr string, body []byte) *Request {
	return &Request{
		Addr: addr,
		Header: defaultHeader(len(body)),
		Body:   body,
	}
}

func (r *Request) Close() error {
	return r.conn.Close()
}

func (r *Request) Send() (int64, error) {
	var err error
	if r.conn == nil {
		r.conn, err = net.Dial("tcp", r.Addr)
		if err != nil {
			return 0, err
		}
	}
	msg := append(netstring(r.Header), r.Body...)
	return io.Copy(r.conn, bytes.NewReader(msg))
}

func (r *Request) Receive() (*Response, error) {
	if r.conn == nil {
		return nil, errors.New("Can not receive on a closed connection")
	}
	return receive(r.conn)
}

type ResponseHeader struct {
	Raw []byte
	Status string
}

type Response struct {
	Header *ResponseHeader
	Body []byte
	conn net.Conn
}

func (r *Response) Close() error {
	return r.conn.Close()
}

func receive(conn net.Conn) (*Response, error) {
	r := bufio.NewReader(conn)
	status, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	terminator := string([]byte{13, 10})
	status = strings.TrimRight(status, terminator)
	header := &ResponseHeader{
		Status: status,
	}
	resp := &Response{
		Header: header,
		conn: conn,
	}
	if status != "Status: 200 OK" {
		return resp, fmt.Errorf("Got %v as response status", status)
	}
	// TODO(mpl): other header fields should not be in the body
	var body bytes.Buffer
	if _, err = io.Copy(&body, r); err != nil {
		return nil, fmt.Errorf("Could not read response: %v", err)
	}
	resp.Body = body.Bytes()
	return resp, nil
}

// TODO(mpl): report hoisie his scgi server panics if field missing
func defaultHeader(bodyLen int) []byte {
	var dh []byte
	defaultHeaderFields["CONTENT_LENGTH"] = strconv.Itoa(bodyLen)
	for k, v := range defaultHeaderFields {
		dh = append(dh, header(k, v)...)
	}
	return dh
}

func header(name, value string) []byte {
	h := append([]byte(name), 0)
	h = append(h, []byte(value)...)
	return append(h, 0)
}

var defaultHeaderFields = map[string]string{
	"CONTENT_LENGTH":  "",
	"SCGI":            "1",
	"REQUEST_METHOD":  "POST",
	"SERVER_PROTOCOL": "HTTP/1.1",
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
