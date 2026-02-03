package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/taurusxin/fastsync/pkg/config"
	"github.com/taurusxin/fastsync/pkg/logger"
	"github.com/taurusxin/fastsync/pkg/protocol"
	pkgSync "github.com/taurusxin/fastsync/pkg/sync"
	"github.com/taurusxin/fastsync/pkg/utils"
)

func Run(cfg *config.Config) {
	addr := fmt.Sprintf("%s:%d", cfg.Address, cfg.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("Failed to bind %s: %v", addr, err)
		return
	}
	logger.Info("Listening on %s", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Error("Accept error: %v", err)
			continue
		}
		go handleConn(conn, cfg)
	}
}

func handleConn(conn net.Conn, cfg *config.Config) {
	defer conn.Close()
	remoteIP, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
	logger.Info("New connection from %s", remoteIP)

	transport := protocol.NewTransport(conn)

	// 1. Auth
	var authReq protocol.AuthRequest
	msgType, err := transport.ReadJSON(&authReq)
	if err != nil {
		logger.Error("Failed to read auth: %v", err)
		return
	}
	if msgType != protocol.MsgAuthReq {
		logger.Error("Unexpected message type: %v", msgType)
		return
	}

	// Validate Instance
	var instance *config.InstanceConfig
	found := false
	for i := range cfg.Instances {
		if cfg.Instances[i].Name == authReq.Instance {
			instance = &cfg.Instances[i]
			found = true
			break
		}
	}

	if !found {
		transport.SendJSON(protocol.MsgAuthResp, protocol.AuthResponse{Success: false, Message: "Instance not found"})
		return
	}

	// Initialize Instance Logger
	var logOut io.Writer = os.Stdout
	if instance.LogFile != "" && instance.LogFile != "stdout" {
		f, err := os.OpenFile(instance.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			logger.Error("Failed to open instance log file %s: %v", instance.LogFile, err)
			// Fallback to stdout
		} else {
			defer f.Close()
			logOut = f
		}
	}
	instLogger := logger.New(logOut, logger.ParseLevel(instance.LogLevel), instance.Name)

	if !utils.CheckAccess(remoteIP, instance.HostAllow, instance.HostDeny) {
		transport.SendJSON(protocol.MsgAuthResp, protocol.AuthResponse{Success: false, Message: "Access denied"})
		instLogger.Warn("Access denied for %s on instance %s", remoteIP, instance.Name)
		return
	}

	if instance.Password != "" && instance.Password != authReq.Password {
		transport.SendJSON(protocol.MsgAuthResp, protocol.AuthResponse{Success: false, Message: "Invalid password"})
		instLogger.Warn("Invalid password for %s on instance %s", remoteIP, instance.Name)
		return
	}

	transport.SendJSON(protocol.MsgAuthResp, protocol.AuthResponse{Success: true})
	instLogger.Info("Client %s connected", remoteIP)

	if authReq.Compress {
		if err := transport.EnableCompression(); err != nil {
			instLogger.Error("Failed to enable compression: %v", err)
			return
		}
	}

	handleSession(transport, instance, instLogger)
}

func handleSession(t *protocol.Transport, inst *config.InstanceConfig, log *logger.Logger) {
	buf := make([]byte, 32*1024)
	for {
		msgType, length, err := t.ReadHeader()
		if err != nil {
			if err != io.EOF {
				log.Error("Read error: %v", err)
			}
			return
		}

		switch msgType {
		case protocol.MsgFileList:
			// Discard payload if any (should be 0 length for request?)
			// Protocol design check: Does client send MsgFileList to request it?
			// Let's assume yes.
			if length > 0 {
				io.CopyN(io.Discard, t.GetConn(), int64(length))
			}

			// Calculate hashes? Expensive.
			// Let's assume we do it.
			files, err := pkgSync.Scan(inst.Path, strings.Split(inst.Exclude, ","), true)
			if err != nil {
				log.Error("Scan failed: %v", err)
				t.SendJSON(protocol.MsgError, protocol.AuthResponse{Message: err.Error()}) // Reuse struct? No, map[string]string?
				// Just send error type with simple string
				t.Send(protocol.MsgError, []byte(err.Error()))
				return
			}
			t.SendJSON(protocol.MsgFileList, files)

		case protocol.MsgFileReq:
			// Client wants file
			pathData := make([]byte, length)
			io.ReadFull(t.GetConn(), pathData)
			relPath := string(pathData)

			absPath, err := utils.SecureJoin(inst.Path, relPath)
			if err != nil {
				log.Error("Security error: %v", err)
				t.Send(protocol.MsgError, []byte("Invalid path"))
				continue
			}

			f, err := os.Open(absPath)
			if err != nil {
				log.Error("Open file error: %v", err)
				t.Send(protocol.MsgError, []byte(err.Error()))
				continue
			}

			info, _ := f.Stat()
			t.SendJSON(protocol.MsgStartFile, protocol.StartFileMsg{
				Path:    relPath,
				Size:    info.Size(),
				Mode:    uint32(info.Mode()),
				ModTime: info.ModTime().Unix(),
			})
			log.Info("Sending file: %s", relPath)

			if info.IsDir() {
				f.Close()
				t.Send(protocol.MsgEndFile, nil)
				continue
			}

			// Send chunks
			for {
				n, err := f.Read(buf)
				if n > 0 {
					t.Send(protocol.MsgData, buf[:n])
				}
				if err != nil {
					break
				}
			}
			f.Close()
			t.Send(protocol.MsgEndFile, nil)

		case protocol.MsgStartFile:
			// Client sending file
			data := make([]byte, length)
			io.ReadFull(t.GetConn(), data)
			var startMsg protocol.StartFileMsg
			json.Unmarshal(data, &startMsg)

			absPath, err := utils.SecureJoin(inst.Path, startMsg.Path)
			if err != nil {
				log.Error("Security error: %v", err)
				// Skip until EndFile
				discardFile(t)
				continue
			}

			// Ensure dir exists
			os.MkdirAll(filepath.Dir(absPath), 0755)

			if os.FileMode(startMsg.Mode).IsDir() {
				os.MkdirAll(absPath, 0755) // Ignore mode for now or use startMsg.Mode
				discardFile(t)
				continue
			}

			f, err := os.OpenFile(absPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(startMsg.Mode))
			if err != nil {
				log.Error("Create file error: %v", err)
				discardFile(t)
				continue
			}

			// Read Data until EndFile
			for {
				mt, l, err := t.ReadHeader()
				if err != nil {
					f.Close()
					return
				}
				if mt == protocol.MsgEndFile {
					break
				}
				if mt == protocol.MsgData {
					io.CopyN(f, t.GetConn(), int64(l))
				} else {
					// Unexpected
					f.Close()
					return
				}
			}
			f.Close()

			// Restore attributes
			if startMsg.ModTime > 0 {
				os.Chtimes(absPath, time.Unix(startMsg.ModTime, 0), time.Unix(startMsg.ModTime, 0))
			}
			if startMsg.Mode > 0 {
				os.Chmod(absPath, os.FileMode(startMsg.Mode))
			}

			log.Info("Received file: %s", startMsg.Path)

		case protocol.MsgDeleteFile:
			pathData := make([]byte, length)
			io.ReadFull(t.GetConn(), pathData)
			relPath := string(pathData)
			absPath, err := utils.SecureJoin(inst.Path, relPath)
			if err == nil {
				os.Remove(absPath) // Or RemoveAll?
				log.Info("Deleted %s", relPath)
			}

		case protocol.MsgDone:
			return
		}
	}
}

func discardFile(t *protocol.Transport) {
	for {
		mt, l, err := t.ReadHeader()
		if err != nil {
			return
		}
		if mt == protocol.MsgEndFile {
			return
		}
		io.CopyN(io.Discard, t.GetConn(), int64(l))
	}
}
