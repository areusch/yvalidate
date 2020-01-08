package yvalidate

import (
	"fmt"
	"strings"
)

func ExampleStruct() {
	type Config struct {
		Version  int32    `yaml:"version" validate:"eq=1"`
		WordList []string `yaml:"word_list" validate:"dive,alpha"`
	}

	yaml := strings.Join([]string{
		"version: 2",
		"word_list:",
		" - Fo2o",
		" - Bar"}, "\n")

	c := Config{}
	err := DecodeStruct(strings.NewReader(yaml), "<input>", &c)
	fmt.Printf("Err: %s\n", err)
	// Output:
	// Err: yaml validation failed with 2 errors:
	//  - version (<input>:1:9) is not equal to 1
	//  - Field word_list[0] (<input>:3:3): validation failed on the 'alpha' tag
}
