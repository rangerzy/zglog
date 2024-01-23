package zglog

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	//  文件时间格式
	backupTimeFormat = "2006-01-02"
	//  压缩文件猴嘴
	compressSuffix = ".zip"
	// 日志文件后缀
	logSuffix = ".log"
	//  压缩文件夹
	compressDir = "./backup"
	//  默认文件最大大小
	defaultMaxSize = 100
)

// 日志文件次数
var counter = 0

// 创建压缩文件次数
var compressCounter = 0

// ensure we always implement io.WriteCloser
var _ io.WriteCloser = (*FLogger)(nil)

// FLogger
// @Description: 自定义文件日志对象
type FLogger struct {
	//
	//  Prefix
	//  @Description: 日志文件前缀
	//
	Prefix string `json:"prefix" yaml:"prefix"`

	//
	//  MaxSize
	//  @Description: 单个日志文件最大大小
	//
	MaxSize int `json:"maxsize" yaml:"maxsize"`

	//
	//  LocalTime
	//  @Description: 是否本地时区
	//
	LocalTime bool `json:"localtime" yaml:"localtime"`

	size int64
	file *os.File
	mu   sync.Mutex
}

var (
	// currentTime exists so it can be mocked out by tests.
	currentTime = time.Now

	// os_Stat exists so it can be mocked out by tests.
	osStat = os.Stat

	// megabyte is the conversion factor between MaxSize and bytes.  It is a
	// variable so tests can mock it out and not need to write megabytes of data
	// to disk.
	megabyte = 1024 * 1024
)

// Write
//
//	@Description: 日志文件写入
//	@receiver l
//	@param p 日志字节
//	@return n
//	@return err
func (l *FLogger) Write(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	writeLen := int64(len(p))
	if writeLen > l.max() {
		return 0, fmt.Errorf(
			"write length %d exceeds maximum file size %d", writeLen, l.max(),
		)
	}

	if l.file == nil {
		if err = l.openExistingOrNew(len(p)); err != nil {
			return 0, err
		}
	}

	if l.size+writeLen > l.max() {
		if err := l.rotate(); err != nil {
			return 0, err
		}
	}

	n, err = l.file.Write(p)
	l.size += int64(n)

	return n, err
}

// Close implements io.Closer, and closes the current logfile.
func (l *FLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.close()
}

// close closes the file if it is open.
func (l *FLogger) close() error {
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}

// Rotate
//
//	@Description: 主动归档日志
//	@receiver l
//	@return error
func (l *FLogger) Rotate() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	err := l.rotate()
	if err != nil {
		return err
	}
	files, err := os.ReadDir(l.dir())
	if err != nil {
		return errors.New("打开文件夹异常")
	}
	var compressFile []os.DirEntry
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if strings.HasSuffix(f.Name(), logSuffix) && f.Name() != filepath.Base(l.filename()) {
			compressFile = append(compressFile, f)
		}
	}

	compressFilePath, _ := filepath.Abs(l.compressFileName())
	_, err = os.Stat(compressFilePath)
	if err != nil {
		_ = os.MkdirAll(filepath.Dir(compressFilePath), os.ModePerm)
	}
	archive, err := os.Create(compressFilePath)
	if err != nil {
		return fmt.Errorf("创建压缩文件异常: %v", err)
	}
	defer archive.Close()
	// 使用 zip.NewWriter 创建一个新的 Writer
	zipWriter := zip.NewWriter(archive)
	// 在完成写入操作后关闭 zipWriter
	defer zipWriter.Close()
	// 循环压缩
	for _, fi := range compressFile {
		src := filepath.Join(l.dir(), fi.Name())
		f, err := os.Open(src)
		if err != nil {
			return fmt.Errorf("failed to open log file: %v", err)
		}
		fileInZip, err := zipWriter.Create(fi.Name())
		if err != nil {
			return fmt.Errorf("打开压缩文件异常", err)
		}
		_, err = io.Copy(fileInZip, f)
		if err != nil {
			return fmt.Errorf("复制压缩文件异常", err)
		}
		f.Close()
		err = os.Remove(src)
		if err != nil {
			return fmt.Errorf("文件删除异常", err)
		}
	}
	return nil
}

// rotate closes the current file, moves it aside with a timestamp in the name,
// (if it exists), opens a new file with the original filename, and then runs
// post-rotation processing and removal.
func (l *FLogger) rotate() error {
	if err := l.close(); err != nil {
		return err
	}
	if err := l.openNew(); err != nil {
		return err
	}
	return nil
}

// openNew
//
//	@Description: 创建新文件名称
//	@receiver l
//	@return error
func (l *FLogger) openNew() error {
	err := os.MkdirAll(l.dir(), 0755)
	if err != nil {
		return fmt.Errorf("can't make directories for new logfile: %s", err)
	}

	name := l.filename()
	mode := os.FileMode(0600)
	info, err := osStat(name)
	if err == nil {
		// Copy the mode off the old logfile.
		mode = info.Mode()
		// move the existing file
		newname := backupName(name, l.Prefix, l.LocalTime)
		if err := os.Rename(name, newname); err != nil {
			return fmt.Errorf("can't rename log file: %s", err)
		}
	}

	// we use truncate here because this should only get called when we've moved
	// the file ourselves. if someone else creates the file in the meantime,
	// just wipe out the contents.
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("can't open new logfile: %s", err)
	}
	l.file = f
	l.size = 0
	return nil
}

// backupName
//
//	@Description: 备份文件名称格式化
//	@param name 当前文件路径
//	@param prefix 日志文件前缀
//	@param local 是否本地时间
//	@return string 新文件名称
func backupName(path string, prefix string, local bool) string {
	dir := filepath.Dir(path)
	filename := filepath.Base(path)
	ext := filepath.Ext(filename)
	t := currentTime()
	if !local {
		t = t.UTC()
	}

	timestamp := t.Format(backupTimeFormat)
	counter = counter + 1
	return filepath.Join(dir, fmt.Sprintf("%s-%s_%d%s", filepath.Base(prefix), timestamp, counter, ext))
}

// openExistingOrNew
//
//	@Description:  打开文件或创建新文件
//	@receiver l
//	@param writeLen
//	@return error
func (l *FLogger) openExistingOrNew(writeLen int) error {

	filename := l.filename()
	info, err := osStat(filename)
	if os.IsNotExist(err) {
		return l.openNew()
	}
	if err != nil {
		return fmt.Errorf("error getting log file info: %s", err)
	}

	if info.Size()+int64(writeLen) >= l.max() {
		return l.rotate()
	}

	file, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		// if we fail to open the old log file for some reason, just ignore
		// it and open a new log file.
		return l.openNew()
	}
	l.file = file
	l.size = info.Size()
	return nil
}

// filename generates the name of the logfile from the current time.
func (l *FLogger) filename() string {
	if l.Prefix != "" {
		return fmt.Sprintf("%s%s", l.Prefix, logSuffix)
	}
	name := filepath.Base(os.Args[0]) + "-lumberjack.log"
	return filepath.Join(os.TempDir(), name)
}

// max returns the maximum size in bytes of log files before rolling.
func (l *FLogger) max() int64 {
	if l.MaxSize == 0 {
		return int64(defaultMaxSize * megabyte)
	}
	return int64(l.MaxSize) * int64(megabyte)
}

// dir returns the directory for the current filename.
func (l *FLogger) dir() string {
	return filepath.Dir(l.filename())
}

// zipDir
//
//	@Description: 压缩文件夹
//	@receiver l
//	@return string
func (l *FLogger) compressDir() string {
	return filepath.Join(filepath.Dir(l.filename()), compressDir)
}

// compressFileName
//
//	@Description: 压缩文件名称
//	@receiver l
//	@return string
func (l *FLogger) compressFileName() string {
	t := currentTime()
	if !l.LocalTime {
		t = t.UTC()
	}
	timestamp := t.Format(backupTimeFormat)
	compressCounter = compressCounter + 1
	return filepath.Join(l.compressDir(), fmt.Sprintf("%s_%d%s", timestamp, compressCounter, compressSuffix))
}
