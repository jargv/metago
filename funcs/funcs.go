package funcs

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path"
)

type Package struct {
	pkgNode ast.Node
}

type Type struct {
	Name     string
	pkg      *Package
	typeNode *ast.TypeSpec
}

type Method struct {
	Name     string
	typ      *Type
	funcNode *ast.FuncDecl
}

type Field struct {
	Name string
	Type Type
	Tag  string
}

type Const struct {
	Name string
	Type Type
	// TODO: add Value string
}

func (p *Package) Types(types ...string) chan *Type {
	cap := 10
	filter := func(name string) bool {
		return true
	}

	if len(types) != 0 {
		cap = len(types)
		index := map[string]bool{}
		for _, str := range types {
			index[str] = true
		}
		filter = func(name string) bool {
			return index[name]
		}
	}

	c := make(chan *Type, cap)
	traverse := func() {
		ast.Inspect(p.pkgNode, func(node ast.Node) bool {
			switch node := node.(type) {
			case nil:
				return false
			case *ast.TypeSpec:
				if filter(node.Name.Name) {
					c <- &Type{node.Name.Name, p, node}
				}
				return false
			default:
				return true
			}
		})
	}

	go func() {
		traverse()
		close(c)
	}()

	return c
}

func (p *Package) Consts() chan *Const {
	c := make(chan *Const, 10)
	traverse := func() {
		ast.Inspect(p.pkgNode, func(node ast.Node) bool {
			switch node := node.(type) {
			case nil:
				return false

			case *ast.GenDecl:
				if node.Tok != token.CONST {
					return false
				}

				var prevType ast.Expr
				for _, spec := range node.Specs {
					if val, ok := spec.(*ast.ValueSpec); ok {
						typ := val.Type
						if typ == nil {
							typ = prevType
						}
						prevType = typ

						fset := token.NewFileSet()
						buffer := &bytes.Buffer{}
						err := printer.Fprint(buffer, fset, typ)
						if err != nil {
							panic(err)
						}
						typeName := string(buffer.Bytes())

						for _, name := range val.Names {
							c <- &Const{
								name.Name,
								Type{
									typeName,
									p,
									nil,
								},
							}
						}
					}
				}
				return false
			default:
				return true
			}
		})
	}

	go func() {
		traverse()
		close(c)
	}()

	return c
}

func (t *Type) Methods() chan *Method {
	c := make(chan *Method, 10)
	traverse := func() {
		ast.Inspect(t.pkg.pkgNode, func(node ast.Node) bool {
			switch node := node.(type) {
			case nil:
				return false
			case *ast.FuncDecl:
				// this is not a method
				if node.Recv == nil {
					return false
				}

				// this may be take a pointer reciever
				rec := node.Recv.List[0].Type
				if star, ok := rec.(*ast.StarExpr); ok {
					rec = star.X
				}

				// this is not a method of the right target type
				if ident, ok := rec.(*ast.Ident); !ok || ident.Name != t.Name {
					return false
				} else {
					c <- &Method{
						node.Name.Name,
						t,
						node,
					}
				}

				return true
			default:
				return true
			}
		})
	}

	go func() {
		traverse()
		close(c)
	}()

	return c
}

func (t *Type) Fields() chan *Field {
	c := make(chan *Field)

	traverse := func(typeNode ast.Node) {
		ast.Inspect(typeNode, func(node ast.Node) bool {
			switch node := node.(type) {
			case nil:
				return false
			case *ast.Field:
				fset := token.NewFileSet()
				buffer := &bytes.Buffer{}
				err := printer.Fprint(buffer, fset, node.Type)
				if err != nil {
					panic(err)
				}
				typ := string(buffer.Bytes())
				tag := ""
				if node.Tag != nil {
					tag = node.Tag.Value
				}
				for _, ident := range node.Names {
					name := ident.Name
					c <- &Field{
						name,
						Type{typ, t.pkg, nil},
						tag,
					}
				}
				return false
			default:
				return true
			}
		})
	}

	go func() {
		typeNode := t.typeNode
		if typeNode == nil {
			typeNode = (<-t.pkg.Types(t.Name)).typeNode
		}
		traverse(typeNode)
		close(c)
	}()

	return c
}

func newPackage(pkgName, pkgPath string) (*Package, error) {
	fset := token.NewFileSet()

	// TODO: make this work with multiple go paths
	gopath := os.Getenv("GOPATH")

	pkgs, err := parser.ParseDir(fset, path.Join(gopath, "src", pkgPath), nil, 0)
	if err != nil {
		return nil, err
	}

	pkgNode := pkgs[pkgName]
	if pkgNode == nil {
		return nil, fmt.Errorf("couldn't get package %s at path %s", pkgName, pkgPath)
	}

	return &Package{pkgNode}, nil
}

func PackageFunc(pkgName string) (*Package, error) {
	pkg, err := importer.Default().Import(pkgName)
	if err != nil {
		return nil, err
	}

	p, err := newPackage(pkg.Name(), pkg.Path())
	if err != nil {
		return nil, err
	}

	return p, nil
}
