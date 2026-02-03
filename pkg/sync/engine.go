package sync

import (
	"github.com/taurusxin/fastsync/pkg/protocol"
)

type Options struct {
	Delete    bool
	Overwrite bool
	Checksum  bool
	Compress  bool
	Archive   bool
}

type ActionType int

const (
	ActionCopy ActionType = iota
	ActionDelete
	ActionSkip
)

type FileAction struct {
	Path   string
	Type   ActionType
	Reason string
	Info   protocol.FileInfo // Source info for copy, Target info for delete
}

func Compare(source, target []protocol.FileInfo, opts Options) []FileAction {
	targetMap := make(map[string]protocol.FileInfo)
	for _, f := range target {
		targetMap[f.Path] = f
	}

	var actions []FileAction

	// Check Source -> Target
	for _, src := range source {
		tgt, exists := targetMap[src.Path]

		if !exists {
			actions = append(actions, FileAction{Path: src.Path, Type: ActionCopy, Reason: "new", Info: src})
			continue
		}

		if src.IsDir {
			continue
		}

		// File exists
		if opts.Overwrite {
			actions = append(actions, FileAction{Path: src.Path, Type: ActionCopy, Reason: "overwrite", Info: src})
			continue
		}

		if opts.Checksum {
			// If hashes are available and different
			if src.Hash != "" && tgt.Hash != "" && src.Hash != tgt.Hash {
				actions = append(actions, FileAction{Path: src.Path, Type: ActionCopy, Reason: "checksum_diff", Info: src})
				continue
			} else if src.Hash == "" || tgt.Hash == "" {
				// Fallback if hash missing? Or skip?
				// If we requested checksum but failed to calculate, maybe skip or warn.
			}
		}

		// "Otherwise ignore" -> Skip
	}

	// Check Target -> Source (for Delete)
	if opts.Delete {
		sourceMap := make(map[string]bool)
		for _, f := range source {
			sourceMap[f.Path] = true
		}

		for _, tgt := range target {
			if !sourceMap[tgt.Path] {
				actions = append(actions, FileAction{Path: tgt.Path, Type: ActionDelete, Reason: "extraneous", Info: tgt})
			}
		}
	}

	return actions
}
