package main

import _ "embed"

// exerciseSeed contains the built-in exercise catalog used to initialize an
// empty exercises table.
//
//go:embed tmpkin_jp.json
var exerciseSeed []byte
