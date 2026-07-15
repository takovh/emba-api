package utils

import (
	"os"
	"strings"
)

type LogReader struct {
	file   *os.File
	offset int64
	path   string
}

func NewLogReader(path string) *LogReader {
	return &LogReader{path: path}
}

func (r *LogReader) ReadNewLines() ([]string, error) {
	if r.file == nil {
		f, err := os.Open(r.path)
		if err != nil {
			return nil, nil
		}
		r.file = f
	}

	r.file.Seek(r.offset, 0)
	buf := make([]byte, 32*1024)
	n, err := r.file.Read(buf)
	if n == 0 {
		return nil, nil
	}
	r.offset, _ = r.file.Seek(0, 1)
	data := strings.ToValidUTF8(string(buf[:n]), "�")
	lines := strings.Split(data, "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if line != "" {
			result = append(result, line)
		}
	}
	return result, err
}

func (r *LogReader) Close() {
	if r.file != nil {
		r.file.Close()
		r.file = nil
	}
}
