package yvalidate

import (
	"github.com/lithammer/dedent"
	. "gopkg.in/check.v1"
	"reflect"
	"strings"
	"testing"
)

func Test(t *testing.T) { TestingT(t) }

type InvalidSuite struct{}

var _ = Suite(&InvalidSuite{})

type TestDescription struct {
	decodeType interface{}
	yaml       string
	errors     []string
}

var tests []TestDescription = []TestDescription{
	{
		struct {
			Foo []string `yaml:"foo_bar" validate:"dive,eq=Blah"`
		}{},
		`
     foo_bar:
       - "Blah2"
     `,
		[]string{"foo_bar[0] (<input>:3:4) is not equal to Blah"},
	},
	{
		struct {
			Foo string `validate:"eqfield=Bar"`
			Bar string `validate:"eq=Baz"`
		}{},
		`
     bar: "Rats"
     foo: "Blah"
    `,
		[]string{
			"foo (<input>:3:5) must be equal to bar (<input>:2:5)",
			"bar (<input>:2:5) is not equal to Baz",
		},
	},
	{
		struct {
			Emb struct {
				Foo string `validate:"eq=Baz"`
			}
			Bar string `validate:"eqfield=Emb.Foo"`
		}{},
		`
     emb:
         foo: "Bar"
     bar: "Zap"
    `,
		[]string{"emb.foo (<input>:3:9) is not equal to Baz", "bar (<input>:4:5) must be equal to emb.foo (<input>:3:9)"},
	},
	{
		struct {
			Bar struct {
				VersionString string `yaml:"version_string" validate:"required"`
			}  `yaml:"bar,omitempty"`
			Foo int32
		}{},
		`foo: 1`,
		[]string{"bar.version_string is a required field"},
	},
}

func (s *InvalidSuite) TestInvalidStruct(c *C) {
	for i, t := range tests {
		c.Logf("Test %d: %s", i, t.yaml)
		rv := reflect.New(reflect.TypeOf(t.decodeType)).Interface()
		err := DecodeStruct(strings.NewReader(dedent.Dedent(t.yaml)), "<input>", rv)
		if t.errors == nil {
			c.Check(err, DeepEquals, nil)
		} else if te, ok := err.(TranslatedErrors); ok {
			c.Check([]string(te), DeepEquals, t.errors)
		} else {
			c.Assert(err, Equals, nil)
		}
	}
}

func (s *InvalidSuite) TestInvalidField(c *C) {
	v := struct {
		Foo int32  `yaml:"foo"`
	}{}

	err := DecodeStruct(strings.NewReader(`bar: 2`), "file name", &v)
	c.Assert(err.Error(), Equals,
		"yaml: unmarshal errors:\n" +
		`  line 1: field bar not found in type struct { Foo int32 "yaml:\"foo\"" }`)
}

func (s *InvalidSuite) TestMap(c *C) {
	m := map[string]interface{}{}
	err := DecodeVar(strings.NewReader("foo: 1\nbar: 3"), "<input>", m, "dive,keys,eq=foo|eq=bar,endkeys,eq=1|eq=2")
	te, ok := err.(TranslatedErrors)
	c.Assert(ok, Equals, true, Commentf("Error: %v", err))
	c.Check([]string(te), DeepEquals, []string{"Field [bar] (<input>:2:5): validation failed on the 'eq=1|eq=2' tag"})
}
