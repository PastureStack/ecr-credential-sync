package logrus

import (
	"bufio"
	"io"
	"runtime"
	"strings"
)

func (logger *Logger) Writer() *io.PipeWriter {
	reader, writer := io.Pipe()

	go logger.writerScanner(reader)
	runtime.SetFinalizer(writer, writerFinalizer)

	return writer
}

func (logger *Logger) writerScanner(reader *io.PipeReader) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, bufio.MaxScanTokenSize), bufio.MaxScanTokenSize)
	chunkSize := bufio.MaxScanTokenSize
	scanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		if len(data) >= chunkSize {
			return chunkSize, data[:chunkSize], nil
		}

		return bufio.ScanLines(data, atEOF)
	})
	for scanner.Scan() {
		logger.Print(strings.TrimRight(scanner.Text(), "\r\n"))
	}
	if err := scanner.Err(); err != nil {
		logger.Errorf("Error while reading from Writer: %s", err)
	}
	reader.Close()
}

func writerFinalizer(writer *io.PipeWriter) {
	writer.Close()
}
