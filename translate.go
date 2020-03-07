package yvalidate

import (
	"fmt"
	"github.com/areusch/yvalidate/validator"
	"github.com/areusch/yvalidate/validator/translations/en"
	"github.com/areusch/yvalidate/yaml" // Forked copy of go-yaml
	"github.com/go-playground/locales/en_US"
	ut "github.com/go-playground/universal-translator"
	"io"
	"reflect"
	"strings"
)

type mappedFieldError struct {
	validator.FieldError
	fileName string
	sm       *sourceMapper
	t reflect.Type
}

func (e mappedFieldError) Field() string {
	return e.Namespace()
}

func (e mappedFieldError) StructField() string {
	return e.StructNamespace()
}

func (e mappedFieldError) Namespace() string {
	return e.sm.Translate(e.t, e.FieldError.Namespace(), e.fileName)
}

func (e mappedFieldError) StructNamespace() string {
	return e.sm.Translate(e.t, e.FieldError.StructNamespace(), e.fileName)
}

const (
	fieldErrMsg = "Field %s: validation failed on the '%s' tag"
)

func (e mappedFieldError) Error() string {
	return fmt.Sprintf(fieldErrMsg, e.Namespace(), e.Tag())
}

type sourceMapper struct {
	Prefix string
	smap   map[string]yaml.SourceMap
}

func (t *sourceMapper) AddSourceMap(smap yaml.SourceMap) {
	if t.smap == nil {
		t.smap = make(map[string]yaml.SourceMap)
	}
	t.smap[smap.GoName] = smap
}

func (t *sourceMapper) Translate(target reflect.Type, f string, fileName string) string {
	if strings.HasPrefix(f, t.Prefix) {
		f = f[len(t.Prefix):]
	}
	if xf, ok := t.smap[f]; ok {
		return fmt.Sprintf("%s (%s:%d:%d)", xf.YamlName, fileName, xf.Line, xf.Column)
	}

	if target == nil {
		return f
	}

	parts := strings.Split(f, ".")
	yamlParts := make([]string, 0, len(parts))
	for _, p := range parts {
		for k := target.Kind(); k != reflect.Struct; k = target.Kind() {
			switch k := target.Kind(); k {
			case reflect.Ptr:
				fallthrough
			case reflect.Slice:
				target = target.Elem()
			default:
				panic(fmt.Sprintf("unexpected kind: %s", k))
			}
		}

		key := p
		suffix := ""
		if idx := strings.Index(p, "["); idx != -1 {
			key = p[:idx]
			suffix = p[idx:]
		}
		if sf, ok := target.FieldByName(key); !ok {
			return f
		} else {
			target = sf.Type
			if tag, ok := sf.Tag.Lookup("yaml"); ok {
				yamlParts = append(yamlParts, fmt.Sprintf("%s%s", strings.SplitN(tag, ",", 2)[0], suffix))
			} else {
				yamlParts = append(yamlParts, p)
			}
  	}
	}

	return strings.Join(yamlParts, ".")
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

func decodeInternal(r io.Reader, fileName string, i interface{}) (t ut.Translator, sm sourceMapper, v *validator.Validate, err error) {
	dec := yaml.NewDecoder(r)
	dec.SetSourceMapReceiver(sm.AddSourceMap)
	dec.SetStrict(true)
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

	iType := reflect.TypeOf(i)
	tf := func(ut ut.Translator, fe validator.FieldError) string {
		param := fe.Param()
		if strings.HasSuffix(fe.Tag(), "field") {
			param = sm.Translate(iType, param, fileName)
		}
		result, err := ut.T(fe.Tag(), sm.Translate(iType, fe.StructNamespace(), fileName), param)
		if err != nil {
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

func translateErrors(ve validator.ValidationErrors, v *validator.Validate, t ut.Translator, fileName string, target reflect.Type, sm sourceMapper) (te TranslatedErrors) {
	for _, x := range ve {
		mfe := mappedFieldError{x, fileName, &sm, target}
		if translated, success := v.TranslateError(t, mfe); success {
			te = append(te, translated)
		} else {
			te = append(te, mfe.Error())
		}
	}

	return
}

// Parse one yaml document from 'i' into the struct pointed to by 'i', then
// checks the parsed struct against any `validate` tags present. Any validation
// errors returned cite the invalid field's location by 1-based line and column
// number. Those locations are referenced to `fileName`.
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

	ti := rv.Type()
	if ti.Name() != "" {
		sm.Prefix = ti.Name() + "."
	}

	err = v.Struct(i)
	if err != nil {
		if ve, ok := err.(validator.ValidationErrors); ok {
			return translateErrors(ve, v, t, fileName, reflect.TypeOf(i), sm)
		}
	}

	return err
}

// Decode one yaml literal from `r` into the pointer `m`. Then, validation is
// performed according to `tag` (as if it were a tag on the field containing
// `m`). Any validation errors returned cite the invalid field's location by
// 1-based line and column number. Those locations are referenced to `fileName`.
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
			return translateErrors(ve, v, t, fileName, nil, sm)
		}
	}

	return err
}
