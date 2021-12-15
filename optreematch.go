package yarex

import (
	"strings"
	"unicode/utf8"
	"unsafe"
)

type opExecer struct {
	op OpTree
}

func (oe opExecer) exec(str string, pos int, onSuccess func(MatchContext)) bool {
	op := oe.op
	_, headOnly := op.(*OpAssertBegin)
	if headOnly && pos != 0 {
		return false
	}
	minReq := op.minimumReq()
	if minReq > len(str)-pos {
		return false
	}
	stack := make([]opStackFrame, initialStackSize, initialStackSize)
	getter := func() []opStackFrame { return stack }
	setter := func(s []opStackFrame) { stack = s }
	ctx0 := makeOpMatchContext(&str, &getter, &setter)
	if opTreeExec(op, ctx0.Push(ContextKey{'c', 0}, 0), pos, onSuccess) {
		return true
	}
	if headOnly {
		return false
	}
	for i := pos + 1; minReq <= len(str)-i; i++ {
		if opTreeExec(op, ctx0.Push(ContextKey{'c', 0}, i), i, onSuccess) {
			return true
		}
	}
	return false
}

func opTreeExec(next OpTree, ctx MatchContext, p int, onSuccess func(MatchContext)) bool {
	str := *(*string)(unsafe.Pointer(ctx.Str))
	for {
		switch op := next.(type) {
		case OpSuccess:
			ctx := ctx.Push(ContextKey{'c', 0}, p)
			onSuccess(ctx)
			return true
		case *OpStr:
			if len(str)-p < op.minReq {
				return false
			}
			for i := 0; i < len(op.str); i++ {
				if str[p+i] != op.str[i] {
					return false
				}
			}
			next = op.follower
			p += len(op.str)
		case *OpAlt:
			if opTreeExec(op.follower, ctx, p, onSuccess) {
				return true
			}
			next = op.alt
		case *OpRepeat:
			prev := ctx.FindVal(op.key)
			if prev == p { // This means zero-width matching occurs.
				next = op.alt // So, terminate repeating.
				continue
			}
			ctx2 := ctx.Push(op.key, p)
			if opTreeExec(op.follower, ctx2, p, onSuccess) {
				return true
			}
			next = op.alt
		case *OpClass:
			if len(str)-p < op.minReq {
				return false
			}
			r, size := utf8.DecodeRuneInString(str[p:])
			if size == 0 || r == utf8.RuneError {
				return false
			}
			if !op.cls.Contains(r) {
				return false
			}
			next = op.follower
			p += size
		case *OpNotNewLine:
			if len(str)-p < op.minReq {
				return false
			}
			r, size := utf8.DecodeRuneInString(str[p:])
			if size == 0 || r == utf8.RuneError {
				return false
			}
			if r == '\n' {
				return false
			}
			next = op.follower
			p += size
		case *OpCaptureStart:
			return opTreeExec(op.follower, ctx.Push(op.key, p), p, onSuccess)
		case *OpCaptureEnd:
			return opTreeExec(op.follower, ctx.Push(op.key, p), p, onSuccess)
		case *OpBackRef:
			s, ok := ctx.GetCaptured(op.key)
			if !ok || !strings.HasPrefix(str[p:], s) {
				return false
			}
			next = op.follower
			p += len(s)
		case *OpAssertBegin:
			if p != 0 {
				return false
			}
			next = op.follower
		case *OpAssertEnd:
			if p != len(str) {
				return false
			}
			next = op.follower
		}
	}
}
