package cgotbx

import "io"

type pipe struct {
	ch     chan []byte
	b      []byte
	chsize int
}

func newPipe(chsize int) *pipe {
	ch := make(chan []byte, chsize)
	return &pipe{ch: ch, chsize: chsize}
}

func (pi *pipe) Write(p []byte) (int, error) {
	pi.ch <- p
	return len(p), nil
}

func (pi *pipe) Close() error {
	return nil
}

func (pi *pipe) Read(p []byte) (int, error) {
	if pi.b != nil && len(pi.b) > 0 {
		n := copy(p, pi.b)
		// move data to start of buffer
		if n < len(pi.b) {
			copy(pi.b, pi.b[n:])
			pi.b = pi.b[:len(pi.b)-n]
		} else {
			pi.b = pi.b[:0]
		}
		return n, nil
	}
	r, ok := <-pi.ch
	if !ok {
		return 0, io.EOF
	}
	n := copy(p, r)
	if len(p) >= len(r) {
		return n, nil
	}

	if pi.b == nil {
		pi.b = make([]byte, 0, len(r)+10)
	}
	// we have this much left over to put into pi.buf
	pi.b = append(pi.b, r[n:]...)
	return n, nil
}
