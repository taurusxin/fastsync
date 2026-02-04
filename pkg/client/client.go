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
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/taurusxin/fastsync/pkg/logger"

	"github.com/taurusxin/fastsync/pkg/protocol"
	pkgSync "github.com/taurusxin/fastsync/pkg/sync"
	"github.com/taurusxin/fastsync/pkg/utils"
)

func newProgressBar(max int64, description string) *progressbar.ProgressBar {
	return progressbar.NewOptions64(
		max,
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetWriter(os.Stdout),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(15),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionUseIECUnits(true),
	)
}

type Options struct {
	Delete    bool
	Overwrite bool
	Checksum  bool
	Compress  bool
	Archive   bool
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

	logger.Info("Sync completed in %.2fs", time.Since(start).Seconds())
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

	// Calculate total size for summary
	var totalSize int64
	for _, a := range actions {
		if a.Type == pkgSync.ActionCopy {
			totalSize += a.Info.Size
		}
	}

	// Check if source is a file
	srcInfo, _ := os.Stat(source)
	isSourceFile := srcInfo != nil && !srcInfo.IsDir()

	startTime := time.Now()

	for _, a := range actions {
		var srcPath string
		var err error
		if isSourceFile {
			srcPath = source
		} else {
			srcPath, err = utils.SecureJoin(source, a.Path)
			if err != nil {
				logger.Error("Error processing %s: %v", a.Path, err)
				continue
			}
		}
		tgtPath, _ := utils.SecureJoin(target, a.Path)

		switch a.Type {
		case pkgSync.ActionCopy:
			if a.Info.IsDir {
				logger.Info("Creating directory %s", a.Path)
				if err := os.MkdirAll(tgtPath, 0755); err != nil {
					logger.Error("Error creating directory %s: %v", a.Path, err)
				}
				continue
			}
			logger.Info("Copying %s", a.Path)
			bar := newProgressBar(
				a.Info.Size,
				fmt.Sprintf("Copying %s", a.Path),
			)
			if err := copyFile(srcPath, tgtPath, a.Info.Mode, bar); err != nil {
				logger.Error("Error copying %s: %v", a.Path, err)
				continue
			}
			if opts.Archive {
				if a.Info.ModTime > 0 {
					os.Chtimes(tgtPath, time.Unix(a.Info.ModTime, 0), time.Unix(a.Info.ModTime, 0))
				}
				// Mode is already set by copyFile but maybe strict chmod is needed?
				os.Chmod(tgtPath, os.FileMode(a.Info.Mode))
			}
		case pkgSync.ActionDelete:
			logger.Info("Deleting %s", a.Path)
			if err := os.RemoveAll(tgtPath); err != nil {
				logger.Error("Error deleting %s: %v", a.Path, err)
			}
		}
	}

	elapsed := time.Since(startTime)
	avgSpeed := float64(totalSize) / elapsed.Seconds()
	logger.Info("Total size: %s, Time elapsed: %.2fs, Average speed: %s/s", utils.FormatBytes(totalSize), elapsed.Seconds(), utils.FormatBytes(int64(avgSpeed)))
}

func copyFile(src, dst string, mode uint32, bar *progressbar.ProgressBar) error {
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

	var writer io.Writer = d
	if bar != nil {
		writer = io.MultiWriter(d, bar)
	}

	_, err = io.Copy(writer, s)
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

	// Calculate total size for summary
	var totalSize int64
	for _, a := range actions {
		if a.Type == pkgSync.ActionCopy {
			totalSize += a.Info.Size
		}
	}

	startTime := time.Now()

	// 5. Execute
	for _, a := range actions {
		tgtPath, _ := utils.SecureJoin(target, a.Path)

		switch a.Type {
		case pkgSync.ActionDelete:
			logger.Info("Deleting %s", a.Path)
			if err = os.Remove(tgtPath); err != nil {
				logger.Error("Error deleting %s: %v", a.Path, err)
			}

		case pkgSync.ActionCopy:
			logger.Info("Pulling %s", a.Path)

			// Request File
			if err = t.Send(protocol.MsgFileReq, []byte(a.Path)); err != nil {
				logger.Error("Error requesting file %s: %v", a.Path, err)
				continue
			}

			// Receive Start
			var startMsg protocol.StartFileMsg
			var mt protocol.MessageType
			mt, err = t.ReadJSON(&startMsg)
			if err != nil {
				logger.Error("Error reading start msg for %s: %v", a.Path, err)
				continue
			}
			if mt == protocol.MsgError {
				logger.Error("Remote error for %s", a.Path)
				continue
			}

			// Ensure dir exists
			os.MkdirAll(filepath.Dir(tgtPath), 0755)

			if os.FileMode(startMsg.Mode).IsDir() {
				os.MkdirAll(tgtPath, 0755)
				// Read EndFile
				mt, _, err = t.ReadHeader()
				if err != nil {
					logger.Error("Error reading end file for dir %s: %v", a.Path, err)
					continue
				}
				if mt != protocol.MsgEndFile {
					logger.Error("Expected EndFile for dir %s", a.Path)
				}
				continue
			}

			f, err := os.OpenFile(tgtPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(startMsg.Mode))
			if err != nil {
				logger.Error("Error opening file %s: %v", tgtPath, err)
				continue
			}

			bar := newProgressBar(
				startMsg.Size,
				fmt.Sprintf("Pulling %s", a.Path),
			)

			// Read Data
			var data []byte
			for {
				mt, data, err = t.ReadData()
				if err != nil {
					f.Close()
					logger.Error("Error reading data for %s: %v", a.Path, err)
					break
				}
				if mt == protocol.MsgEndFile {
					break
				}
				if mt == protocol.MsgData {
					n, _ := f.Write(data)
					bar.Add(n)
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
		}
	}

	t.Send(protocol.MsgDone, nil)

	elapsed := time.Since(startTime)
	avgSpeed := float64(totalSize) / elapsed.Seconds()
	logger.Info("Total size: %s, Time elapsed: %.2fs, Average speed: %s/s", utils.FormatBytes(totalSize), elapsed.Seconds(), utils.FormatBytes(int64(avgSpeed)))
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

	// Calculate total size for summary
	var totalSize int64
	for _, a := range actions {
		if a.Type == pkgSync.ActionCopy {
			totalSize += a.Info.Size
		}
	}

	startTime := time.Now()

	// Check if source is a file
	srcInfo, _ := os.Stat(source)
	isSourceFile := srcInfo != nil && !srcInfo.IsDir()

	for _, a := range actions {
		var srcPath string
		var err error
		if isSourceFile {
			srcPath = source
		} else {
			srcPath, err = utils.SecureJoin(source, a.Path)
			if err != nil {
				logger.Error("Error secure join %s: %v", a.Path, err)
				continue
			}
		}

		switch a.Type {
		case pkgSync.ActionDelete:
			logger.Info("Remote Deleting %s", a.Path)
			t.Send(protocol.MsgDeleteFile, []byte(a.Path))

		case pkgSync.ActionCopy:
			logger.Info("Pushing %s", a.Path)

			if a.Info.IsDir {
				t.SendJSON(protocol.MsgStartFile, protocol.StartFileMsg{
					Path: a.Path,
					Size: 0,
					Mode: uint32(a.Info.Mode),
				})
				t.Send(protocol.MsgEndFile, nil)
				continue
			}

			f, openErr := os.Open(srcPath)
			if openErr != nil {
				logger.Error("Error opening %s: %v", srcPath, openErr)
				continue
			}

			info, _ := f.Stat()

			// Send Start
			t.SendJSON(protocol.MsgStartFile, protocol.StartFileMsg{
				Path:    a.Path,
				Size:    info.Size(),
				Mode:    uint32(info.Mode()),
				ModTime: info.ModTime().Unix(),
			})

			bar := newProgressBar(
				info.Size(),
				fmt.Sprintf("Pushing %s", a.Path),
			)

			// Send Data
			buf := make([]byte, 32*1024)
			for {
				n, readErr := f.Read(buf)
				if n > 0 {
					t.Send(protocol.MsgData, buf[:n])
					bar.Add(n)
				}
				if readErr != nil {
					break
				}
			}
			t.Send(protocol.MsgEndFile, nil)
			f.Close()
		}
	}

	t.Send(protocol.MsgDone, nil)

	elapsed := time.Since(startTime)
	avgSpeed := float64(totalSize) / elapsed.Seconds()
	logger.Info("Total size: %s, Time elapsed: %.2fs, Average speed: %s/s", utils.FormatBytes(totalSize), elapsed.Seconds(), utils.FormatBytes(int64(avgSpeed)))
}
