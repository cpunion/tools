// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssa_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func TestExplicitPointerMethodReceiverDeref(t *testing.T) {
	for _, test := range []struct {
		name, parameter, body string
		wantDeref             bool
	}{
		{"call", "*T", "(*p).Nop(0)", true},
		{"parenthesized", "*T", "((*(p))).Nop(0)", true},
		{"defer", "*T", "defer (*p).Nop(0)", true},
		{"go", "*T", "go (*p).Nop(0)", true},
		{"method-value", "*T", "f := (*p).Nop; f(0)", true},
		{"argument-order", "*T", "(*p).Nop(side())", true},
		{"generic", "*Generic[int]", "(*p).Nop(0)", true},
		{"implicit", "*T", "p.Nop(0)", false},
		{"implicit-method-value", "*T", "f := p.Nop; f(0)", false},
		{"field-assignment", "*T", "(*p).x = side()", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			src := `package p
type T struct { x int }
type Generic[V any] struct { x V }
var touched int
func (*T) Nop(int) { touched++ }
func (*Generic[V]) Nop(int) { touched++ }
func side() int { touched++; return 1 }
func Root(p ` + test.parameter + `) { ` + test.body + ` }
`
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "p.go", src, 0)
			if err != nil {
				t.Fatal(err)
			}
			pkg, _, err := ssautil.BuildPackage(&types.Config{}, fset, types.NewPackage("p", "p"), []*ast.File{file}, ssa.SanityCheckFunctions|ssa.InstantiateGenerics)
			if err != nil {
				t.Fatal(err)
			}
			fn := pkg.Func("Root")
			loads, firstLoad, firstEffect, ordinal := 0, -1, -1, 0
			for _, block := range fn.Blocks {
				for _, instruction := range block.Instrs {
					if load, ok := instruction.(*ssa.UnOp); ok && load.Op == token.MUL && load.X == fn.Params[0] {
						loads++
						if firstLoad < 0 {
							firstLoad = ordinal
						}
						if !load.Pos().IsValid() {
							t.Error("explicit dereference lost its source position")
						}
					}
					switch instruction.(type) {
					case ssa.CallInstruction, *ssa.MakeClosure:
						if firstEffect < 0 {
							firstEffect = ordinal
						}
					}
					ordinal++
				}
			}
			wantLoads := 0
			if test.wantDeref {
				wantLoads = 1
			}
			if loads != wantLoads {
				t.Fatalf("receiver loads = %d, want %d", loads, wantLoads)
			}
			if test.wantDeref && (firstEffect < 0 || firstLoad >= firstEffect) {
				t.Fatal("receiver dereference does not precede method selection and argument evaluation")
			}
		})
	}
}
