// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This program generates a test to verify that the standard arithmetic
// operators properly handle const cases. The test file should be
// generated with a known working version of go.
// launch with `go run arithConstGen.go` a file called arithConst.go
// will be written into the parent directory containing the tests

package main

import (
	"bytes"
	"fmt"
	"go/format"
	"io/ioutil"
	"log"
	"strings"
	"text/template"
)

type op struct {
	name, symbol string
}
type szD struct {
	name string
	sn   string
	u    []uint64
	i    []int64
}

var szs = []szD{
	{name: "uint64", sn: "64", u: []uint64{0, 1, 4294967296, 0x8000000000000000, 0xffffFFFFffffFFFF}},
	{name: "int64", sn: "64", i: []int64{-0x8000000000000000, -0x7FFFFFFFFFFFFFFF,
		-4294967296, -1, 0, 1, 4294967296, 0x7FFFFFFFFFFFFFFE, 0x7FFFFFFFFFFFFFFF}},

	{name: "uint32", sn: "32", u: []uint64{0, 1, 4294967295}},
	{name: "int32", sn: "32", i: []int64{-0x80000000, -0x7FFFFFFF, -1, 0,
		1, 0x7FFFFFFF}},

	{name: "uint16", sn: "16", u: []uint64{0, 1, 65535}},
	{name: "int16", sn: "16", i: []int64{-32768, -32767, -1, 0, 1, 32766, 32767}},

	{name: "uint8", sn: "8", u: []uint64{0, 1, 255}},
	{name: "int8", sn: "8", i: []int64{-128, -127, -1, 0, 1, 126, 127}},
}

var ops = []op{
	{"add", "+"},
	{"sub", "-"},
	{"div", "/"},
	{"mul", "*"},
	{"lsh", "<<"},
	{"rsh", ">>"},
	{"mod", "%"},
	{"and", "&"},
	{"or", "|"},
	{"xor", "^"},
}

// compute the result of i op j, cast as type t.
func ansU(i, j uint64, t, op string) string {
	var ans uint64
	switch op {
	case "+":
		ans = i + j
	case "-":
		ans = i - j
	case "*":
		ans = i * j
	case "/":
		if j != 0 {
			ans = i / j
		}
	case "%":
		if j != 0 {
			ans = i % j
		}
	case "<<":
		ans = i << j
	case ">>":
		ans = i >> j
	case "&":
		ans = i & j
	case "|":
		ans = i | j
	case "^":
		ans = i ^ j
	}
	switch t {
	case "uint32":
		ans = uint64(uint32(ans))
	case "uint16":
		ans = uint64(uint16(ans))
	case "uint8":
		ans = uint64(uint8(ans))
	}
	return fmt.Sprintf("%d", ans)
}

// compute the result of i op j, cast as type t.
func ansS(i, j int64, t, op string) string {
	var ans int64
	switch op {
	case "+":
		ans = i + j
	case "-":
		ans = i - j
	case "*":
		ans = i * j
	case "/":
		if j != 0 {
			ans = i / j
		}
	case "%":
		if j != 0 {
			ans = i % j
		}
	case "<<":
		ans = i << uint64(j)
	case ">>":
		ans = i >> uint64(j)
	case "&":
		ans = i & j
	case "|":
		ans = i | j
	case "^":
		ans = i ^ j
	}
	switch t {
	case "int32":
		ans = int64(int32(ans))
	case "int16":
		ans = int64(int16(ans))
	case "int8":
		ans = int64(int8(ans))
	}
	return fmt.Sprintf("%d", ans)
}

func main() {
	w := new(bytes.Buffer)
	fmt.Fprintf(w, "// run\n")
	fmt.Fprintf(w, "// Code generated by gen/arithConstGen.go. DO NOT EDIT.\n\n")
	fmt.Fprintf(w, "package main;\n")
	fmt.Fprintf(w, "import \"fmt\"\n")
	fmt.Fprintf(w, "import \"os\"\n")

	fncCnst1 := template.Must(template.New("fnc").Parse(
		`//go:noinline
func {{.Name}}_{{.Type_}}_{{.FNumber}}(a {{.Type_}}) {{.Type_}} { return a {{.Symbol}} {{.Number}} }
`))
	fncCnst2 := template.Must(template.New("fnc").Parse(
		`//go:noinline
func {{.Name}}_{{.FNumber}}_{{.Type_}}(a {{.Type_}}) {{.Type_}} { return {{.Number}} {{.Symbol}} a }
`))

	type fncData struct {
		Name, Type_, Symbol, FNumber, Number string
	}

	for _, s := range szs {
		for _, o := range ops {
			fd := fncData{o.name, s.name, o.symbol, "", ""}

			// unsigned test cases
			if len(s.u) > 0 {
				for _, i := range s.u {
					fd.Number = fmt.Sprintf("%d", i)
					fd.FNumber = strings.Replace(fd.Number, "-", "Neg", -1)

					// avoid division by zero
					if o.name != "mod" && o.name != "div" || i != 0 {
						// introduce uint64 cast for rhs shift operands
						// if they are too large for default uint type
						number := fd.Number
						if (o.name == "lsh" || o.name == "rsh") && uint64(uint32(i)) != i {
							fd.Number = fmt.Sprintf("uint64(%s)", number)
						}
						fncCnst1.Execute(w, fd)
						fd.Number = number
					}

					fncCnst2.Execute(w, fd)
				}
			}

			// signed test cases
			if len(s.i) > 0 {
				// don't generate tests for shifts by signed integers
				if o.name == "lsh" || o.name == "rsh" {
					continue
				}
				for _, i := range s.i {
					fd.Number = fmt.Sprintf("%d", i)
					fd.FNumber = strings.Replace(fd.Number, "-", "Neg", -1)

					// avoid division by zero
					if o.name != "mod" && o.name != "div" || i != 0 {
						fncCnst1.Execute(w, fd)
					}
					fncCnst2.Execute(w, fd)
				}
			}
		}
	}

	vrf1 := template.Must(template.New("vrf1").Parse(`
		test_{{.Size}}{fn: {{.Name}}_{{.FNumber}}_{{.Type_}}, fnname: "{{.Name}}_{{.FNumber}}_{{.Type_}}", in: {{.Input}}, want: {{.Ans}}},`))

	vrf2 := template.Must(template.New("vrf2").Parse(`
		test_{{.Size}}{fn: {{.Name}}_{{.Type_}}_{{.FNumber}}, fnname: "{{.Name}}_{{.Type_}}_{{.FNumber}}", in: {{.Input}}, want: {{.Ans}}},`))

	type cfncData struct {
		Size, Name, Type_, Symbol, FNumber, Number string
		Ans, Input                                 string
	}
	for _, s := range szs {
		fmt.Fprintf(w, `
type test_%[1]s struct {
	fn func (%[1]s) %[1]s
	fnname string
	in %[1]s
	want %[1]s
}
`, s.name)
		fmt.Fprintf(w, "var tests_%[1]s =[]test_%[1]s {\n\n", s.name)

		if len(s.u) > 0 {
			for _, o := range ops {
				fd := cfncData{s.name, o.name, s.name, o.symbol, "", "", "", ""}
				for _, i := range s.u {
					fd.Number = fmt.Sprintf("%d", i)
					fd.FNumber = strings.Replace(fd.Number, "-", "Neg", -1)

					// unsigned
					for _, j := range s.u {

						if o.name != "mod" && o.name != "div" || j != 0 {
							fd.Ans = ansU(i, j, s.name, o.symbol)
							fd.Input = fmt.Sprintf("%d", j)
							if err := vrf1.Execute(w, fd); err != nil {
								panic(err)
							}
						}

						if o.name != "mod" && o.name != "div" || i != 0 {
							fd.Ans = ansU(j, i, s.name, o.symbol)
							fd.Input = fmt.Sprintf("%d", j)
							if err := vrf2.Execute(w, fd); err != nil {
								panic(err)
							}
						}

					}
				}

			}
		}

		// signed
		if len(s.i) > 0 {
			for _, o := range ops {
				// don't generate tests for shifts by signed integers
				if o.name == "lsh" || o.name == "rsh" {
					continue
				}
				fd := cfncData{s.name, o.name, s.name, o.symbol, "", "", "", ""}
				for _, i := range s.i {
					fd.Number = fmt.Sprintf("%d", i)
					fd.FNumber = strings.Replace(fd.Number, "-", "Neg", -1)
					for _, j := range s.i {
						if o.name != "mod" && o.name != "div" || j != 0 {
							fd.Ans = ansS(i, j, s.name, o.symbol)
							fd.Input = fmt.Sprintf("%d", j)
							if err := vrf1.Execute(w, fd); err != nil {
								panic(err)
							}
						}

						if o.name != "mod" && o.name != "div" || i != 0 {
							fd.Ans = ansS(j, i, s.name, o.symbol)
							fd.Input = fmt.Sprintf("%d", j)
							if err := vrf2.Execute(w, fd); err != nil {
								panic(err)
							}
						}

					}
				}

			}
		}

		fmt.Fprintf(w, "}\n\n")
	}

	fmt.Fprint(w, `
var failed bool

func main() {
`)

	for _, s := range szs {
		fmt.Fprintf(w, `for _, test := range tests_%s {`, s.name)
		// Use WriteString here to avoid a vet warning about formatting directives.
		w.WriteString(`if got := test.fn(test.in); got != test.want {
			fmt.Printf("%s(%d) = %d, want %d\n", test.fnname, test.in, got, test.want)
			failed = true
		}
	}
`)
	}

	fmt.Fprint(w, `
	if failed {
		os.Exit(1)
    }
}
`)

	// gofmt result
	b := w.Bytes()
	src, err := format.Source(b)
	if err != nil {
		fmt.Printf("%s\n", b)
		panic(err)
	}

	// write to file
	err = ioutil.WriteFile("../arithConst.go", src, 0666)
	if err != nil {
		log.Fatalf("can't write output: %v\n", err)
	}
}
