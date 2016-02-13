package frame

const (
	// offsets for packing/unpacking frames
	lengthOffset = 32 + 16
	flagsOffset  = 32 + 8
	typeOffset   = 32 + 3

	// masks for packing/unpacking frames
	lengthMask   = 0x3FFF
	streamMask   = 0x7FFFFFFF
	flagsMask    = 0xFF
	typeMask     = 0x1F
	wndIncMask   = 0x7FFFFFFF
	priorityMask = 0x7FFFFFFF
)

// a frameType is a 5-bit integer in the frame header that identifies the type of frame
type FrameType uint8

const (
	TypeStreamSyn    = 0x1
	TypeStreamRst    = 0x2
	TypeStreamData   = 0x3
	TypeStreamWndInc = 0x4
	TypeStreamPri    = 0x5
	TypeGoAway       = 0x6
)

// a flagsType is an 8-bit integer containing frame-specific flag bits in the frame header
type flagsType uint8

const (
	flagFin            = 0x1
	flagStreamPriority = 0x2
	flagStreamType     = 0x4
)

func (ft flagsType) IsSet(f flagsType) bool {
	return (ft & f) != 0
}

func (ft *flagsType) Set(f flagsType) {
	*ft |= f
}

func (ft *flagsType) Unset(f flagsType) {
	*ft = *ft &^ f
}

// StreamId is 31-bit integer uniquely identifying a stream within a session
type StreamId uint32

// StreamPriority is 31-bit integer specifying a stream's priority
type StreamPriority uint32

// StreamType is 32-bit integer specifying a stream's type
type StreamType uint32

// ErrorCode is a 32-bit integer indicating a error condition included in rst/goaway frames
type ErrorCode uint32
