// This program generates messages.go from messages.xml

// +build ignore

package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Definitions struct {
	Message []*Message
}

func (d *Definitions) Link() {
	for _, v := range d.Message {
		v.Link()
	}
}

type Message struct {
	Name        string
	Type        string
	Description string
	Comment     string
	Firmware    string
	Class       Hex      `xml:"Structure>Class"`
	Id          Hex      `xml:"Structure>Id"`
	Length      string   `xml:"Structure>Length"` // of the form A + N * B, but varying syntax
	Blocks      []*Block `xml:"Structure>Payload>Block"`
}

func (d *Message) Link() {
	for _, v := range d.Blocks {
		v.Link(d)
	}
}

type Block struct {
	Cardinality string `xml:"type,attr"` // repeated or optional, in which case 'nested' is non nil
	CountField  string `xml:"name,attr"` // for repeated fields: name of the count field

	Offset   string
	Name     string
	Type     string
	Comment  string
	Scale    string
	Unit     string
	Bitfield []*BitDef `xml:"Bitfield>Type"`

	Nested []*Block `xml:"Block"`

	Message *Message `xml:"-"` // link back up
}

func (b *Block) Link(m *Message) {
	b.Message = m
	for _, v := range b.Bitfield {
		v.Field = b
	}
	for _, v := range b.Nested {
		v.Link(m)
	}
}

type BitDef struct {
	Index       string
	Type        string
	Name        string
	Description string

	Field *Block `xml:"-"` // link back up
}

type Hex uint64

func (v *Hex) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var f string
	if err := d.DecodeElement(&f, &start); err != nil {
		return err
	}
	vv, err := strconv.ParseUint(f, 0, 64)
	if err != nil {
		return err
	}
	*v = Hex(vv)
	return nil
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("msggen: ")
	flag.Parse()

	if len(flag.Args()) != 1 {
		log.Fatalf("Usage: %s path/to/code.tmpl < messages.xml > code.go", os.Args[0])
	}

	tmpl, err := template.New(filepath.Base(flag.Arg(0))).Funcs(tmplfuncs).ParseFiles(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	log.Println("template file:", tmpl.Name())

	var definitions Definitions

	if err := xml.NewDecoder(os.Stdin).Decode(&definitions); err != nil {
		log.Fatal(err)
	}

	definitions.Link()

	fmt.Printf("// Code generated by go run msggen.go %s; DO NOT EDIT.\n", flag.Arg(0))

	if err := tmpl.Execute(os.Stdout, definitions); err != nil {
		log.Fatal(err)
	}

}

var tmplfuncs = template.FuncMap{
	"lower":       strings.ToLower,
	"upper":       strings.ToUpper,
	"title":       strings.Title,
	"notabs":      notabs,
	"msgtypename": msgtypename,
	"gotype":      goType,
	"mask":        mask,
}

var wstospace = strings.NewReplacer("\t", " ", "\n", " ")

func notabs(s string) string { return wstospace.Replace(s) }

func msgtypename(s string) string {
	parts := strings.Split(strings.ToLower(s), "-")
	for i, v := range parts {
		parts[i] = strings.Title(v)
	}
	return strings.Join(parts[1:], "")
}

var reCType = regexp.MustCompile(`([CHIRUX0-9_]+)(\[[0-9]+\])?`)

func goType(ctype string) (string, error) {
	parts := reCType.FindStringSubmatch(ctype)
	if len(parts) != 3 {
		return "", fmt.Errorf("Cannot parse %q as ctype([arraylen])", ctype)
	}
	var t string
	switch parts[1] {
	case "RU1_3":
		t = "Float8"
	case "R4":
		t = "float32"
	case "R8":
		t = "float64"
	case "I1":
		t = "int8"
	case "U1", "CH", "X1":
		t = "byte"
	case "I2":
		t = "int16"
	case "U2", "X2":
		t = "uint16"
	case "I4":
		t = "int32"
	case "U4", "X4":
		t = "uint32"
	case "I8":
		t = "int64"
	case "U8":
		t = "uint64"
	default:
		return "", fmt.Errorf("Cannot parse %q as a ctype (invalid scalar part %q)", ctype, parts[1])
	}
	if parts[2] == "" {
		return t, nil
	}
	return fmt.Sprintf("%s%s", parts[2], t), nil
}

func mask(s string) string {
	parts := strings.Split(s, ":")
	if len(parts) == 2 {
		hi, _ := strconv.ParseUint(parts[0], 0, 8)
		lo, _ := strconv.ParseUint(parts[1], 0, 8)
		return fmt.Sprintf("0x%x", ((1<<(hi+1))-1)^((1<<(lo))-1))
	}
	i, _ := strconv.ParseUint(s, 0, 8)
	return fmt.Sprintf("0x%x", 1<<i)
}
