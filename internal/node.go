package internal

import (
	"encoding/json"
	"fmt"
	"io"

	//nolint: staticcheck
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/alswl/go-tree/constant"
)

// Node ..
type node struct {
	root  *node
	nodes []node
	depth int
	last  bool
	path  string
	total map[string]int
	info  os.FileInfo
}

type jnode struct {
	Files []string          `json:"f,omitempty"`
	Dirs  map[string]*jnode `json:",omitempty"`
}

func (j *jnode) MarshalJSON() ([]byte, error) {
	if len(j.Dirs) == 0 {
		return json.Marshal(j.Files)
	}
	m := map[string]interface{}{}
	if len(j.Files) > 0 {
		m["f"] = j.Files
	}
	for k, v := range j.Dirs {
		m[k] = v
	}
	return json.Marshal(m)
}

func (n *node) buildTree(flags map[string]interface{}) error {
	entries, err := ioutil.ReadDir(n.path)
	if err != nil {
		return err
	}

	hasAll := *(flags[constant.All].(*bool))
	hasWin := *(flags[constant.Win].(*bool))
	entries = exceptHiddens(hasAll, hasWin, n.path, entries)

	if subStr := *(flags[constant.Find].(*string)); subStr != "" {
		entries = search(entries, subStr)
	}

	dirs := justDirs(entries)
	n.total[dir] = n.total[dir] + len(dirs)
	n.total[file] = n.total[file] + len(entries) - len(dirs)

	if justDir := *(flags[constant.Justdir].(*bool)); justDir {
		entries = dirs
	}

	for i, e := range entries {
		last := false
		if i+1 == len(entries) {
			last = true
		}
		_n := node{n, nil, n.depth + 1, last, fmt.Sprintf("%s%s", appendSeperator(n.path), e.Name()), n.total, e}
		if maxDepth := *(flags[constant.Level].(*int)); e.Mode().IsDir() {
			if !(maxDepth != 0 && n.depth+1 >= maxDepth) {
				if err = _n.buildTree(flags); err != nil {
					return err
				}
			}
		}

		if hasTrim := *(flags[constant.Trim].(*bool)); !hasTrim || (!_n.info.IsDir() || len(_n.nodes) > 0) {
			n.nodes = append(n.nodes, _n)
		} else if last && len(n.nodes) > 0 {
			n.nodes[len(n.nodes)-1].last = true
		}
	}
	return nil
}

func (n node) draw(wr io.Writer, flags map[string]interface{}) {
	n.print(wr, flags)
	for _, _n := range n.nodes {
		if _n.nodes != nil {
			_n.draw(wr, flags)
		} else {
			_n.print(wr, flags)
		}
	}
}

func (n node) print(wr io.Writer, flags map[string]interface{}) {
	line := ""
	hasColor := *(flags[constant.Color].(*bool))
	outputFile := *(flags[constant.Output].(*string))
	if n.root != nil {
		for _n := n.root; _n.root != nil; _n = _n.root {
			prefix := fmt.Sprintf("%s%s", "│", strings.Repeat(" ", 3))
			if _n.last {
				prefix = strings.Repeat(" ", 4)
			}
			if hasColor && outputFile == "" {
				prefix = colorize(lineColors[_n.depth%len(lineColors)], prefix)
			}
			line = fmt.Sprintf("%s%s", prefix, line)
		}

		suffix := "├── "
		if n.last {
			suffix = "└── "
		}

		if hasEmoji := *(flags[constant.Emoji].(*bool)); hasEmoji && outputFile == "" {
			e := fileEmoji
			if n.info.IsDir() {
				e = dirEmoji
			}
			suffix = fmt.Sprintf("%s%s%s", suffix, e, strings.Repeat(" ", 2))
		}

		if hasColor && outputFile == "" {
			suffix = colorize(lineColors[n.depth%len(lineColors)], suffix)
		}
		line = fmt.Sprintf("%s%s", line, suffix)
	}

	name := filepath.Base(n.path)
	if hasPath := *(flags[constant.Path].(*bool)); hasPath || n.root == nil {
		name = n.path
	}

	nameColor := nameColors[file]
	if n.info.IsDir() {
		name = appendSeperator(name)
		nameColor = nameColors[dir]
	}

	meta := ""
	if hasMode := *(flags[constant.Mode].(*bool)); hasMode {
		meta = fmt.Sprintf("%v  %v", meta, n.info.Mode())
	}

	if hasSize := *(flags[constant.Size].(*bool)); hasSize {
		if hasVerbose := *(flags[constant.Verbose].(*bool)); hasVerbose {
			meta = fmt.Sprintf("%v  %v", meta, n.info.Size())
		} else {
			meta = fmt.Sprintf("%v  %v", meta, formatSize(n.info.Size()))
		}
	}

	if hasDate := *(flags[constant.Date].(*bool)); hasDate {
		meta = fmt.Sprintf("%v  %v", meta, n.info.ModTime().Format("2-Jan-06 15:04"))
	}

	if meta != "" {
		if strings.HasPrefix(meta, strings.Repeat(" ", 2)) {
			meta = meta[2:]
		}
		name = fmt.Sprintf("[%v] %v", meta, name)
	}

	if hasColor && outputFile == "" {
		name = colorize(nameColor, name)
	}

	fmt.Fprintf(wr, "%s%s\n", line, name)
}

func (n *node) drawJson(w io.Writer, flags map[string]interface{}) {
	jn := parseJNode(n)
	if b, err := json.Marshal(jn); err == nil {
		_, _ = w.Write(b)
	}
}

func parseJNode(n *node) *jnode {
	jn := &jnode{}
	for _, _n := range n.nodes {
		if _n.nodes != nil {
			if jn.Dirs == nil {
				jn.Dirs = map[string]*jnode{}
			}
			jn.Dirs[filepath.Base(_n.path)] = parseJNode(&_n)
		} else {
			if jn.Files == nil {
				jn.Files = []string{}
			}
			jn.Files = append(jn.Files, filepath.Base(_n.path))
		}
	}
	return jn
}
