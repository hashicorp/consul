package main

import (
	"fmt"
	"go/types"
)

// mappingOp is implemented by all general types of mapping operations between
// types.
type mappingOp interface {
	isMappingOp()
	fmt.Stringer
}

type assignMappingOp struct {
	// Left is the original type of the LHS of the assignment.
	Left types.Type

	// Right is the original type of the RHS of the assignment.
	Right types.Type

	// Direct implies that no conversion is needed and direct assignment should
	// occur.
	Direct bool
}

var _ mappingOp = (*assignMappingOp)(nil)

func (o *assignMappingOp) isMappingOp() {}
func (o *assignMappingOp) String() string {
	s := fmt.Sprintf("%s := %s", debugPrintType(o.Left), debugPrintType(o.Right))
	if o.Direct {
		s += " (direct)"
	}
	return s
}

type sliceMappingOp struct {
	Left, Right         types.Type // both slice-ish
	LeftElem, RightElem types.Type
	ElemOp              *assignMappingOp
}

var _ mappingOp = (*sliceMappingOp)(nil)

func (o *sliceMappingOp) isMappingOp() {}
func (o *sliceMappingOp) String() string {
	return fmt.Sprintf("%s[%s] := %s[%s] <op: %s>",
		debugPrintType(o.Left),
		debugPrintType(o.LeftElem),
		debugPrintType(o.Right),
		debugPrintType(o.RightElem),
		o.ElemOp,
	)
}

type mapMappingOp struct {
	Left, Right               types.Type // both map-ish
	LeftKeyElem, RightKeyElem types.Type
	LeftElem, RightElem       types.Type
	ElemOp                    *assignMappingOp
}

var _ mappingOp = (*mapMappingOp)(nil)

func (o *mapMappingOp) isMappingOp() {}
func (o *mapMappingOp) String() string {
	return fmt.Sprintf("%s<%s,%s> := %s<%s,%s> <op: %s>",
		debugPrintType(o.Left),
		debugPrintType(o.LeftKeyElem),
		debugPrintType(o.LeftElem),
		debugPrintType(o.Right),
		debugPrintType(o.RightKeyElem),
		debugPrintType(o.RightElem),
		o.ElemOp,
	)
}

func computeMappingOperation(leftType, rightType types.Type) (mo mappingOp, xok bool) {
	if types.AssignableTo(rightType, leftType) {
		if !types.AssignableTo(leftType, rightType) {
			return nil, false // needs to be bidirectional
		}
		return &assignMappingOp{
			Left:   leftType,
			Right:  rightType,
			Direct: true,
		}, true
	}

	leftTypeDecode, leftOk := decodeType(leftType)
	rightTypeDecode, rightOk := decodeType(rightType)
	if !leftOk || !rightOk {
		return nil, false
	}

	switch left := leftTypeDecode.(type) {
	case *types.Basic:
		_, ok := rightTypeDecode.(*types.Basic)
		if !ok {
			return nil, false
		}
		return &assignMappingOp{
			Left:  leftType,
			Right: rightType,
		}, true
	case *types.Named:
		_, ok := rightTypeDecode.(*types.Named)
		if !ok {
			return nil, false
		}
		return &assignMappingOp{
			Left:  leftType,
			Right: rightType,
		}, true
	case *types.Slice:
		right, ok := rightTypeDecode.(*types.Slice)
		if !ok {
			return nil, false
		}

		rawOp, ok := computeMappingOperation(left.Elem(), right.Elem())
		if !ok {
			return nil, false
		}

		op, ok := rawOp.(*assignMappingOp)
		if !ok {
			return nil, false
		}

		return &sliceMappingOp{
			Left:      leftType,
			LeftElem:  left.Elem(),
			Right:     rightType,
			RightElem: right.Elem(),
			ElemOp:    op,
		}, true
	case *types.Map:
		right, ok := rightTypeDecode.(*types.Map)
		if !ok {
			return nil, false
		}

		_, ok = computeMappingOperation(left.Key(), right.Key())
		if !ok {
			return nil, false
		}

		rawOp, ok := computeMappingOperation(left.Elem(), right.Elem())
		if !ok {
			return nil, false
		}

		op, ok := rawOp.(*assignMappingOp)
		if !ok {
			return nil, false
		}

		return &mapMappingOp{
			Left:         leftType,
			LeftKeyElem:  left.Key(),
			LeftElem:     left.Elem(),
			Right:        rightType,
			RightKeyElem: right.Key(),
			RightElem:    right.Elem(),
			ElemOp:       op,
		}, true
	}

	return nil, false
}
