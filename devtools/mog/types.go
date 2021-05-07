package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"path"
)

// typeToExpr converts a go/types representation of a type into a go/ast
// representation of a type.
//
// If element is true it restricts the allowed types. Mean to represent the
// types allowed to be the target of a Pointer, or the elements of a slice or
// map.
//
// Returns a nil expression if the type is not supported.
func typeToExpr(t types.Type, imports *imports, element bool) (x ast.Expr) {
	defer func() {
		prefix := ""
		if element {
			prefix = "ELEM-"
		}
		fmt.Printf("%sTYPE-TO-EXPR: [%T :: %+v] => [%T :: %+v]\n",
			prefix,
			t, t, x, x,
		)
	}()

	switch x := t.(type) {
	case *types.Basic:
		return &ast.Ident{Name: x.Name()}

	case *types.Named:
		targetTypeName := x.Obj()

		pkgPath := targetTypeName.Pkg().Path()
		if imports != nil {
			pkgPath = imports.AliasFor(pkgPath)
		}

		pkg := path.Base(pkgPath)
		if pkg == "" || pkg == "." { // package-scoped
			return &ast.Ident{Name: targetTypeName.Name()}
		}

		sel := &ast.SelectorExpr{
			X:   &ast.Ident{Name: pkg},
			Sel: &ast.Ident{Name: targetTypeName.Name()},
		}

		return sel

	case *types.Pointer:
		actual := typeToExpr(x.Elem(), imports, element)
		if actual == nil {
			return nil
		}
		return &ast.StarExpr{X: actual}

	case *types.Slice:
		if element {
			return nil
		}
		actual := typeToExpr(x.Elem(), imports, element)
		if actual == nil {
			return nil
		}
		return &ast.ArrayType{Elt: actual}

	case *types.Map:
		if element {
			return nil
		}
		key := typeToExpr(x.Key(), imports, element)
		val := typeToExpr(x.Elem(), imports, element)
		if key == nil || val == nil {
			return nil
		}
		return &ast.MapType{
			Key:   key,
			Value: val,
		}

	case *types.Interface: // needed to target map[string]interface{} at all
		if element {
			return nil
		}
		return &ast.InterfaceType{}

	}

	return nil
}

// decodeType peeks into the type provided to discover after some of the
// pointer/aliasing ceremony what the ultimate unit of assignment will be
//
// Note this does not descend into Slices or Maps, and it doesn't handle ALL
// possible type assignments, just the reasonable ones.
//
// Emits only: Basic, Named[Struct], Slice, Map
func decodeType(t types.Type) (types.Type, bool) {
	switch x := t.(type) {
	case *types.Basic:
		return x, true

	case *types.Named:
		if _, ok := x.Underlying().(*types.Struct); ok {
			// We never return the struct, since we need the name.
			return x, true
		}
		return decodeType(x.Underlying())

	case *types.Pointer:
		// We only allow pointers to basic types and named types.
		switch pt := x.Elem().(type) {
		case *types.Basic:
			return pt, true
		case *types.Named:
			return decodeType(pt)
		}
	case *types.Slice:
		return x, true
	case *types.Map:
		return x, true
	}
	return nil, false
}

func debugPrintType(t types.Type) string {
	return fmt.Sprintf("[%T, %+v]", t, t)
}
