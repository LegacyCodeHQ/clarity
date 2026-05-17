package zig

//#cgo CFLAGS: -std=c11 -fPIC
//#include "tree_sitter/parser.h"
//const TSLanguage *tree_sitter_zig(void);
import "C"

import (
	"unsafe"

	sitter "github.com/smacker/go-tree-sitter"
)

func GetLanguage() *sitter.Language {
	ptr := unsafe.Pointer(C.tree_sitter_zig())
	return sitter.NewLanguage(ptr)
}
