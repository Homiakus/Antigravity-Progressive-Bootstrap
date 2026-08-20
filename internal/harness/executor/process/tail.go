package process

type ringTail struct {
	buf  []byte
	pos  int
	full bool
}

func newRingTail(size int) *ringTail {
	if size < 0 {
		size = 0
	}
	return &ringTail{buf: make([]byte, size)}
}

func (r *ringTail) Write(p []byte) {
	if len(r.buf) == 0 || len(p) == 0 {
		return
	}
	if len(p) >= len(r.buf) {
		copy(r.buf, p[len(p)-len(r.buf):])
		r.pos = 0
		r.full = true
		return
	}
	first := copy(r.buf[r.pos:], p)
	if first < len(p) {
		copy(r.buf, p[first:])
	}
	r.pos = (r.pos + len(p)) % len(r.buf)
	if !r.full && r.pos == 0 {
		r.full = true
	}
}

func (r *ringTail) Bytes() []byte {
	if len(r.buf) == 0 {
		return nil
	}
	if !r.full {
		return append([]byte(nil), r.buf[:r.pos]...)
	}
	out := make([]byte, 0, len(r.buf))
	out = append(out, r.buf[r.pos:]...)
	out = append(out, r.buf[:r.pos]...)
	return out
}
