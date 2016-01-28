package cgotbx

import "io"

type pipe struct {
	ch     chan []byte
	b      []byte
	chsize int
}

func newPipe(b []byte, chsize int) *pipe {
	ch := make(chan []byte, chsize)
	return &pipe{ch, b, chsize}
}

func (pi *pipe) Write(p []byte) (int, error) {
	pi.ch <- p
	return len(p), nil
}

func (pi *pipe) Close() error {
	return nil
}

func (pi *pipe) Read(p []byte) (int, error) {
	if len(pi.b) > 0 {
		n := copy(p, pi.b)
		//log.Println("begin", n, len(p), len(pi.b))
		// move data to start of buffer
		if n < len(pi.b) {
			copy(pi.b, pi.b[n:])
			pi.b = pi.b[:len(pi.b)-n]
			//log.Println("copying", len(pi.b))
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

	// we have this much left over to put into pi.buf
	pi.b = append(pi.b, r[n:]...)
	return n, nil
}
