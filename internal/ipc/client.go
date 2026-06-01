package ipc

import "net"

// Client is a one-shot Unix-socket client for the control protocol.
type Client struct{ sock string }

// NewClient returns a client for the daemon socket at path sock.
func NewClient(sock string) *Client { return &Client{sock: sock} }

// Do sends one request and returns the single response.
func (c *Client) Do(req Request) (Response, error) {
	conn, err := net.Dial("unix", c.sock)
	if err != nil {
		return Response{}, err
	}
	defer conn.Close()
	if err := NewEncoder(conn).Encode(req); err != nil {
		return Response{}, err
	}
	var resp Response
	if err := NewDecoder(conn).Decode(&resp); err != nil {
		return Response{}, err
	}
	return resp, nil
}

// Watch sends a streaming request and calls handler for each pushed response.
// It returns when handler returns a non-nil error or the connection closes.
func (c *Client) Watch(req Request, handler func(Response) error) error {
	conn, err := net.Dial("unix", c.sock)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := NewEncoder(conn).Encode(req); err != nil {
		return err
	}
	dec := NewDecoder(conn)
	for {
		var resp Response
		if err := dec.Decode(&resp); err != nil {
			return err
		}
		if err := handler(resp); err != nil {
			return err
		}
	}
}
