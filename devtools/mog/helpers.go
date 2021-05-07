package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"path"
)

func stripCurrentPackagePrefix(expr ast.Expr, currentPkg string) ast.Expr {
	if expr == nil {
		return nil
	}
	se, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return expr
	}
	xIdent, ok := se.X.(*ast.Ident)
	if !ok {
		return expr
	}
	if xIdent.Name != currentPkg {
		return expr
	}
	return se.Sel
}

// astTypeFromTypesType converts a go/types representation of a type into a
// go/ast representation of a type.
//
// Works on 3 kinds: basic types, named types, and pointers to either of those.
//
// Returns an ast.Expr for the basic/named type, and a boolean indicating if it
// was originally a pointer.
//
// Returns a nil expression if the type is not supported.
func astTypeFromTypesType(imports *imports, typ types.Type, pointerAllowed bool) (ast.Expr, bool) {
	switch x := typ.(type) {
	case *types.Basic:
		return &ast.Ident{Name: x.Name()}, false
	case *types.Named:
		targetTypeName := x.Obj()

		pkgPath := targetTypeName.Pkg().Path()
		if imports != nil {
			pkgPath = imports.AliasFor(pkgPath)
		}

		return &ast.SelectorExpr{
			X:   &ast.Ident{Name: path.Base(pkgPath)},
			Sel: &ast.Ident{Name: targetTypeName.Name()},
		}, false

	case *types.Pointer:
		if !pointerAllowed {
			return nil, false
		}
		expr, _ := astTypeFromTypesType(imports, x.Elem(), false)
		return expr, true
	}
	return nil, false
}

func astAssign(left, right ast.Expr) ast.Stmt {
	return &ast.AssignStmt{
		Lhs: []ast.Expr{left},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{right},
	}
}

func astDeclare(varName string, varType ast.Expr) ast.Stmt {
	return &ast.DeclStmt{Decl: &ast.GenDecl{
		Tok: token.VAR,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names: []*ast.Ident{{Name: varName}},
				Type:  varType,
			},
		},
	}}
}

func astIsNotNil(expr ast.Expr) ast.Expr {
	return &ast.BinaryExpr{
		X:  expr,
		Op: token.NEQ,
		Y:  &ast.Ident{Name: "nil"},
	}
}

func astAddressOf(expr ast.Expr) ast.Expr {
	return &ast.UnaryExpr{
		Op: token.AND,
		X:  expr,
	}
}

func astCallConvertFunc(funcName string, args ...ast.Expr) ast.Stmt {
	return &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun:  &ast.Ident{Name: funcName},
			Args: args,
		},
	}
}

func newAddressOf(id string) *ast.UnaryExpr {
	// &<id>
	return &ast.UnaryExpr{
		Op: token.AND,
		X:  &ast.Ident{Name: id},
	}
}

func newPointerTo(id string) *ast.StarExpr {
	// *<id>
	return &ast.StarExpr{
		X: &ast.Ident{Name: id},
	}
}

func newIfNilReturn(cmpID string) ast.Stmt {
	// if <cmpID> == nil {
	// 	return
	// }
	return &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X:  &ast.Ident{Name: cmpID},
			Op: token.EQL,
			Y:  &ast.Ident{Name: "nil"},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{},
			},
		},
	}
}

func newIfNilReturnIdent(cmpID, retID string) ast.Stmt {
	// if <cmpID> == nil {
	// 	return <retID>
	// }
	return &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X:  &ast.Ident{Name: cmpID},
			Op: token.EQL,
			Y:  &ast.Ident{Name: "nil"},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.Ident{Name: retID},
					},
				},
			},
		},
	}
}

type astType int

const (
	astTypeUnknown     astType = iota
	astTypeConvertible         // aka struct or pointer-to-struct
	astTypeMap
	astTypeArray
)

type astTypeInfo struct {
	Type astType

	// fields for: astTypeUnknown
	UserFuncNameTo   string
	UserFuncNameFrom string

	// fields for: astTypeConvertible
	StructType          ast.Expr
	WasPointer          bool
	ConvertFuncNameTo   string
	ConvertFuncNameFrom string
}

func newAstType(sourceField fieldConfig) astTypeInfo {
	var info astTypeInfo

	if sourceField.FuncTo != "" || sourceField.FuncFrom != "" {
		info.Type = astTypeUnknown
		info.UserFuncNameTo = sourceField.FuncTo
		info.UserFuncNameFrom = sourceField.FuncFrom
		return info
	}

	switch x := sourceField.SourceExpr.(type) {
	case *ast.Ident:
		info.Type = astTypeConvertible
		info.StructType = sourceField.SourceExpr
	case *ast.StarExpr:
		if y, ok := x.X.(*ast.Ident); ok {
			info.Type = astTypeConvertible
			info.StructType = y
			info.WasPointer = true
		}
	case *ast.MapType:
		info.Type = astTypeMap
	case *ast.ArrayType:
		info.Type = astTypeArray
	}

	if info.Type == astTypeConvertible {
		funcTo, funcFrom := sourceField.ConvertFuncs()
		if funcTo != "" && funcFrom != "" {
			info.ConvertFuncNameTo = funcTo
			info.ConvertFuncNameFrom = funcFrom
		} else {
			info.Type = astTypeUnknown
		}
	}

	return info
}

func newAssignStmtConvertible(
	left ast.Expr,
	leftPtr bool,
	leftType ast.Expr,
	right ast.Expr,
	rightPtr bool,
	convertFuncName string,
) ast.Stmt {
	switch {
	case !leftPtr && !rightPtr:
		// Value to Value
		//
		// <convertFuncName>(&<right>, &<left>)
		return astCallConvertFunc(convertFuncName,
			astAddressOf(right),
			astAddressOf(left))
	case !leftPtr && rightPtr:
		// Pointer to Value
		//
		// if <right> != nil {
		// 	<convertFuncName>(<right>, &<left>)
		// }
		return &ast.IfStmt{
			Cond: astIsNotNil(right),
			Body: &ast.BlockStmt{List: []ast.Stmt{
				astCallConvertFunc(convertFuncName,
					right,
					astAddressOf(left)),
			}},
		}
	case leftPtr && !rightPtr:
		if leftType == nil {
			panic("unknown type for LHS")
		}
		// Value to Pointer
		// var <varTarget> <typeTarget>
		// <convertFuncName>(&<right>, &<varTarget>)
		// <left> = &<varTarget>
		return &ast.BlockStmt{List: []ast.Stmt{
			astDeclare(varNamePlaceholder, leftType),
			astCallConvertFunc(convertFuncName,
				astAddressOf(right),
				astAddressOf(&ast.Ident{Name: varNamePlaceholder})),
			astAssign(left, newAddressOf(varNamePlaceholder)),
		}}
	case leftPtr && rightPtr:
		if leftType == nil {
			panic("unknown type for LHS")
		}
		// Pointer to Pointer
		return &ast.BlockStmt{List: []ast.Stmt{
			&ast.IfStmt{
				Cond: astIsNotNil(right),
				Body: &ast.BlockStmt{List: []ast.Stmt{
					astDeclare(varNamePlaceholder, leftType),
					astCallConvertFunc(convertFuncName,
						right,
						astAddressOf(&ast.Ident{Name: varNamePlaceholder})),
					astAssign(left, newAddressOf(varNamePlaceholder)),
				}},
			},
		}}
	default:
		panic("impossible")
	}
}

func newAssignStmtUnknown(
	left ast.Expr,
	right ast.Expr,
	userFuncName string,
) ast.Stmt {
	if userFuncName != "" {
		// No special handling for pointers here if someone used the mog
		// annotations themselves. The assumption is the user knows what
		// they're doing.

		// <left> = <funcName>(<right>)
		return astAssign(left, &ast.CallExpr{
			Fun:  &ast.Ident{Name: userFuncName},
			Args: []ast.Expr{right},
		})
	}
	// <left> = <right>
	return astAssign(left, right)
}
