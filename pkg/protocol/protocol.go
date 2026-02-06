package protocol

import (
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

type MessageType byte

const (
	MsgAuthReq  MessageType = iota // {Instance, Password, Mode (Send/Receive)}
	MsgAuthResp                    // {Success, Message}
	MsgFileList                    // []FileInfo
	MsgFileReq                     // Path
	MsgFileData                    // {Path, Data} (Chunk) - Wait, sending Path every chunk is wasteful.
	// Better: StartFile(Path, Size, Mode), Data(Chunk), EndFile
	MsgStartFile
	MsgData
	MsgEndFile
	MsgDeleteFile // Path
	MsgError
	MsgDone // Sync complete
)

const (
	// MaxMessageSize limits the maximum size of a single message payload (10MB).
	// This prevents OOM attacks where a malicious client sends a huge length header.
	MaxMessageSize = 10 * 1024 * 1024
)

type AuthRequest struct {
	Instance string
	Password string
	IsSender bool // If true, Client wants to SEND files to Server (Server is Receiver).
	// If false, Client wants to RECEIVE files from Server (Server is Sender).
	Compress bool
}

type FileListRequest struct {
	Checksum bool `json:"checksum"`
}

type AuthResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Exclude string `json:"exclude,omitempty"`
}

type FileInfo struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mod_time"`
	Mode    uint32 `json:"mode"`
	IsDir   bool   `json:"is_dir"`
	Hash    string `json:"hash,omitempty"`
}

type StartFileMsg struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Mode    uint32 `json:"mode"`
	ModTime int64  `json:"mod_time"`
}

// Transport helper
type Transport struct {
	conn io.ReadWriteCloser
	r    io.Reader
	w    io.Writer
	zw   *zlib.Writer
	zr   io.ReadCloser
}

func NewTransport(conn io.ReadWriteCloser) *Transport {
	return &Transport{
		conn: conn,
		r:    conn,
		w:    conn,
	}
}

func (t *Transport) EnableCompression() error {
	t.zw = zlib.NewWriter(t.conn)
	t.w = t.zw
	// Lazy initialize zlib reader when first read is needed to avoid deadlock
	// because zlib.NewReader tries to read header immediately.
	return nil
}

func (t *Transport) SendJSON(msgType MessageType, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return t.Send(msgType, data)
}

func (t *Transport) Send(msgType MessageType, data []byte) error {
	length := uint32(len(data))
	header := make([]byte, 5)
	header[0] = byte(msgType)
	binary.BigEndian.PutUint32(header[1:], length)

	if _, err := t.w.Write(header); err != nil {
		return err
	}
	if length > 0 {
		if _, err := t.w.Write(data); err != nil {
			return err
		}
	}
	if t.zw != nil {
		return t.zw.Flush()
	}
	return nil
}

func (t *Transport) ReadHeader() (MessageType, uint32, error) {
	if t.zw != nil && t.zr == nil {
		var err error
		t.zr, err = zlib.NewReader(t.conn)
		if err != nil {
			return 0, 0, err
		}
		t.r = t.zr
	}

	header := make([]byte, 5)
	if _, err := io.ReadFull(t.r, header); err != nil {
		return 0, 0, err
	}
	msgType := MessageType(header[0])
	length := binary.BigEndian.Uint32(header[1:])

	if length > MaxMessageSize {
		return 0, 0, fmt.Errorf("message too large: %d > %d", length, MaxMessageSize)
	}

	return msgType, length, nil
}

func (t *Transport) ReadJSON(v interface{}) (MessageType, error) {
	msgType, length, err := t.ReadHeader()
	if err != nil {
		return 0, err
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(t.r, data); err != nil {
		return msgType, err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return msgType, err
	}
	return msgType, nil
}

func (t *Transport) ReadData() (MessageType, []byte, error) {
	msgType, length, err := t.ReadHeader()
	if err != nil {
		return 0, nil, err
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(t.r, data); err != nil {
		return msgType, nil, err
	}
	return msgType, data, nil
}

func (t *Transport) Close() error {
	if t.zw != nil {
		t.zw.Close()
	}
	if t.zr != nil {
		t.zr.Close()
	}
	return t.conn.Close()
}

func (t *Transport) GetConn() io.Reader {
	return t.r
}
