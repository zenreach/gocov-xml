package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"go/token"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/axw/gocov"
)

type Coverage struct {
	XMLName    xml.Name  `xml:"coverage"`
	LineRate   float32   `xml:"line-rate,attr"`
	BranchRate float32   `xml:"branch-rate,attr"`
	Version    string    `xml:"version,attr"`
	Timestamp  int64     `xml:"timestamp,attr"`
	Packages   []Package `xml:"packages>package"`
}

type Package struct {
	Name       string  `xml:"name,attr"`
	LineRate   float32 `xml:"line-rate,attr"`
	BranchRate float32 `xml:"branch-rate,attr"`
	Complexity float32 `xml:"complexity,attr"`
	Classes    []Class `xml:"classes>class"`
	LineCount  int64   `xml:"line-count,attr"`
	LineHits   int64   `xml:"line-hits,attr"`
}

type Class struct {
	Name       string   `xml:"name,attr"`
	Filename   string   `xml:"filename,attr"`
	LineRate   float32  `xml:"line-rate,attr"`
	BranchRate float32  `xml:"branch-rate,attr"`
	Complexity float32  `xml:"complexity,attr"`
	Methods    []Method `xml:"methods>method"`
	Lines      []Line   `xml:"lines>line"`
	LineCount  int64    `xml:"line-count,attr"`
	LineHits   int64    `xml:"line-hits,attr"`
}

type Method struct {
	Name       string  `xml:"name,attr"`
	Signature  string  `xml:"signature,attr"`
	LineRate   float32 `xml:"line-rate,attr"`
	BranchRate float32 `xml:"branch-rate,attr"`
	Lines      []Line  `xml:"lines>line"`
	LineCount  int64   `xml:"line-count,attr"`
	LineHits   int64   `xml:"line-hits,attr"`
}

type Line struct {
	Number int   `xml:"number,attr"`
	Hits   int64 `xml:"hits,attr"`
}

func main() {
	var r struct{ Packages []gocov.Package }
	var total_lines, total_hits int64 = 0, 0
	err := json.NewDecoder(os.Stdin).Decode(&r)
	if err != nil {
		panic(err)
	}

	fset := token.NewFileSet()
	tokenFiles := make(map[string]*token.File)

	// convert packages
	packages := make([]Package, len(r.Packages))
	for i, gPackage := range r.Packages {
		// group functions by filename and "class" (type)
		files := make(map[string]map[string]*Class)
		for _, gFunction := range gPackage.Functions {
			classes := files[gFunction.File]
			if classes == nil {
				// group functions by "class" (type) in a File
				classes = make(map[string]*Class)
				files[gFunction.File] = classes
			}

			s := strings.Split("-."+gFunction.Name, ".") // className is "-" for package-level functions
			className, methodName := s[len(s)-2], s[len(s)-1]
			class := classes[className]
			if class == nil {
				class = &Class{Name: className, Filename: gFunction.File, Methods: []Method{}, Lines: []Line{}, LineCount: 0, LineHits: 0}
				classes[className] = class
			}

			// from github.com/axw/gocov /gocov/annotate.go#printFunctionSource
			// Load the file for line information. Probably overkill, maybe
			// just compute the lines from offsets in here.
			setContent := false
			tokenFile := tokenFiles[gFunction.File]
			if tokenFile == nil {
				info, err := os.Stat(gFunction.File)
				if err != nil {
					panic(err)
				}
				tokenFile = fset.AddFile(gFunction.File, fset.Base(), int(info.Size()))
				setContent = true
			}

			tokenData, err := ioutil.ReadFile(gFunction.File)
			if err != nil {
				panic(err)
			}
			if setContent {
				// This processes the content and records line number info.
				tokenFile.SetLinesForContent(tokenData)
			}

			// convert statements to lines
			lines := make([]Line, len(gFunction.Statements))
			func_hits := 0
			for i, s := range gFunction.Statements {
				lineno := tokenFile.Line(tokenFile.Pos(s.Start))
				line := Line{Number: lineno, Hits: s.Reached}
				func_hits += int(s.Reached)
				lines[i] = line
				class.Lines = append(class.Lines, line)
			}
			line_rate := float32(func_hits) / float32(len(gFunction.Statements))

			class.Methods = append(class.Methods, Method{Name: methodName, Lines: lines, LineRate: line_rate})
			class.LineCount += int64(len(gFunction.Statements))
			class.LineHits += int64(func_hits)
		}

		// fill package with "classes"
		p := Package{Name: gPackage.Name, Classes: []Class{}, LineCount: 0, LineHits: 0}
		for _, classes := range files {
			for _, class := range classes {
				p.LineCount += class.LineCount
				p.LineHits += class.LineHits
				class.LineRate = float32(class.LineHits) / float32(class.LineCount)
				p.Classes = append(p.Classes, *class)
			}
			p.LineRate = float32(p.LineHits) / float32(p.LineCount)
			total_lines += p.LineCount
			total_hits += p.LineHits
		}
		packages[i] = p
	}

	coverage := Coverage{Packages: packages, Timestamp: time.Now().UnixNano() / int64(time.Millisecond), LineRate: float32(total_hits) / float32(total_lines)}

	fmt.Printf(xml.Header)
	fmt.Printf("<!DOCTYPE coverage SYSTEM \"http://cobertura.sourceforge.net/xml/coverage-03.dtd\">\n")

	encoder := xml.NewEncoder(os.Stdout)
	encoder.Indent("", "\t")
	err = encoder.Encode(coverage)
	if err != nil {
		panic(err)
	}

	fmt.Println()
}
