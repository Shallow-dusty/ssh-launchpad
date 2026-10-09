// bridge-types derives TypeScript wire models and Wails method signatures from
// the Go source. Output is a local build artifact, not hand-maintained source.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/Shallow-dusty/ssh-launchpad/internal/launchpad"
)

type registry struct {
	types   map[string]ast.Expr
	enums   map[string][]string
	methods []*ast.FuncDecl
	output  map[string]string
}

func main() {
	root := flag.String("root", ".", "repository root")
	output := flag.String("output", "build/contracts/wire-types.ts", "output relative to repository root")
	flag.Parse()
	text, err := generate(*root)
	if err == nil {
		path := filepath.Join(*root, *output)
		err = os.MkdirAll(filepath.Dir(path), 0o755)
		if err == nil {
			err = os.WriteFile(path, []byte(text), 0o644)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(root string) (string, error) {
	r := &registry{types: map[string]ast.Expr{}, enums: map[string][]string{}, output: map[string]string{}}
	for _, directory := range []string{root, filepath.Join(root, "internal", "launchpad")} {
		files, err := filepath.Glob(filepath.Join(directory, "*.go"))
		if err != nil {
			return "", err
		}
		for _, path := range files {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return "", err
			}
			r.addFile(file)
		}
	}
	sort.Slice(r.methods, func(i, j int) bool { return r.methods[i].Name.Name < r.methods[j].Name.Name })
	var methods strings.Builder
	for _, method := range r.methods {
		var params []string
		for _, field := range method.Type.Params.List {
			typeName, err := r.renderType(field.Type, true)
			if err != nil {
				return "", fmt.Errorf("%s argument: %w", method.Name, err)
			}
			for _, name := range field.Names {
				params = append(params, name.Name+": "+typeName)
			}
		}
		result := "void"
		var results []*ast.Field
		if method.Type.Results != nil {
			results = method.Type.Results.List
		}
		for _, field := range results {
			if ident, ok := field.Type.(*ast.Ident); ok && ident.Name == "error" {
				continue
			}
			if result != "void" {
				return "", fmt.Errorf("%s has multiple data results", method.Name)
			}
			var err error
			result, err = r.renderType(field.Type, true)
			if err != nil {
				return "", fmt.Errorf("%s result: %w", method.Name, err)
			}
		}
		fmt.Fprintf(&methods, "  %s(%s): Promise<%s>;\n", method.Name, strings.Join(params, ", "), result)
	}
	if len(r.methods) == 0 {
		return "", fmt.Errorf("no exported App methods found under %s", root)
	}
	var output strings.Builder
	output.WriteString("// Generated from Go JSON models and App methods. Do not edit.\n\n")
	var names []string
	for name := range r.output {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		output.WriteString(r.output[name])
		output.WriteString("\n")
	}
	output.WriteString("export interface DesktopBridge {\n")
	output.WriteString(methods.String())
	output.WriteString("}\n")
	if _, usesProfile := r.output["Profile"]; usesProfile {
		defaults, err := json.MarshalIndent(defaultFields(reflect.ValueOf(launchpad.DefaultProfile())), "", "  ")
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&output, "\nexport const wireDefaultProfile = %s satisfies WireProfile;\n", defaults)
	}
	return output.String(), nil
}

// Include optional empty fields in form defaults, rather than duplicating
// their names/values in TypeScript. Go remains the source of policy defaults.
func defaultFields(value reflect.Value) any {
	if value.Kind() != reflect.Struct {
		return value.Interface()
	}
	result := map[string]any{}
	for index := 0; index < value.NumField(); index++ {
		field := value.Type().Field(index)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if field.IsExported() && name != "" && name != "-" {
			result[name] = defaultFields(value.Field(index))
		}
	}
	return result
}

func (r *registry) addFile(file *ast.File) {
	for _, declaration := range file.Decls {
		switch decl := declaration.(type) {
		case *ast.GenDecl:
			for _, specification := range decl.Specs {
				switch spec := specification.(type) {
				case *ast.TypeSpec:
					r.types[spec.Name.Name] = spec.Type
				case *ast.ValueSpec:
					if decl.Tok != token.CONST || spec.Type == nil {
						continue
					}
					ident, ok := spec.Type.(*ast.Ident)
					if !ok {
						continue
					}
					for _, value := range spec.Values {
						if literal, ok := value.(*ast.BasicLit); ok && literal.Kind == token.STRING {
							r.enums[ident.Name] = append(r.enums[ident.Name], literal.Value)
						}
					}
				}
			}
		case *ast.FuncDecl:
			if decl.Recv == nil || !decl.Name.IsExported() {
				continue
			}
			receiver := decl.Recv.List[0].Type
			if pointer, ok := receiver.(*ast.StarExpr); ok {
				receiver = pointer.X
			}
			if ident, ok := receiver.(*ast.Ident); ok && ident.Name == "App" {
				r.methods = append(r.methods, decl)
			}
		}
	}
}

func (r *registry) model(name string) (string, error) {
	wire := "Wire" + name
	if _, exists := r.output[name]; exists {
		return wire, nil
	}
	expr, exists := r.types[name]
	if !exists {
		return "", fmt.Errorf("unknown Go wire type %q", name)
	}
	r.output[name] = "" // Register before following references.
	if values := r.enums[name]; len(values) > 0 {
		sort.Strings(values)
		r.output[name] = fmt.Sprintf("export type %s = %s;\n", wire, strings.Join(values, " | "))
		return wire, nil
	}
	if object, ok := expr.(*ast.StructType); ok {
		var fields strings.Builder
		for _, field := range object.Fields.List {
			if len(field.Names) != 1 || !field.Names[0].IsExported() {
				continue
			}
			if field.Tag == nil {
				return "", fmt.Errorf("%s.%s has no JSON tag", name, field.Names[0])
			}
			tag, err := strconv.Unquote(field.Tag.Value)
			if err != nil {
				return "", err
			}
			parts := strings.Split(reflect.StructTag(tag).Get("json"), ",")
			if parts[0] == "-" {
				continue
			}
			if parts[0] == "" {
				return "", fmt.Errorf("%s.%s has no JSON name", name, field.Names[0])
			}
			optional := false
			for _, option := range parts[1:] {
				optional = optional || option == "omitempty"
			}
			value, err := r.renderType(field.Type, !optional)
			if err != nil {
				return "", fmt.Errorf("%s.%s: %w", name, field.Names[0], err)
			}
			marker := ""
			if optional {
				marker = "?"
			}
			fmt.Fprintf(&fields, "  %s%s: %s;\n", parts[0], marker, value)
		}
		r.output[name] = fmt.Sprintf("export interface %s {\n%s}\n", wire, fields.String())
	} else {
		value, err := r.renderType(expr, false)
		if err != nil {
			return "", err
		}
		r.output[name] = fmt.Sprintf("export type %s = %s;\n", wire, value)
	}
	return wire, nil
}

func (r *registry) renderType(expr ast.Expr, nullable bool) (string, error) {
	null := ""
	if nullable {
		null = " | null"
	}
	switch value := expr.(type) {
	case *ast.Ident:
		switch value.Name {
		case "string":
			return "string", nil
		case "bool":
			return "boolean", nil
		case "int", "int32", "int64", "uint", "uint32", "uint64", "float32", "float64":
			return "number", nil
		case "any":
			return "unknown", nil
		default:
			return r.model(value.Name)
		}
	case *ast.SelectorExpr:
		if owner, ok := value.X.(*ast.Ident); ok && owner.Name == "time" && value.Sel.Name == "Time" {
			return "string", nil
		}
		return r.model(value.Sel.Name)
	case *ast.ArrayType:
		element, err := r.renderType(value.Elt, true)
		if value.Len != nil {
			null = ""
		}
		return "Array<" + element + ">" + null, err
	case *ast.MapType:
		key, ok := value.Key.(*ast.Ident)
		if !ok || key.Name != "string" {
			return "", fmt.Errorf("wire maps require string keys")
		}
		element, err := r.renderType(value.Value, true)
		return "Record<string, " + element + ">" + null, err
	case *ast.StarExpr:
		element, err := r.renderType(value.X, false)
		return element + null, err
	case *ast.InterfaceType:
		if len(value.Methods.List) == 0 {
			return "unknown", nil
		}
	}
	return "", fmt.Errorf("unsupported Go wire shape %T", expr)
}
