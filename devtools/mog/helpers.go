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

// Returns go/ast type for something concrete. This should be suitable for
// stack-allocating. If the input is a pointer, this is the type that the
// pointer points to.
//
// The boolean return value is true if the input was a pointer.
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

func newAssignStmt(
	sourceField fieldConfig,
	left ast.Expr,
	leftPtr bool,
	leftType ast.Expr,
	right ast.Expr,
	rightPtr bool,
	direction Direction,
) ast.Stmt {
	if userFuncName := sourceField.UserFuncName(direction); userFuncName != "" {
		// No special handling for pointers here if someone used the mog
		// annotations themselves. The assumption is the user knows what
		// they're doing.

		// <left> = <funcName>(<right>)
		return &ast.AssignStmt{
			Lhs: []ast.Expr{left},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{
				&ast.CallExpr{
					Fun:  &ast.Ident{Name: userFuncName},
					Args: []ast.Expr{right},
				},
			},
		}
	}

	convertFuncName := sourceField.ConvertFuncName(direction)
	if convertFuncName == "" {
		// <left> = <right>
		return astAssign(left, right)
	}

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
