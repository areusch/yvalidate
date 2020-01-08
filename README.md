# yaml-translator

[![Build Status](https://travis-ci.org/areusch/yvalidate.svg?branch=master)](https://travis-ci.org/areusch/yvalidate)

Decode YAML documents, valiate them, and translate validation errors back to
yaml doc line numbers.

For example, this struct definition:

	type Config struct {
		Version  int32    `yaml:"version" validate:"eq=1"`
		WordList []string `yaml:"word_list" validate:"dive,alpha"`
	}

would, when parsed against this yaml:

    version: 2
    word_list:
     - Fo2o
     - Bar

produces validation errors like:

    yaml validation failed with 2 errors:
	  - version (<input>:1:9) is not equal to 1
	  - Field word_list[0] (<input>:3:3): validation failed on the 'alpha' tag
