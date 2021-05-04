package main

import (
	"go/ast"
	"go/token"
)

func newComments(lines ...string) *ast.CommentGroup {
	if len(lines) == 0 {
		return nil
	}

	var g ast.CommentGroup
	for _, line := range lines {
		g.List = append(g.List, &ast.Comment{Text: "// " + line + "\n"})
	}
	return &g
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

// TODO: test case with funcFrom/FuncTo
func newAssignStmt(left ast.Expr, right ast.Expr, funcName string) *ast.AssignStmt {
	if funcName != "" {
		right = &ast.CallExpr{
			Fun:  &ast.Ident{Name: funcName},
			Args: []ast.Expr{right},
		}
	}

	// <left> := <right>
	// <left> := <funcName>(<right>)
	return &ast.AssignStmt{
		Lhs: []ast.Expr{left},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{right},
	}
}
