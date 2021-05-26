package main

import (
	"fmt"
	"go/ast"
	"go/token"
)

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
		Body: &ast.BlockStmt{List: []ast.Stmt{
			&ast.ReturnStmt{},
		}},
	}
}

// TODO: do the pointer stuff with go/types instead like everything else now?
func newAssignStmtConvertible(
	left ast.Expr,
	leftType ast.Expr,
	right ast.Expr,
	rightType ast.Expr,
	convertFuncName string,
) ast.Stmt {
	leftPtrType, leftPtr := leftType.(*ast.StarExpr)
	_, rightPtr := rightType.(*ast.StarExpr)

	leftRealType := leftType
	if leftPtr {
		leftRealType = leftPtrType.X
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
		// Value to Pointer
		// var <varTarget> <typeTarget>
		// <convertFuncName>(&<right>, &<varTarget>)
		// <left> = &<varTarget>
		return &ast.BlockStmt{List: []ast.Stmt{
			astDeclare(varNamePlaceholder, leftRealType),
			astCallConvertFunc(convertFuncName,
				astAddressOf(right),
				astAddressOf(&ast.Ident{Name: varNamePlaceholder})),
			astAssign(left, newAddressOf(varNamePlaceholder)),
		}}
	case leftPtr && rightPtr:
		// Pointer to Pointer
		return &ast.IfStmt{
			Cond: astIsNotNil(right),
			Body: &ast.BlockStmt{List: []ast.Stmt{
				astDeclare(varNamePlaceholder, leftRealType),
				astCallConvertFunc(convertFuncName,
					right,
					astAddressOf(&ast.Ident{Name: varNamePlaceholder})),
				astAssign(left, newAddressOf(varNamePlaceholder)),
			}},
		}
	default:
		panic("impossible")
	}
}

func newAssignStmtSlice(
	left ast.Expr,
	leftType ast.Expr,
	leftElemType ast.Expr,
	right ast.Expr,
	rightElemType ast.Expr,
	convertFuncName string,
	direct bool,
) ast.Stmt {
	return &ast.BlockStmt{List: []ast.Stmt{
		// <left> = make(<leftType>, len(<right>))
		&ast.AssignStmt{
			Tok: token.ASSIGN,
			Lhs: []ast.Expr{left},
			Rhs: []ast.Expr{
				&ast.CallExpr{
					Fun: &ast.Ident{Name: "make"},
					Args: []ast.Expr{
						leftType,
						&ast.CallExpr{
							Fun:  &ast.Ident{Name: "len"},
							Args: []ast.Expr{right},
						},
					},
				},
			},
		},
		// for i, item := range <right> {
		// 	<left>[i] ??assign?? <right>[i]
		// }
		&ast.RangeStmt{
			Key: &ast.Ident{Name: "i"},
			Tok: token.DEFINE,
			X:   right,
			Body: &ast.BlockStmt{List: []ast.Stmt{
				newAssignStmt(
					&ast.IndexExpr{
						X:     left,
						Index: &ast.Ident{Name: "i"},
					},
					leftElemType,
					&ast.IndexExpr{
						X:     right,
						Index: &ast.Ident{Name: "i"},
					},
					rightElemType,
					convertFuncName,
					direct,
				),
			}},
		},
	}}
}

func newAssignStmtMap(
	left ast.Expr,
	leftType ast.Expr,
	leftElemType ast.Expr,
	right ast.Expr,
	rightElemType ast.Expr,
	convertFuncName string,
	direct bool,
) ast.Stmt {
	return &ast.BlockStmt{List: []ast.Stmt{
		// <left> = make(<leftType>, len(<right>))
		&ast.AssignStmt{
			Tok: token.ASSIGN,
			Lhs: []ast.Expr{left},
			Rhs: []ast.Expr{
				&ast.CallExpr{
					Fun: &ast.Ident{Name: "make"},
					Args: []ast.Expr{
						leftType,
						&ast.CallExpr{
							Fun: &ast.Ident{Name: "len"},
							Args: []ast.Expr{
								right,
							},
						},
					},
				},
			},
		},
		// for k, v := range <right> {
		// 	var x <left-value>
		// 	x ??assign?? v
		// 	<left>[k] = x
		// }
		&ast.RangeStmt{
			Key:   &ast.Ident{Name: "k"},
			Value: &ast.Ident{Name: "v"},
			Tok:   token.DEFINE,
			X:     right,
			Body: &ast.BlockStmt{List: []ast.Stmt{
				astDeclare(varNameElemPlaceholder, leftElemType),
				newAssignStmt(
					&ast.Ident{Name: varNameElemPlaceholder},
					leftElemType,
					&ast.Ident{Name: "v"},
					rightElemType,
					convertFuncName,
					direct,
				),
				newAssignStmtStructsAndPointers(
					&ast.IndexExpr{
						X:     left,
						Index: &ast.Ident{Name: "k"},
					},
					leftElemType,
					&ast.Ident{Name: varNameElemPlaceholder},
					leftElemType,
				),
			}},
		},
	}}
}

func newAssignStmt(
	left ast.Expr,
	leftType ast.Expr,
	right ast.Expr,
	rightType ast.Expr,
	convertFuncName string,
	direct bool,
) ast.Stmt {
	if convertFuncName != "" && !direct {
		return newAssignStmtConvertible(
			left,
			leftType,
			right,
			rightType,
			convertFuncName,
		)
	}
	return newAssignStmtStructsAndPointers(
		left,
		leftType,
		right,
		rightType,
	)
}

func newAssignStmtUserFunc(
	left ast.Expr,
	right ast.Expr,
	userFuncName string,
) ast.Stmt {
	// No special handling for pointers here if someone used the mog
	// annotations themselves. The assumption is the user knows what
	// they're doing.

	// <left> = <funcName>(<right>)
	return astAssign(left, &ast.CallExpr{
		Fun:  &ast.Ident{Name: userFuncName},
		Args: []ast.Expr{right},
	})
}

// TODO: do the pointer stuff with go/types instead like everything else now?
func newAssignStmtStructsAndPointers(
	left ast.Expr,
	leftType ast.Expr,
	right ast.Expr,
	rightType ast.Expr,
) ast.Stmt {
	_, leftPtr := leftType.(*ast.StarExpr)
	_, rightPtr := rightType.(*ast.StarExpr)

	switch {
	case !leftPtr && !rightPtr:
		// Value to Value
		//
		// <left> = <right>
		return astAssign(left, right)
	case !leftPtr && rightPtr:
		// Pointer to Value
		//
		// <left> = *<right>
		return astAssign(left, &ast.StarExpr{X: right})
	case leftPtr && !rightPtr:
		// Value to Pointer
		//
		// <left> = &<right>
		return astAssign(left, astAddressOf(right))
	case leftPtr && rightPtr:
		// Pointer to Pointer
		//
		// <left> = <right>
		return astAssign(left, right)
	default:
		panic("impossible")
	}
}

// printTypeExpr is useful when debugging changes to mog itself
func printTypeExpr(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name

	case *ast.StarExpr:
		return "*" + printTypeExpr(x.X)

	case *ast.ArrayType:
		return "[]" + printTypeExpr(x.Elt)

	case *ast.MapType:
		return "map[" + printTypeExpr(x.Key) + "]" + printTypeExpr(x.Value)

	case *ast.SelectorExpr:
		return printTypeExpr(x.X) + "." + printTypeExpr(x.Sel)

	case *ast.InterfaceType:
		return "interface{}"

	}
	return fmt.Sprintf("<UNKNOWN: %T :: %+v>", expr, expr)
}
