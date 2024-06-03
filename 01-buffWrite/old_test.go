package examples

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"os"
	"testing"
	"time"
)

type loggerv1 struct {
	f *os.File
}

func (l *loggerv1) Append(stuff []byte) {
	l.f.Write(stuff)
	l.f.Write([]byte("\n"))
}

func (l *loggerv1) Flush() {
	l.f.Sync()
}

func createRandomFile() (filepath string, f *os.File) {
	os.MkdirAll("./tmp", os.ModePerm)
	filePath := "./tmp/" + fmt.Sprintf("output-%d", time.Now().UnixNano())
	os.Remove(filePath)
	fo, err := os.Create(filePath)
	if err != nil {
		panic(err)
	}

	return filePath, fo
}

var randomContent [][]byte

func init() {
	randomContent = make([][]byte, 100)
	for i := 0; i < 100; i++ {
		randomBytes := make([]byte, 512)
		_, err := rand.Read(randomBytes)
		if err != nil {
			fmt.Println("Error generating random bytes:", err)
			return
		}

		randomContent[i] = randomBytes
	}
}

func BenchmarkWritesv1(t *testing.B) {
	for eps := 32; eps <= 512; eps = eps * 2 {
		t.Run(fmt.Sprintf("loggver - v1, entries: %d", eps), func(t *testing.B) {
			_, f := createRandomFile()
			l := loggerv1{f}

			for i := 0; i < t.N; i++ {
				for i := 0; i < eps; i++ {
					l.Append(randomContent[eps%len(randomContent)])
				}
			}
		})
	}

	os.RemoveAll("./tmp/")
}

type loggerv2 struct {
	f    *os.File
	buff *bufio.Writer
}

func (l *loggerv2) Append(stuff []byte) {
	l.buff.Write(stuff)
	l.buff.Write([]byte("\n"))
}

func (l *loggerv2) Flush() {
	l.buff.Flush()
	l.f.Sync()
}

func BenchmarkWritesv2(t *testing.B) {
	for eps := 32; eps <= 512; eps = eps * 4 {
		t.Run(fmt.Sprintf("loggver-v1,entries:%d", eps), func(t *testing.B) {
			_, f := createRandomFile()
			l := loggerv1{f}

			for i := 0; i < t.N; i++ {
				for i := 0; i < eps; i++ {
					l.Append(randomContent[eps%len(randomContent)])
				}
			}
		})

		bufferSize := 1024 * 1024 * 5
		t.Run(fmt.Sprintf("loggver-v2,entries:%d", eps), func(t *testing.B) {
			_, f := createRandomFile()
			l := loggerv2{f, bufio.NewWriterSize(f, bufferSize)}

			for i := 0; i < t.N; i++ {
				for i := 0; i < eps; i++ {
					l.Append(randomContent[eps%len(randomContent)])
				}
			}

			l.Flush()
		})

	}

	os.RemoveAll("./tmp/")
}
