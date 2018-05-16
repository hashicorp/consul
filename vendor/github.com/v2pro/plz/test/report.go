package test

import (
	"io/ioutil"
	"bytes"
)

var newLine = []byte("\n")
var checkFailed = []byte("failed: ")

func ExtractFailedLines(file string, line int) string {
	fileContent, err := ioutil.ReadFile(file)
	if err != nil {
		return "check failed"
	}
	line = line - 1
	lines := bytes.Split(fileContent, newLine)
	if line < 0 || line >= len(lines) {
		return "check failed"
	}
	report := checkFailed
	openBraces := 0
	for ;line < len(lines);line++ {
		report = append(report, bytes.TrimSpace(lines[line])...)
		openBraces += bytes.Count(lines[line], []byte{'('})
		openBraces -= bytes.Count(lines[line], []byte{')'})
		if openBraces == 0 {
			break
		}
	}
	return string(report)
}
