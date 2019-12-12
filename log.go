package rollinglogger

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	defaultMaxSize = 100
	megabyte       = 1024 * 1024
	ext            = ".gz"
	timeFormat     = "2006-01-02-15-04-05"
)

type Logger struct {
	Filename string
	MaxSize  int // in MB
	size     int
	fd       *os.File
	mu       sync.Mutex
}

func (l *Logger) Write(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	cursize := len(p)
	if cursize > l.max() {
		return 0, fmt.Errorf("write length %d larger than the maxsize", cursize, l.max())
	}
	if l.fd == nil {
		err := l.openFile(cursize)
		if err != nil {
			return 0, err
		}
	}

	if l.size+cursize > l.max() {
		err := l.makeNewFile()
		if err != nil {
			return 0, err
		}
	}

	n, err = l.fd.Write(p)
	if err != nil {
		return 0, err
	}
	l.size += n
	return n, nil
}

func (l *Logger) openFile(curlen int) error {
	fileinfo, err := os.Stat(l.Filename)
	if os.IsNotExist(err) {
		return l.openNewFile()
	}
	if err != nil {
		return fmt.Errorf("error in getting file %s stat", l.Filename)
	}
	if int(fileinfo.Size())+curlen >= l.max() {
		return l.makeNewFile()
	}
	file, err := os.OpenFile(l.Filename, os.O_WRONLY|os.O_APPEND, 0755)
	if err != nil {
		return fmt.Errorf("error in opening file %s ", l.Filename)
	}
	l.fd = file
	l.size = int(fileinfo.Size())
	return nil
}

func (l *Logger) openNewFile() error {
	file, err := os.OpenFile(l.Filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("error in opening file %s ", l.Filename)
	}
	l.fd = file
	l.size = 0
	return nil
}

func (l *Logger) makeNewFile() error {
	err := l.close()
	if err != nil {
		return err
	}

	err = l.composeFile()
	if err != nil {
		return err
	}

	err = l.openNewFile()
	if err != nil {
		return err
	}
	return nil
}

func (l *Logger) composeFile() error {
	file, err := os.Open(l.Filename)
	if err != nil {
		return fmt.Errorf("error in opening file %s ", l.Filename)
	}
	defer file.Close()

	fileinfo, err := os.Stat(l.Filename)
	if err != nil {
		return fmt.Errorf("error in getting file %s stat", l.Filename)
	}

	dst := l.getBackupFileName()
	gzf, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fileinfo.Mode())
	if err != nil {
		return fmt.Errorf("error in opening compressed log file %s")
	}
	defer gzf.Close()

	gz := gzip.NewWriter(gzf)

	_, err = io.Copy(gz, file)
	if err != nil {
		return err
	}
	err = gz.Close()
	if err != nil {
		return err
	}
	err = os.Remove(l.Filename)
	if err != nil {
		return err
	}
	return nil
}

func (l *Logger) getBackupFileName() string {
	dir := filepath.Dir(l.Filename)
	base := filepath.Base(l.Filename)
	currentTime := time.Now()
	return filepath.Join(dir, fmt.Sprintf("%s-%d-%s%s", currentTime.Format(timeFormat), currentTime.Nanosecond(), base, ext))
}

func (l *Logger) close() error {
	if l.fd == nil {
		return nil
	}
	err := l.fd.Close()
	l.fd = nil
	return err
}

func (l *Logger) max() int {
	if l.MaxSize == 0 {
		return defaultMaxSize * megabyte
	}
	return l.MaxSize * megabyte
}
