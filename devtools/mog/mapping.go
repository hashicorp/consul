package main

import (
	"fmt"
	"go/types"
)

// assignmentKind is implemented by all general types of mapping operations
// between types.
type assignmentKind interface {
	isAssignmentKind()
	fmt.Stringer
}

// singleAssignmentKind is a mapping operation between two fields that
// ultimately are:
//
//  - basic
//  - named structs
//  - pointers to either of the above
type singleAssignmentKind struct {
	// Left is the original type of the LHS of the assignment.
	Left types.Type

	// Right is the original type of the RHS of the assignment.
	Right types.Type

	// Direct implies that no conversion is needed and direct assignment should
	// occur.
	Direct bool
}

var _ assignmentKind = (*singleAssignmentKind)(nil)

func (o *singleAssignmentKind) isAssignmentKind() {}
func (o *singleAssignmentKind) String() string {
	s := fmt.Sprintf("%s := %s", debugPrintType(o.Left), debugPrintType(o.Right))
	if o.Direct {
		s += " (direct)"
	}
	return s
}

// sliceAssignmentKind is a mapping operation between two fields that are
// slice-ish and have elements that would satisfy singleAssignmentKind
type sliceAssignmentKind struct {
	// Left is the original type of the LHS of the assignment. Should be
	// slice-ish.
	Left types.Type

	// Right is the original type of the RHS of the assignment. Should be
	// slice-ish.
	Right types.Type

	// LeftElem is the original type of the elements of the LHS of the
	// assignment.
	LeftElem types.Type

	// RightElem is the original type of the elements of the LHS of the
	// assignment.
	RightElem types.Type

	// ElemDirect implies that no conversion is needed and direct assignment
	// should occur for elements of the slice.
	ElemDirect bool
}

var _ assignmentKind = (*sliceAssignmentKind)(nil)

func (o *sliceAssignmentKind) isAssignmentKind() {}
func (o *sliceAssignmentKind) String() string {
	s := fmt.Sprintf("%s[%s] := %s[%s]",
		debugPrintType(o.Left),
		debugPrintType(o.LeftElem),
		debugPrintType(o.Right),
		debugPrintType(o.RightElem),
	)
	if o.ElemDirect {
		s += " (direct)"
	}
	return s
}

// mapAssignmentKind is a mapping operation between two fields that are map-ish
// and have value elements that would satisfy singleAssignmentKind and key
// elements that are directly assignable.
type mapAssignmentKind struct {
	// Left is the original type of the LHS of the assignment. Should be
	// map-ish.
	Left types.Type

	// Right is the original type of the RHS of the assignment. Should be
	// map-ish.
	Right types.Type

	// LeftKey is the original type of the keys of the LHS of the
	// assignment.
	LeftKey types.Type

	// RightKey is the original type of the keys of the LHS of the
	// assignment.
	RightKey types.Type

	// LeftElem is the original type of the elements of the LHS of the
	// assignment.
	LeftElem types.Type

	// RightElem is the original type of the elements of the LHS of the
	// assignment.
	RightElem types.Type

	// ElemDirect implies that no conversion is needed and direct assignment
	// should occur for elements of the slice.
	ElemDirect bool
}

var _ assignmentKind = (*mapAssignmentKind)(nil)

func (o *mapAssignmentKind) isAssignmentKind() {}
func (o *mapAssignmentKind) String() string {
	s := fmt.Sprintf("%s<%s,%s> := %s<%s,%s>",
		debugPrintType(o.Left),
		debugPrintType(o.LeftKey),
		debugPrintType(o.LeftElem),
		debugPrintType(o.Right),
		debugPrintType(o.RightKey),
		debugPrintType(o.RightElem),
	)
	if o.ElemDirect {
		s += " (direct)"
	}
	return s
}

// computeAssignment attempts to determine how to assign something of the
// rightType to something of the leftType.
//
// If this is not possible, or not currently supported (nil, false) is
// returned.
func computeAssignment(leftType, rightType types.Type) (assignmentKind, bool) {
	// First check if the types are naturally directly assignable. Only allow
	// type pairs that are symmetrically assignable for simplicity.
	if types.AssignableTo(rightType, leftType) {
		if !types.AssignableTo(leftType, rightType) {
			return nil, false
		}
		return &singleAssignmentKind{
			Left:   leftType,
			Right:  rightType,
			Direct: true,
		}, true
	}

	// We don't really care about type aliases or pointerness here, so peel
	// those off first to simplify the space we have to consider below.
	leftTypeDecode, leftOk := decodeType(leftType)
	rightTypeDecode, rightOk := decodeType(rightType)
	if !leftOk || !rightOk {
		return nil, false
	}

	switch left := leftTypeDecode.(type) {
	case *types.Basic:
		// basic can only assign to basic
		_, ok := rightTypeDecode.(*types.Basic)
		if !ok {
			return nil, false
		}
		return &singleAssignmentKind{
			Left:   leftType,
			Right:  rightType,
			Direct: true,
		}, true
	case *types.Named:
		// named can only assign to named
		_, ok := rightTypeDecode.(*types.Named)
		if !ok {
			return nil, false
		}
		return &singleAssignmentKind{
			Left:  leftType,
			Right: rightType,
		}, true
	case *types.Slice:
		// slices can only assign to slices
		right, ok := rightTypeDecode.(*types.Slice)
		if !ok {
			return nil, false
		}

		// the elements have to be assignable
		rawOp, ok := computeAssignment(left.Elem(), right.Elem())
		if !ok {
			return nil, false
		}

		op, ok := rawOp.(*singleAssignmentKind)
		if !ok {
			return nil, false
		}

		return &sliceAssignmentKind{
			Left:       leftType,
			LeftElem:   left.Elem(),
			Right:      rightType,
			RightElem:  right.Elem(),
			ElemDirect: op.Direct,
		}, true
	case *types.Map:
		right, ok := rightTypeDecode.(*types.Map)
		if !ok {
			return nil, false
		}

		rawKeyOp, ok := computeAssignment(left.Key(), right.Key())
		if !ok {
			return nil, false
		}

		// the map keys have to be directly assignable
		keyOp, ok := rawKeyOp.(*singleAssignmentKind)
		if !ok {
			return nil, false
		}
		if !keyOp.Direct {
			return nil, false
		}

		// the map values have to be assignable
		rawOp, ok := computeAssignment(left.Elem(), right.Elem())
		if !ok {
			return nil, false
		}

		op, ok := rawOp.(*singleAssignmentKind)
		if !ok {
			return nil, false
		}

		return &mapAssignmentKind{
			Left:       leftType,
			LeftKey:    left.Key(),
			LeftElem:   left.Elem(),
			Right:      rightType,
			RightKey:   right.Key(),
			RightElem:  right.Elem(),
			ElemDirect: op.Direct,
		}, true
	}

	return nil, false
}
