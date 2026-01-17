package ccguard

// RingBuffer 环形缓冲区，用于存储最近的输出内容
type RingBuffer struct {
	data []byte
	size int
	pos  int
	full bool
}

// NewRingBuffer 创建指定大小的环形缓冲区
func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		data: make([]byte, size),
		size: size,
	}
}

// Write 写入数据到缓冲区
func (r *RingBuffer) Write(p []byte) (n int, err error) {
	for _, b := range p {
		r.data[r.pos] = b
		r.pos = (r.pos + 1) % r.size
		if r.pos == 0 {
			r.full = true
		}
	}
	return len(p), nil
}

// String 返回缓冲区中的所有内容
func (r *RingBuffer) String() string {
	if !r.full {
		return string(r.data[:r.pos])
	}
	return string(r.data[r.pos:]) + string(r.data[:r.pos])
}

// Len 返回缓冲区中有效数据的长度
func (r *RingBuffer) Len() int {
	if !r.full {
		return r.pos
	}
	return r.size
}

// Reset 重置缓冲区
func (r *RingBuffer) Reset() {
	r.pos = 0
	r.full = false
}
