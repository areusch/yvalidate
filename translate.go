package yaml_translator

import (
	"fmt"
	"github.com/areusch/yaml-translator/validator"
	"github.com/areusch/yaml-translator/validator/translations/en"
	"github.com/areusch/yaml-translator/yaml" // Forked copy of go-yaml
	"github.com/go-playground/locales/en_US"
	ut "github.com/go-playground/universal-translator"
	"io"
	"reflect"
	"strings"
)

type MappedFieldError struct {
	validator.FieldError
	fileName string
	sm       *SourceMapper
}

func (e MappedFieldError) Field() string {
	return e.Namespace()
}

func (e MappedFieldError) StructField() string {
	return e.StructNamespace()
}

func (e MappedFieldError) Namespace() string {
	return e.sm.Translate(e.FieldError.Namespace(), e.fileName)
}

func (e MappedFieldError) StructNamespace() string {
	return e.sm.Translate(e.FieldError.StructNamespace(), e.fileName)
}

const (
	fieldErrMsg = "Field %s: validation failed on the '%s' tag"
)

func (e MappedFieldError) Error() string {
	return fmt.Sprintf(fieldErrMsg, e.Namespace(), e.Tag())
}

type SourceMapper struct {
	smap map[string]yaml.SourceMap
}

func (t *SourceMapper) AddSourceMap(smap yaml.SourceMap) {
	if t.smap == nil {
		t.smap = make(map[string]yaml.SourceMap)
	}
	t.smap[smap.GoName] = smap
}

func (t *SourceMapper) Translate(f string, fileName string) string {
	if xf, ok := t.smap[f]; ok {
		return fmt.Sprintf("%s (%s:%d:%d)", xf.YamlName, fileName, xf.Line, xf.Column)
	}

	return f
}

var translations []string = []string{
	"required",
	"len",
	"min",
	"max",
	"eq",
	"ne",
	"lt",
	"lte",
	"gt",
	"gte",
	"eqfield",
	"eqcsfield",
	"necsfield",
	"gtcsfield",
}

type TranslatedErrors []string

func (t TranslatedErrors) Error() string {
	lines := make([]string, 0, len(t))
	for _, v := range t {
		lines = append(lines, fmt.Sprintf(" - %s", v))
	}

	return fmt.Sprintf("yaml validation failed with %d errors:\n%s", len(t), strings.Join(lines, "\n"))
}

func registrationFunc(tag string, translation string, override bool) validator.RegisterTranslationsFunc {
	return func(ut ut.Translator) (err error) {
		if err = ut.Add(tag, translation, override); err != nil {
			return
		}
		return
	}
}

func decodeInternal(r io.Reader, fileName string, i interface{}) (t ut.Translator, sm SourceMapper, v *validator.Validate, err error) {
	dec := yaml.NewDecoder(r)
	dec.SetSourceMapReceiver(sm.AddSourceMap)
	err = dec.Decode(i)
	if err != nil {
		return
	}

	tv := validator.New()
	trans := ut.New(en_US.New(), en_US.New())
	t, found := trans.GetTranslator("en_US")
	if !found {
		err = fmt.Errorf("No such translator: en_US")
		return
	}
	err = en.RegisterDefaultTranslations(tv, t)

	tf := func(ut ut.Translator, fe validator.FieldError) string {
		param := fe.Param()
		if strings.HasSuffix(fe.Tag(), "field") {
			param = sm.Translate(param, fileName)
		}
		result, err := ut.T(fe.Tag(), sm.Translate(fe.StructNamespace(), fileName), param)
		if err != nil {
			fmt.Printf("warning: error translating FieldError: %#v", fe)
			return fe.(error).Error()
		}

		return result
	}

	v = validator.New()
	for _, tag := range translations {
		v.RegisterTranslation(tag, t, func(ut ut.Translator) error { return nil }, tf)
	}

	return
}

func translateErrors(ve validator.ValidationErrors, v *validator.Validate, t ut.Translator, fileName string, sm SourceMapper) (te TranslatedErrors) {
	for _, x := range ve {
		mfe := MappedFieldError{x, fileName, &sm}
		if translated, success := v.TranslateError(t, mfe); success {
			te = append(te, translated)
		} else {
			te = append(te, mfe.Error())
		}
	}

	return
}

func DecodeVar(r io.Reader, fileName string, m interface{}, tag string) error {
	rv := reflect.ValueOf(m)
	if rv.Kind() != reflect.Map {
		return fmt.Errorf("DecodeMap: want map, got %s", rv.Kind())
	}

	t, sm, v, err := decodeInternal(r, fileName, m)
	if err != nil {
		return err
	}

	err = v.Var(m, tag)
	if err != nil {
		if ve, ok := err.(validator.ValidationErrors); ok {
			return translateErrors(ve, v, t, fileName, sm)
		}
	}

	return err
}

func DecodeStruct(r io.Reader, fileName string, i interface{}) error {
	rv := reflect.ValueOf(i)
	k := rv.Kind()
	if k == reflect.Ptr {
		rv = rv.Elem()
		k = rv.Kind()
	}
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("DecodeStruct: want struct, got %v", rv.Kind())
	}

	t, sm, v, err := decodeInternal(r, fileName, i)
	if err != nil {
		return err
	}
	err = v.Struct(i)
	if err != nil {
		if ve, ok := err.(validator.ValidationErrors); ok {
			return translateErrors(ve, v, t, fileName, sm)
		}
	}

	return err
}
