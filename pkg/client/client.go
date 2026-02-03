package client

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/taurusxin/fastsync/pkg/logger"

	"github.com/taurusxin/fastsync/pkg/protocol"
	pkgSync "github.com/taurusxin/fastsync/pkg/sync"
	"github.com/taurusxin/fastsync/pkg/utils"
)

type Options struct {
	Delete    bool
	Overwrite bool
	Checksum  bool
	Compress  bool
	Archive   bool
	Threads   int
	Verbose   bool
}

type RemoteInfo struct {
	Password string
	Host     string
	Port     int
	Instance string
}

// Format: [password@]host:port/instance or host:port/instance
// Default port 7900 if not specified (but regex makes it tricky)
// Simplified: password@ip:port/instance
// If password missing: ip:port/instance
var remoteRegex = regexp.MustCompile(`^(([^@]+)@)?([^:/]+)(:(\d+))?/([^/]+)$`)

func parseRemote(s string) *RemoteInfo {
	// Heuristic: Must contain '@' or ':\d+' to be considered remote.
	// Otherwise treat as local path.
	if !strings.Contains(s, "@") && !regexp.MustCompile(`:\d+`).MatchString(s) {
		return nil
	}

	matches := remoteRegex.FindStringSubmatch(s)
	if matches == nil {
		return nil
	}
	// matches[0] full
	// matches[2] password (if exists)
	// matches[3] host
	// matches[5] port (if exists)
	// matches[6] instance

	info := &RemoteInfo{
		Password: matches[2],
		Host:     matches[3],
		Instance: matches[6],
		Port:     7963,
	}
	if matches[5] != "" {
		p, _ := strconv.Atoi(matches[5])
		info.Port = p
	}
	return info
}

func Run(source, target string, opts Options) {
	srcRemote := parseRemote(source)
	tgtRemote := parseRemote(target)

	if srcRemote != nil && tgtRemote != nil {
		logger.Error("Source and Target cannot both be remote")
		os.Exit(1)
	}

	start := time.Now()

	if srcRemote == nil && tgtRemote == nil {
		syncLocalLocal(source, target, opts)
	} else if srcRemote != nil {
		syncRemoteLocal(srcRemote, target, opts)
	} else {
		syncLocalRemote(source, tgtRemote, opts)
	}

	logger.Info("Sync completed in %v", time.Since(start))
}

func syncLocalLocal(source, target string, opts Options) {
	logger.Info("Syncing Local %s -> Local %s", source, target)

	// Scan Source
	srcFiles, err := pkgSync.Scan(source, nil, opts.Checksum)
	if err != nil {
		logger.Error("Failed to scan source: %v", err)
		return
	}

	// Scan Target
	tgtFiles, err := pkgSync.Scan(target, nil, opts.Checksum)
	if err != nil {
		// If target doesn't exist, it's empty
		if !os.IsNotExist(err) {
			logger.Warn("Failed to scan target (assuming empty): %v", err)
		}
		tgtFiles = []protocol.FileInfo{}
	}

	actions := pkgSync.Compare(srcFiles, tgtFiles, pkgSync.Options{
		Delete:    opts.Delete,
		Overwrite: opts.Overwrite,
		Checksum:  opts.Checksum,
	})

	logger.Info("Found %d actions", len(actions))

	// Check if source is a file
	srcInfo, _ := os.Stat(source)
	isSourceFile := srcInfo != nil && !srcInfo.IsDir()

	executeActions(actions, opts.Threads, func(a pkgSync.FileAction) error {
		var srcPath string
		var err error
		if isSourceFile {
			srcPath = source
		} else {
			srcPath, err = utils.SecureJoin(source, a.Path)
			if err != nil {
				return err
			}
		}
		tgtPath, _ := utils.SecureJoin(target, a.Path)

		switch a.Type {
		case pkgSync.ActionCopy:
			if a.Info.IsDir {
				logger.Info("Creating directory %s", a.Path)
				return os.MkdirAll(tgtPath, 0755)
			}
			logger.Info("Copying %s", a.Path)
			if err := copyFile(srcPath, tgtPath, a.Info.Mode); err != nil {
				return err
			}
			if opts.Archive {
				if a.Info.ModTime > 0 {
					os.Chtimes(tgtPath, time.Unix(a.Info.ModTime, 0), time.Unix(a.Info.ModTime, 0))
				}
				// Mode is already set by copyFile but maybe strict chmod is needed?
				os.Chmod(tgtPath, os.FileMode(a.Info.Mode))
			}
			return nil
		case pkgSync.ActionDelete:
			logger.Info("Deleting %s", a.Path)
			return os.RemoveAll(tgtPath) // RemoveAll for dirs
		}
		return nil
	})
}

func copyFile(src, dst string, mode uint32) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	os.MkdirAll(filepath.Dir(dst), 0755)
	d, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(mode))
	if err != nil {
		return err
	}
	defer d.Close()

	_, err = io.Copy(d, s)
	return err
}

func connectAndAuth(info *RemoteInfo, isSender bool, opts Options) (*protocol.Transport, string, error) {
	addr := net.JoinHostPort(info.Host, strconv.Itoa(info.Port))
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, "", err
	}
	t := protocol.NewTransport(conn)

	// Auth
	req := protocol.AuthRequest{
		Instance: info.Instance,
		Password: info.Password,
		IsSender: isSender,
		Compress: opts.Compress,
	}
	if err := t.SendJSON(protocol.MsgAuthReq, req); err != nil {
		t.Close()
		return nil, "", err
	}

	var resp protocol.AuthResponse
	if _, err := t.ReadJSON(&resp); err != nil {
		t.Close()
		return nil, "", err
	}
	if !resp.Success {
		t.Close()
		return nil, "", fmt.Errorf("auth failed: %s", resp.Message)
	}

	if opts.Compress {
		if err := t.EnableCompression(); err != nil {
			t.Close()
			return nil, "", err
		}
	}

	return t, resp.Exclude, nil
}

func syncRemoteLocal(srcInfo *RemoteInfo, target string, opts Options) {
	logger.Info("Syncing Remote %s -> Local %s", srcInfo.Host, target)

	// 1. Connect Main
	t, remoteExcludes, err := connectAndAuth(srcInfo, false, opts) // Client is Receiver (Sender=false)
	if err != nil {
		logger.Error("Connection failed: %v", err)
		return
	}
	defer t.Close() // Main connection

	// 2. Request File List
	req := protocol.FileListRequest{
		Checksum: opts.Checksum,
	}
	if err = t.SendJSON(protocol.MsgFileList, req); err != nil {
		logger.Error("Failed to request file list: %v", err)
		return
	}

	var srcFiles []protocol.FileInfo
	if _, err = t.ReadJSON(&srcFiles); err != nil {
		logger.Error("Failed to read file list: %v", err)
		return
	}

	// 3. Scan Local Target
	excludes := []string{}
	if remoteExcludes != "" {
		excludes = strings.Split(remoteExcludes, ",")
	}
	tgtFiles, scanErr := pkgSync.Scan(target, excludes, opts.Checksum)
	if scanErr != nil {
		tgtFiles = []protocol.FileInfo{}
	}

	// 4. Compare
	actions := pkgSync.Compare(srcFiles, tgtFiles, pkgSync.Options{
		Delete:    opts.Delete,
		Overwrite: opts.Overwrite,
		Checksum:  opts.Checksum,
	})
	logger.Info("Found %d actions", len(actions))

	// 5. Execute
	// If threads > 1, main connection cannot be shared easily.
	// We will spawn workers that create their OWN connections.

	executeActions(actions, opts.Threads, func(a pkgSync.FileAction) error {
		tgtPath, _ := utils.SecureJoin(target, a.Path)

		switch a.Type {
		case pkgSync.ActionDelete:
			logger.Info("Deleting %s", a.Path)
			return os.Remove(tgtPath)

		case pkgSync.ActionCopy:
			logger.Info("Pulling %s", a.Path)
			// Worker connection
			wt, _, err := connectAndAuth(srcInfo, false, opts)
			if err != nil {
				return err
			}
			defer wt.Close()

			// Request File
			if err = wt.Send(protocol.MsgFileReq, []byte(a.Path)); err != nil {
				return err
			}

			// Receive Start
			var startMsg protocol.StartFileMsg
			var mt protocol.MessageType
			mt, err = wt.ReadJSON(&startMsg)
			if err != nil {
				return err
			}
			if mt == protocol.MsgError {
				return fmt.Errorf("remote error")
			}

			// Receive Data
			os.MkdirAll(filepath.Dir(tgtPath), 0755)

			if os.FileMode(startMsg.Mode).IsDir() {
				os.MkdirAll(tgtPath, 0755)
				// Consume empty stream
				for {
					mt, _, err = wt.ReadData()
					if err != nil || mt == protocol.MsgEndFile {
						break
					}
				}
				return nil
			}

			f, err := os.OpenFile(tgtPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(startMsg.Mode))
			if err != nil {
				return err
			}
			defer f.Close()

			for {
				var data []byte
				mt, data, err = wt.ReadData()
				if err != nil {
					return err
				}
				if mt == protocol.MsgEndFile {
					break
				}
				if mt == protocol.MsgData {
					f.Write(data)
				}
			}
			f.Close()

			// Restore attributes if Archive mode is enabled
			if opts.Archive {
				if startMsg.ModTime > 0 {
					os.Chtimes(tgtPath, time.Unix(startMsg.ModTime, 0), time.Unix(startMsg.ModTime, 0))
				}
				if startMsg.Mode > 0 {
					os.Chmod(tgtPath, os.FileMode(startMsg.Mode))
				}
			}

			return nil
		}
		return nil
	})

	// Send Done on main? Not strictly needed as we close.
	t.Send(protocol.MsgDone, nil)
}

func syncLocalRemote(source string, tgtInfo *RemoteInfo, opts Options) {
	logger.Info("Syncing Local %s -> Remote %s", source, tgtInfo.Host)

	t, remoteExcludes, err := connectAndAuth(tgtInfo, true, opts) // Client is Sender
	if err != nil {
		logger.Error("Connection failed: %v", err)
		return
	}
	defer t.Close()

	// Request Remote File List
	if err = t.Send(protocol.MsgFileList, nil); err != nil {
		logger.Error("Failed to request file list: %v", err)
		return
	}
	var tgtFiles []protocol.FileInfo
	if _, err = t.ReadJSON(&tgtFiles); err != nil {
		logger.Error("Failed to read file list: %v", err)
		return
	}

	// Scan Local
	excludes := []string{}
	if remoteExcludes != "" {
		excludes = strings.Split(remoteExcludes, ",")
	}
	srcFiles, scanErr := pkgSync.Scan(source, excludes, opts.Checksum)
	if scanErr != nil {
		logger.Error("Failed to scan local: %v", scanErr)
		return
	}

	actions := pkgSync.Compare(srcFiles, tgtFiles, pkgSync.Options{
		Delete:    opts.Delete,
		Overwrite: opts.Overwrite,
		Checksum:  opts.Checksum,
	})
	logger.Info("Found %d actions", len(actions))

	// Check if source is a file
	srcInfo, _ := os.Stat(source)
	isSourceFile := srcInfo != nil && !srcInfo.IsDir()

	executeActions(actions, opts.Threads, func(a pkgSync.FileAction) error {
		var srcPath string
		var err error
		if isSourceFile {
			srcPath = source
		} else {
			srcPath, err = utils.SecureJoin(source, a.Path)
			if err != nil {
				return err
			}
		}

		switch a.Type {
		case pkgSync.ActionDelete:
			logger.Info("Remote Deleting %s", a.Path)
			wt, _, err := connectAndAuth(tgtInfo, true, opts)
			if err != nil {
				return err
			}
			defer wt.Close()
			wt.Send(protocol.MsgDeleteFile, []byte(a.Path))

		case pkgSync.ActionCopy:
			logger.Info("Pushing %s", a.Path)
			wt, _, err := connectAndAuth(tgtInfo, true, opts)
			if err != nil {
				return err
			}
			defer wt.Close()

			if a.Info.IsDir {
				wt.SendJSON(protocol.MsgStartFile, protocol.StartFileMsg{
					Path: a.Path,
					Size: 0,
					Mode: uint32(a.Info.Mode),
				})
				wt.Send(protocol.MsgEndFile, nil)
				return nil
			}

			f, openErr := os.Open(srcPath)
			if openErr != nil {
				return openErr
			}
			defer f.Close()

			info, _ := f.Stat()

			// Send Start
			wt.SendJSON(protocol.MsgStartFile, protocol.StartFileMsg{
				Path:    a.Path,
				Size:    info.Size(),
				Mode:    uint32(info.Mode()),
				ModTime: info.ModTime().Unix(),
			})

			// Send Data
			buf := make([]byte, 32*1024)
			for {
				n, readErr := f.Read(buf)
				if n > 0 {
					wt.Send(protocol.MsgData, buf[:n])
				}
				if readErr != nil {
					break
				}
			}
			wt.Send(protocol.MsgEndFile, nil)
		}
		return nil
	})

	t.Send(protocol.MsgDone, nil)
}

func executeActions(actions []pkgSync.FileAction, workerCount int, handler func(pkgSync.FileAction) error) {
	if workerCount <= 0 {
		workerCount = 1
	}
	// Cap worker count at number of actions
	if workerCount > len(actions) {
		workerCount = len(actions)
	}
	if workerCount == 0 {
		return
	}

	ch := make(chan pkgSync.FileAction, len(actions))
	for _, a := range actions {
		ch <- a
	}
	close(ch)

	var wg sync.WaitGroup
	wg.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go func() {
			defer wg.Done()
			for a := range ch {
				if err := handler(a); err != nil {
					logger.Error("Error processing %s: %v", a.Path, err)
				}
			}
		}()
	}
	wg.Wait()
}
