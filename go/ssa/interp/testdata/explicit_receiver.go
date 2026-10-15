// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

type T struct{ x int }

var touched, arguments, receivers int

// This method deliberately accepts nil: any caller-side fault must precede it.
func (*T) Nop(int)  { touched++ }
func argument() int { arguments++; return 1 }
func receiver() *T  { receivers++; return nil }

func mustPanic(f func()) {
	before := touched
	panicked := true
	func() {
		defer func() { recover() }()
		f()
		panicked = false
	}()
	if !panicked {
		panic("explicit receiver dereference did not panic")
	}
	if touched != before {
		panic("nil-safe method ran before caller-side panic")
	}
}

func main() {
	var p *T
	mustPanic(func() { (*p).Nop(0) })
	mustPanic(func() { (*(p)).Nop(0) })
	mustPanic(func() { defer (*p).Nop(0) })
	mustPanic(func() { go (*p).Nop(0) })
	mustPanic(func() { f := (*p).Nop; _ = f })
	mustPanic(func() { (*p).Nop(argument()) })
	// Do not assume an ordering between the nil fault and argument calls.
	arguments = 0
	mustPanic(func() { (*receiver()).Nop(0) })
	if receivers != 1 {
		panic("receiver expression was not evaluated exactly once")
	}

	// Implicit address-taking for field assignment must retain its existing
	// delayed store fault; the RHS still executes before the nil assignment.
	mustPanic(func() { (*p).x = argument() })
	if arguments != 1 {
		panic("field-assignment RHS did not run before fault")
	}

	p.Nop(0)
	f := p.Nop
	f(0)
	valid := new(T)
	(*valid).Nop(0)
	if touched != 3 {
		panic("valid or implicit receiver calls changed")
	}
}
